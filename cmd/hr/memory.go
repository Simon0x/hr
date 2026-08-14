package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/memory"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/statement"
)

func recallQuestion(args []string) string {
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--scope" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(a, "--") {
			continue
		}
		return a
	}
	return ""
}

func cmdRecall(root string, args []string) int {
	question := recallQuestion(args)
	scope, _ := flagValue(args, "scope")
	jsonOut := hasFlag(args, "json")
	includeStale := hasFlag(args, "include-stale")

	dir := memory.Dir(root)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Println("nothing remembered yet")
		return 0
	}

	result, err := memory.Recall(root, question, scope, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if len(result.Matched) == 0 {
		questionWords := memory.Words(question)
		if jsonOut {
			b, _ := json.MarshalIndent(map[string]any{
				"matched": []string{}, "hits": []string{}, "reason": "no vocabulary overlap",
			}, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Printf("nothing stored uses any of: %s\n\n", strings.Join(questionWords, ", "))
			fmt.Println("The store has no vocabulary for this question. Not guessing — a plausible\n" +
				"answer that was never stored is worse than no answer, because it cannot be\n" +
				"told apart from a real one afterwards.")
		}
		return 0
	}

	if jsonOut {
		hits := []map[string]any{}
		for _, h := range result.Hits {
			m := map[string]any{"digest": h.Digest}
			for k, v := range h.Predicate {
				m[k] = v
			}
			hits = append(hits, m)
		}
		withheld := []map[string]any{}
		for _, w := range result.Withheld {
			withheld = append(withheld, map[string]any{"digest": w.Digest, "claim": w.Claim, "why": w.Why})
		}
		b, _ := json.MarshalIndent(map[string]any{"matched": result.Matched, "hits": hits, "withheld": withheld}, "", "  ")
		fmt.Println(string(b))
		return 0
	}

	fmt.Printf("matched on: %s\n\n", strings.Join(result.Matched, ", "))
	for _, h := range result.Hits {
		claim, _ := h.Predicate["claim"].(string)
		confidence, _ := h.Predicate["confidence"].(string)
		owner, _ := h.Predicate["owner"].(string)
		lastVerified, _ := h.Predicate["lastVerified"].(string)
		source, _ := h.Predicate["source"].(string)
		fmt.Printf("  %s\n", claim)
		fmt.Printf("    %s · %s · verified %s · %s\n", confidence, owner, lastVerified, source)
	}
	if len(result.Withheld) > 0 {
		fmt.Println("\nwithheld")
		for _, w := range result.Withheld {
			fmt.Printf("  %s — %s\n", w.Claim, w.Why)
		}
		if !includeStale {
			fmt.Println("\npass --include-stale to see them anyway")
		}
	}
	return 0
}

func countJSONFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			n++
		}
	}
	return n
}

func cmdRemember(root string, args []string) int {
	if hasFlag(args, "forget") {
		return rememberForget(root, args)
	}

	var file string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			file = a
			break
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "usage: hr remember <memory.json>")
		return 2
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	var stmt statement.Statement
	if err := json.Unmarshal(raw, &stmt); err != nil {
		fmt.Fprintf(os.Stderr, "not valid JSON: %v\n", err)
		return 2
	}
	if stmt.PredicateType != "https://hr.dev/memory/v1" {
		fmt.Fprintf(os.Stderr, "predicateType must be https://hr.dev/memory/v1, got %q\n", stmt.PredicateType)
		return 2
	}

	canonical, err := statement.Canonical(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	sum := sha256.Sum256(canonical)
	digest := hex.EncodeToString(sum[:])[:12]

	dir := memory.Dir(root)
	beforeCount := countJSONFiles(dir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	outPath := filepath.Join(dir, fmt.Sprintf("memory-%s.json", digest))
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, canonical, "", "  "); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := os.WriteFile(outPath, append(pretty.Bytes(), '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	problems := contracts.ValidateStatement(reg, pretty.Bytes(), relPath(root, outPath))
	if len(problems) > 0 {
		os.Remove(outPath)
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		fmt.Fprintln(os.Stderr, "\nnot stored — the memory does not satisfy its contract")
		return 1
	}

	if countJSONFiles(dir) < beforeCount {
		fmt.Fprintln(os.Stderr, "store shrank during write — refusing")
		return 1
	}

	var pred map[string]any
	_ = json.Unmarshal(stmt.Predicate, &pred)

	if supersedes, ok := pred["supersedes"].(string); ok && supersedes != "" {
		livePath := filepath.Join(dir, fmt.Sprintf("memory-%s.json", supersedes))
		forgottenPath := filepath.Join(memory.ForgottenDir(root), fmt.Sprintf("memory-%s.json", supersedes))
		_, liveErr := os.Stat(livePath)
		_, forgottenErr := os.Stat(forgottenPath)
		if liveErr != nil && forgottenErr != nil {
			os.Remove(outPath)
			fmt.Fprintf(os.Stderr, "supersedes %s, which is nowhere in the store — nothing was resolved\n", supersedes)
			return 1
		}
	}

	actor := os.Getenv("HR_ACTOR")
	if actor == "" {
		actor = "spiffe://hr.local/unattributed"
	}
	claim, _ := pred["claim"].(string)
	if len(claim) > 60 {
		claim = claim[:60]
	}
	if _, err := (persistence.File{Root: root}).Append(context.Background(), ledger.Entry{
		Kind: "promotion", Actor: actor,
		Artifacts: &ledger.Artifacts{In: []string{}, Out: []string{digest}},
		Outcome:   "ok", Detail: "remembered: " + claim,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	quarantined, _ := pred["quarantined"].(bool)
	status := fmt.Sprintf("remembered %s", digest)
	if supersedes, ok := pred["supersedes"].(string); ok && supersedes != "" {
		status += fmt.Sprintf(" (supersedes %s)", supersedes)
	}
	if quarantined {
		status += " — quarantined, will not be served as fact"
	}
	fmt.Fprintln(os.Stderr, status)
	fmt.Println(digest)
	return 0
}

func rememberForget(root string, args []string) int {
	digest, _ := flagValue(args, "forget")
	reason, hasReason := flagValue(args, "because")
	if !hasReason || reason == "" || strings.HasPrefix(reason, "--") {
		fmt.Fprintln(os.Stderr, "a deletion with no reason cannot be reviewed")
		return 2
	}

	dir := memory.Dir(root)
	path := filepath.Join(dir, fmt.Sprintf("memory-%s.json", digest))
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(os.Stderr, "no such memory: %s\n", digest)
		return 1
	}

	forgottenDir := memory.ForgottenDir(root)
	if err := os.MkdirAll(forgottenDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	newPath := filepath.Join(forgottenDir, fmt.Sprintf("memory-%s.json", digest))
	if err := os.Rename(path, newPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	reasonPath := filepath.Join(forgottenDir, fmt.Sprintf("memory-%s.reason.txt", digest))
	content := time.Now().UTC().Format("2006-01-02T15:04:05.000Z") + "\n" + reason + "\n"
	_ = os.WriteFile(reasonPath, []byte(content), 0o644)

	actor := os.Getenv("HR_ACTOR")
	if actor == "" {
		actor = "spiffe://hr.local/unattributed"
	}
	if _, err := (persistence.File{Root: root}).Append(context.Background(), ledger.Entry{
		Kind: "promotion", Actor: actor, Outcome: "ok",
		Detail: fmt.Sprintf("forgot %s: %s", digest, reason),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	fmt.Printf("forgot %s — tombstoned in .hr/memory/forgotten/\n", digest)
	return 0
}
