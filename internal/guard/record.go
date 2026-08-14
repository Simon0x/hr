package guard

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/policy"
)

// Denial is one refused tool call, written by the guard process and folded
// into the ledger by the invocation that produced it.
//
// The guard does not append to the ledger itself. The chain is serialized by
// a process-local mutex, and a fan-out runs several agents at once, so
// several guard processes would be appending to one chain with nothing
// ordering them. Writing per-invocation files and folding them in afterwards
// keeps every ledger write in the one process that already owns them.
type Denial struct {
	Action  string   `json:"action"`
	Tool    string   `json:"tool"`
	Risk    string   `json:"risk"`
	Rev     string   `json:"reversibility"`
	Why     string   `json:"why"`
	Verdict string   `json:"verdict"`
	Policy  string   `json:"policy"`
	Reasons []string `json:"reasons"`
}

// DenialLogPath is where a guard writes for one invocation. The token is
// supplied by the dispatching step, so two concurrent leads never share a file.
func DenialLogPath(root, token string) string {
	return filepath.Join(root, ".hr", "guard", token+".jsonl")
}

// RecordDenial appends one refusal to this invocation's log.
func RecordDenial(root, token string, d Denial) error {
	path := DenialLogPath(root, token)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(d)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// TakeDenials reads and removes an invocation's log. Removing it is what
// makes a fold exactly once: a token is per-invocation, and a leftover file
// would be replayed into the ledger by whatever ran next.
func TakeDenials(root, token string) ([]Denial, error) {
	path := DenialLogPath(root, token)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer os.Remove(path)

	var out []Denial
	for _, line := range splitLines(raw) {
		var d Denial
		if err := json.Unmarshal(line, &d); err != nil {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			if i > start {
				out = append(out, raw[start:i])
			}
			start = i + 1
		}
	}
	if start < len(raw) {
		out = append(out, raw[start:])
	}
	return out
}

// Entries turns folded denials into ledger entries. A refused tool call is a
// decision the engine made, not an action that happened.
func Entries(denials []Denial, actor, goal string) []ledger.Entry {
	entries := make([]ledger.Entry, 0, len(denials))
	for _, d := range denials {
		entries = append(entries, ledger.Entry{
			Kind: "decision", Actor: actor, Goal: goal, Outcome: "refused",
			Policy: d.Policy,
			Detail: "tool call refused — " + d.Action + " (" + d.Risk + "/" + d.Rev + ": " + d.Why + ")",
		})
	}
	return entries
}

// DecisionPolicy is the digest the guard records, so a refusal names the
// exact policy version that produced it.
func DecisionPolicy(d policy.Decision) string { return d.Policy }
