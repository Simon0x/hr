package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Simon0x/hr/internal/attest"
	"github.com/Simon0x/hr/internal/constraints"
	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/memory"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

func cmdValidate(root string, args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "contracts":
		return validateContracts(root, args[1:])
	case "attestation":
		return validateAttestation(root, args[1:])
	case "ledger":
		return validateLedger(root)
	case "constraint-bindings":
		return validateConstraintBindings(root)
	case "commands":
		return validateCommands(root)
	case "links":
		return validateLinks(root)
	case "manifest":
		return validateManifest(root)
	case "memory":
		return validateMemory(root)
	default:
		fmt.Fprintf(os.Stderr, "unknown validate subcommand %q\n", args[0])
		return 2
	}
}

func flagValue(args []string, name string) (string, bool) {
	for i, a := range args {
		if a == "--"+name && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func discoverJSONTargets(root string, args []string, dirs ...string) []string {
	if f, ok := flagValue(args, "file"); ok {
		if filepath.IsAbs(f) {
			return []string{f}
		}
		return []string{filepath.Join(root, f)}
	}

	var targets []string
	for _, d := range dirs {
		dir := filepath.Join(root, d)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			targets = append(targets, filepath.Join(dir, n))
		}
	}
	return targets
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

type contractsResult struct {
	targets    int
	predicates int
	problems   []string
}

func runContractsValidation(root string, targets []string) (contractsResult, error) {
	reg, err := contracts.Load(root)
	if err != nil {
		return contractsResult{}, err
	}

	var problems []string

	for _, t := range targets {
		rel := relPath(root, t)
		raw, err := os.ReadFile(t)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: no such file", rel))
			continue
		}
		problems = append(problems, contracts.ValidateStatement(reg, raw, rel)...)
	}

	return contractsResult{targets: len(targets), predicates: len(reg.Predicates), problems: problems}, nil
}

// @gate validate:contracts
// @covers contracts/**
// @covers .hr/**
func validateContracts(root string, args []string) int {
	targets := discoverJSONTargets(root, args, "contracts/examples", ".hr/artifacts", ".hr/memory")
	if len(targets) == 0 {
		fmt.Println("no artifacts to validate — add one to contracts/examples/")
		return 0
	}

	result, err := runContractsValidation(root, targets)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if len(result.problems) > 0 {
		for _, p := range result.problems {
			fmt.Println(p)
		}
		fmt.Printf("\n%d contract violation(s)\n", len(result.problems))
		return 1
	}

	fmt.Printf("%d artifact(s) valid against %d predicate(s), and in-toto Statement-shaped\n",
		result.targets, result.predicates)
	return 0
}

func inCI() bool {
	return os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true"
}

// @gate validate:attestation
// @covers .hr/artifacts/**
// @covers trust/**
func validateAttestation(root string, args []string) int {
	targets := discoverJSONTargets(root, args, ".hr/artifacts")
	if len(targets) == 0 {
		fmt.Println("no signed artifacts to verify — .hr/artifacts/ is empty")
		return 0
	}

	trust, err := attest.LoadTrust(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(trust.Keys) == 0 {
		if inCI() {
			fmt.Fprintln(os.Stderr, "trust/keys.json has no keys — refusing rather than passing vacuously.")
			return 1
		}
		fmt.Printf("%d artifact(s) pending signature — no trusted keys yet, and this isn't CI\n", len(targets))
		return 0
	}

	failed := 0
	// When each artifact was signed, taken from the ledger's own `emitted`
	// entries. The chain is what makes this a usable clock: an entry cannot
	// be backdated without breaking every link after it, so a compromised key
	// cannot claim it signed inside its validity window.
	emittedAt, terr := emittedTimes(root)
	if terr != nil {
		fmt.Fprintln(os.Stderr, terr)
		return 2
	}

	for _, t := range targets {
		rel := relPath(root, t)
		raw, err := os.ReadFile(t)
		if err != nil {
			fmt.Printf("%s: no such file\n", rel)
			failed++
			continue
		}

		var env dsse.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			fmt.Printf("%s: not valid JSON\n", rel)
			failed++
			continue
		}
		if len(env.Signatures) == 0 {
			fmt.Printf("%s: unsigned — a bare Statement under .hr/artifacts/ proves nothing\n", rel)
			failed++
			continue
		}

		when, ok := emittedAt[artifactDigestOf(t)]
		if !ok {
			fmt.Printf("%s: signed but the ledger has no `emitted` entry for it — "+
				"there is no trustworthy instant to check the key against\n", rel)
			failed++
			continue
		}

		result, err := attest.Verify(&env, trust, when)
		if err != nil {
			fmt.Printf("%s: %v\n", rel, err)
			failed++
			continue
		}

		var stmt statement.Statement
		if err := json.Unmarshal(result.Statement, &stmt); err != nil || stmt.Type != statement.StatementType {
			fmt.Printf("%s: signature is good but the payload is not an in-toto Statement\n", rel)
			failed++
			continue
		}

		fmt.Printf("%s: verified — %s — %s\n", rel, result.KeyID, stmt.PredicateType)
	}

	if failed > 0 {
		fmt.Printf("\n%d artifact(s) failed verification\n", failed)
		return 1
	}
	return 0
}

// @gate validate:ledger
// @covers .hr/ledger.jsonl
// @covers internal/ledger/**
// @covers internal/dispatch/request.go
// @covers cmd/hr/ledger.go
func validateLedger(root string) int {
	lines, err := ledger.ReadLines(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(lines) == 0 {
		fmt.Println("no ledger yet — nothing to verify")
		return 0
	}

	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	eventSchema, ok := reg.Lookup("https://hr.dev/event/v1")
	if !ok {
		fmt.Fprintln(os.Stderr, "event schema not found")
		return 2
	}

	var problems []string
	expected := ledger.Genesis
	for i, line := range lines {
		var e ledger.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			problems = append(problems, fmt.Sprintf("entry %d: not valid JSON", i))
			break
		}

		if doc, err := jsonschema.UnmarshalJSON(strings.NewReader(line)); err == nil {
			if verr := eventSchema.Validate(doc); verr != nil {
				problems = append(problems, fmt.Sprintf("entry %d: %v", i, verr))
			}
		}

		if e.Seq != i {
			problems = append(problems, fmt.Sprintf("entry %d: seq is %d, expected %d — an entry was inserted or removed", i, e.Seq, i))
		}
		if e.Prev != expected {
			problems = append(problems, fmt.Sprintf(
				"entry %d: prev is %s…, expected %s… — entry %d was altered after this one was written",
				i, shortHash(e.Prev), shortHash(expected), i-1))
		}

		sum := sha256.Sum256([]byte(line))
		expected = hex.EncodeToString(sum[:])
	}

	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Println(p)
		}
		fmt.Printf("\n%d ledger problem(s)\n", len(problems))
		return 1
	}

	fmt.Printf("chain intact — %d entr%s, all valid\n", len(lines), plural(len(lines), "y", "ies"))
	return 0
}

var coverageHeadingRe = regexp.MustCompile(`(?m)^## `)
var atGateLineRe = regexp.MustCompile(`(?m)^.*at gate or above.*$`)

func gitTrackedAgentsFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "*AGENTS.md")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

func statedCoverageLine(text string) (string, bool) {
	sections := coverageHeadingRe.Split(text, -1)
	for _, s := range sections {
		if strings.HasPrefix(s, "Coverage") {
			if m := atGateLineRe.FindString(s); m != "" {
				return strings.TrimSpace(m), true
			}
		}
	}
	return "", false
}

// @gate validate:constraint-bindings
// @covers AGENTS.md
// @covers scaffold/AGENTS.md
// @covers internal/constraints/**
func validateConstraintBindings(root string) int {
	files, err := gitTrackedAgentsFiles(root)
	if err != nil || len(files) == 0 {
		fmt.Println("no AGENTS.md found — expected at least one, so something is wrong with the scan")
		return 2
	}

	var problems []string

	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		text := string(raw)
		cs := constraints.Parse(text)
		claimsCoverage := regexp.MustCompile(`(?m)^## Coverage`).MatchString(text)

		if len(cs) == 0 {
			if claimsCoverage {
				problems = append(problems, fmt.Sprintf(
					"%s: states a Coverage number but has no \"## Constraints\" section, so nothing can derive it — the number is a hand-maintained claim", rel))
			} else {
				problems = append(problems, fmt.Sprintf("%s: no constraints found under \"## Constraints\"", rel))
			}
			continue
		}

		unbound := 0
		for _, c := range cs {
			if c.Binding == nil {
				unbound++
				problems = append(problems, fmt.Sprintf(
					"%s: %q has no binding — state how it is enforced, or `none` if the honest answer is nothing", rel, c.Text))
			}
		}

		cov := constraints.ComputeCoverage(cs)
		expected := constraints.CoverageLine(cov)
		stated, ok := statedCoverageLine(text)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: no Coverage line to check", rel))
		} else if stated != expected {
			problems = append(problems, fmt.Sprintf("%s: Coverage line is stale\n    stated  %s\n    derived %s", rel, stated, expected))
		}

		summary := fmt.Sprintf("%s — %d constraint(s), %d/%d at gate or above (%d%%)", rel, len(cs), cov.High, cov.Total, cov.Percent)
		if unbound > 0 {
			summary += fmt.Sprintf(", %d unbound", unbound)
		}
		fmt.Println(summary)

		wantCount := 0
		for _, c := range cs {
			if c.Want != "" {
				wantCount++
			}
		}
		if wantCount > 0 {
			fmt.Printf("    %d carrying `want:` — the backlog that pays\n", wantCount)
		}
	}

	if len(problems) > 0 {
		fmt.Println()
		for _, p := range problems {
			fmt.Println(p)
		}
		fmt.Printf("\n%d problem(s)\n", len(problems))
		return 1
	}
	return 0
}

var hrCommandRe = regexp.MustCompile(`/hr:[a-z-]+`)

// @gate validate:commands
// @covers COMMANDS.md
// @covers skills/**
func validateCommands(root string) int {
	raw, err := os.ReadFile(filepath.Join(root, "COMMANDS.md"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	seen := map[string]bool{}
	for _, m := range hrCommandRe.FindAllString(string(raw), -1) {
		seen[m] = true
	}
	if len(seen) == 0 {
		fmt.Println("no /hr: names found in COMMANDS.md")
		return 1
	}
	var names []string
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)

	var missing []string
	for _, n := range names {
		skillName := strings.TrimPrefix(n, "/hr:")
		if _, err := os.Stat(filepath.Join(root, "skills", skillName, "SKILL.md")); err != nil {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		for _, m := range missing {
			fmt.Printf("%s: no skills/%s/SKILL.md\n", m, strings.TrimPrefix(m, "/hr:"))
		}
		fmt.Printf("\n%d missing\n", len(missing))
		return 1
	}

	fmt.Printf("all %d /hr: names resolve under skills/\n", len(names))
	return 0
}

var mdLinkRe = regexp.MustCompile(`\]\(([^)]+)\)`)

func stripFences(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	inFence := false
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// notOurMarkdown holds directories whose markdown belongs to someone else or
// to a build. A dependency's broken README link is not a rot signal about
// this framework, and reporting it trains the habit of ignoring the gate.
var notOurMarkdown = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
}

// @gate validate:links
// @covers **/*.md
func validateLinks(root string) int {
	var mdFiles []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if notOurMarkdown[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	sort.Strings(mdFiles)

	var broken []string
	for _, f := range mdFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := stripFences(string(raw))
		for _, m := range mdLinkRe.FindAllStringSubmatch(text, -1) {
			target := m[1]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
				continue
			}
			clean := target
			if i := strings.Index(clean, "#"); i >= 0 {
				clean = clean[:i]
			}
			if clean == "" {
				continue
			}
			resolved := filepath.Join(filepath.Dir(f), clean)
			if _, err := os.Stat(resolved); err != nil {
				broken = append(broken, fmt.Sprintf("%s: broken link -> %s", relPath(root, f), target))
			}
		}
	}

	if len(broken) > 0 {
		for _, b := range broken {
			fmt.Println(b)
		}
		fmt.Printf("\n%d broken link(s)\n", len(broken))
		return 1
	}
	fmt.Println("all relative links resolve")
	return 0
}

// @gate validate:manifest
// @covers .claude-plugin/**
func validateManifest(root string) int {
	if _, err := exec.LookPath("claude"); err == nil {
		fmt.Println("claude CLI present - running the official validator")
		cmd := exec.Command("claude", "plugin", "validate", ".")
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
	} else {
		fmt.Println("claude CLI absent - structural check only (weaker than the validator)")
		if code := manifestStructuralCheck(root); code != 0 {
			return code
		}
	}
	return manifestVersionCheck(root)
}

func manifestStructuralCheck(root string) int {
	var plugin map[string]any
	pluginRaw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	json.Unmarshal(pluginRaw, &plugin)

	var problems []string
	for _, f := range []string{"name", "version", "description"} {
		if s, ok := plugin[f].(string); !ok || s == "" {
			problems = append(problems, fmt.Sprintf("plugin.json missing or empty %q", f))
		}
	}

	var marketplace map[string]any
	marketRaw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	json.Unmarshal(marketRaw, &marketplace)
	for _, f := range []string{"name", "owner", "plugins"} {
		if _, ok := marketplace[f]; !ok {
			problems = append(problems, fmt.Sprintf("marketplace.json missing %q", f))
		}
	}

	if plugins, ok := marketplace["plugins"].([]any); ok {
		pname, _ := plugin["name"].(string)
		found := false
		for _, p := range plugins {
			pm, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if pm["name"] == pname {
				found = true
			}
			if _, ok := pm["source"]; !ok {
				problems = append(problems, fmt.Sprintf("marketplace entry %v missing source", pm["name"]))
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("plugin.json name %q not found among marketplace.json plugins", pname))
		}
	}

	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Println(p)
		}
		return 1
	}
	return 0
}

func manifestVersionCheck(root string) int {
	var plugin struct{ Name, Version string }
	pluginRaw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	json.Unmarshal(pluginRaw, &plugin)

	var marketplace struct {
		Plugins []struct{ Name, Version, Source string }
	}
	marketRaw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	json.Unmarshal(marketRaw, &marketplace)

	for _, p := range marketplace.Plugins {
		if p.Name == plugin.Name {
			if p.Version != plugin.Version {
				fmt.Printf("version drift: plugin.json=%s marketplace.json=%s\n", plugin.Version, p.Version)
				return 1
			}
			fmt.Printf("manifests agree at version %s\n", plugin.Version)
			return 0
		}
	}
	fmt.Println("plugin name not found in marketplace.json")
	return 1
}

// @gate validate:memory
// @covers .hr/memory/**
// @covers internal/memory/**
// @covers cmd/hr/memory.go
func validateMemory(root string) int {
	dir := memory.Dir(root)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		fmt.Println("no memory store yet — nothing to verify")
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	var problems []string
	digests := map[string]bool{}
	supersedesRefs := map[string]string{}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		problems = append(problems, contracts.ValidateStatement(reg, raw, "memory/"+e.Name())...)

		digest := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "memory-"), ".json")
		digests[digest] = true

		var stmt statement.Statement
		if json.Unmarshal(raw, &stmt) == nil {
			var pred map[string]any
			json.Unmarshal(stmt.Predicate, &pred)
			if s, ok := pred["supersedes"].(string); ok && s != "" {
				supersedesRefs[digest] = s
			}
		}
	}

	forgottenDigests := map[string]bool{}
	forgottenEntries, _ := os.ReadDir(memory.ForgottenDir(root))
	for _, e := range forgottenEntries {
		if strings.HasSuffix(e.Name(), ".json") {
			forgottenDigests[strings.TrimSuffix(strings.TrimPrefix(e.Name(), "memory-"), ".json")] = true
		}
	}

	for digest, sup := range supersedesRefs {
		if !digests[sup] && !forgottenDigests[sup] {
			problems = append(problems, fmt.Sprintf("memory-%s: supersedes %s, which is nowhere in the store", digest, sup))
		}
	}

	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Println(p)
		}
		fmt.Printf("\n%d memory problem(s)\n", len(problems))
		return 1
	}
	fmt.Printf("%d memor%s valid\n", len(digests), plural(len(digests), "y", "ies"))
	return 0
}

// emittedTimes maps an artifact digest to when the ledger recorded it. The
// ledger is hash-chained, so this instant cannot be moved after the fact
// without the ledger gate saying so.
func emittedTimes(root string) (map[string]time.Time, error) {
	entries, err := persistence.File{Root: root}.Read(context.Background())
	if err != nil {
		return nil, err
	}
	out := map[string]time.Time{}
	for _, e := range entries {
		if e.Kind != "emitted" || e.Artifacts == nil {
			continue
		}
		at, perr := time.Parse("2006-01-02T15:04:05.000Z", e.At)
		if perr != nil {
			if at, perr = time.Parse(time.RFC3339, e.At); perr != nil {
				continue
			}
		}
		for _, digest := range e.Artifacts.Out {
			// First writer wins: the emit that created it, not a later entry
			// that happens to mention the same digest.
			if _, seen := out[digest]; !seen {
				out[digest] = at
			}
		}
	}
	return out, nil
}

// artifactDigestOf recovers the digest an artifact file is named for:
// <kind>-<digest12>.json, the same name emit writes.
func artifactDigestOf(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".json")
	if i := strings.LastIndex(base, "-"); i >= 0 {
		return base[i+1:]
	}
	return base
}
