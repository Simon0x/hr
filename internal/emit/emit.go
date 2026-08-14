package emit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

type Result struct {
	Path   string
	Rel    string
	Digest string
	Kind   string
}

// Emit validates and stores a Statement's raw bytes exactly as `hr emit`
// does, so the CLI and the dispatcher/daemon share one implementation.
// Returns the contract-validation problems (not an error) when the
// Statement is well-formed JSON but fails its schema — same distinction the
// CLI makes between an exit-2 usage error and an exit-1 contract violation.
func Emit(ctx context.Context, st persistence.Store, root string, raw []byte, actor string) (*Result, []string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil, errors.New("nothing to emit")
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, nil, fmt.Errorf("not valid JSON: %w", err)
	}
	_, hasSig := probe["signatures"]
	_, hasPT := probe["payloadType"]
	if hasSig || hasPT {
		return nil, nil, errors.New("that is an envelope, not a Statement — emit takes the Statement")
	}

	canonical, err := statement.Canonical(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("not valid JSON: %w", err)
	}
	sum := sha256.Sum256(canonical)
	digest := hex.EncodeToString(sum[:])

	var stmt statement.Statement
	if err := json.Unmarshal(raw, &stmt); err != nil {
		return nil, nil, fmt.Errorf("not valid JSON: %w", err)
	}
	kind := store.KindOf(stmt.PredicateType)
	if kind == "" {
		kind = "unknown"
	}

	storeDir := filepath.Join(root, ".hr", "artifacts")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return nil, nil, err
	}
	outPath := filepath.Join(storeDir, fmt.Sprintf("%s-%s.json", kind, digest[:12]))
	relTrimmed := strings.TrimPrefix(strings.TrimPrefix(outPath, root), string(filepath.Separator))
	rel := "." + string(filepath.Separator) + relTrimmed

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, canonical, "", "  "); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(outPath, append(pretty.Bytes(), '\n'), 0o644); err != nil {
		return nil, nil, err
	}

	reg, err := contracts.Load(root)
	if err != nil {
		return nil, nil, err
	}
	problems := contracts.ValidateStatement(reg, pretty.Bytes(), rel)
	if len(problems) > 0 {
		os.Remove(outPath)
		return nil, problems, nil
	}

	var predDoc map[string]any
	_ = json.Unmarshal(stmt.Predicate, &predDoc)
	goal, _ := predDoc["goal"].(string)
	if goal == "" {
		goal = os.Getenv("HR_GOAL")
	}
	if actor == "" {
		actor = "spiffe://hr.local/unattributed"
	}

	entry := ledger.Entry{
		Kind: "emitted", Actor: actor,
		Artifacts: &ledger.Artifacts{In: []string{}, Out: []string{digest[:12]}},
		Outcome:   "ok",
		Detail:    fmt.Sprintf("%s %s", kind, rel),
	}
	if goal != "" {
		entry.Goal = goal
	}
	if _, err := st.Append(ctx, entry); err != nil {
		return nil, nil, err
	}

	return &Result{Path: outPath, Rel: rel, Digest: digest, Kind: kind}, nil, nil
}
