package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var Genesis = strings.Repeat("0", 64)

// appendMu serializes Append across goroutines within this process; it does not protect against a second process racing on the same file.
var appendMu sync.Mutex

func Path(root string) string {
	return filepath.Join(root, ".hr", "ledger.jsonl")
}

func Digest(e Entry) (string, error) {
	c, err := Canonical(e)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(c)
	return hex.EncodeToString(sum[:]), nil
}

func ReadLines(root string) ([]string, error) {
	raw, err := os.ReadFile(Path(root))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		lines = append(lines, l)
	}
	return lines, nil
}

func Read(root string) ([]Entry, error) {
	lines, err := ReadLines(root)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(lines))
	for i, l := range lines {
		var e Entry
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			return nil, fmt.Errorf("line %d: %w", i, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func Append(root string, e Entry) (Entry, error) {
	appendMu.Lock()
	defer appendMu.Unlock()

	if e.At == "" {
		e.At = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}

	entries, err := Read(root)
	if err != nil {
		return Entry{}, err
	}

	prev := Genesis
	if len(entries) > 0 {
		prev, err = Digest(entries[len(entries)-1])
		if err != nil {
			return Entry{}, err
		}
	}
	e.Seq = len(entries)
	e.Prev = prev

	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Entry{}, err
	}
	line, err := Canonical(e)
	if err != nil {
		return Entry{}, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Entry{}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Entry{}, err
	}
	return e, nil
}

func VerifyChain(entries []Entry) (string, bool) {
	expected := Genesis
	for i, e := range entries {
		if e.Seq != i {
			return fmt.Sprintf("entry %d: seq is %d, expected %d — an entry was inserted or removed", i, e.Seq, i), false
		}
		if e.Prev != expected {
			return fmt.Sprintf("entry %d: prev is %s…, expected %s… — entry %d was altered after this one was written",
				i, short(e.Prev), short(expected), i-1), false
		}
		d, err := Digest(e)
		if err != nil {
			return fmt.Sprintf("entry %d: %v", i, err), false
		}
		expected = d
	}
	return "", true
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
