package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Simon0x/hr/internal/statement"
)

var kindPattern = regexp.MustCompile(`^https://hr\.dev/([a-z]+)/v[0-9]+$`)

func KindOf(predicateType string) string {
	m := kindPattern.FindStringSubmatch(predicateType)
	if m == nil {
		return ""
	}
	return m[1]
}

type Artifact struct {
	ID            string
	Kind          string
	PredicateType string
	Subject       []statement.Subject
	Predicate     map[string]any
	File          string
	CreatedBy     string
	CreatedAt     time.Time
}

func LoadArtifacts(root string) ([]Artifact, error) {
	dir := filepath.Join(root, ".hr", "artifacts")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []Artifact
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}

		stmtBytes := raw
		var envelope struct {
			Payload string `json:"payload"`
		}
		if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Payload != "" {
			if decoded, err := base64.StdEncoding.DecodeString(envelope.Payload); err == nil {
				stmtBytes = decoded
			}
		}

		var stmt statement.Statement
		if err := json.Unmarshal(stmtBytes, &stmt); err != nil {
			continue
		}

		id, err := statement.Digest(stmtBytes)
		if err != nil {
			continue
		}

		var pred map[string]any
		_ = json.Unmarshal(stmt.Predicate, &pred)

		out = append(out, Artifact{
			ID: id[:12], Kind: KindOf(stmt.PredicateType), PredicateType: stmt.PredicateType,
			Subject: stmt.Subject, Predicate: pred, File: e.Name(),
		})
	}
	return out, nil
}

// DigestID is the artifact's identity: sha256 over its canonical bytes,
// truncated. It is derived from content on both write and read, so an
// artifact cannot be filed under a name its bytes do not produce.
func DigestID(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])[:12]
}

// ArtifactPath is where an artifact of this kind and digest lives under root.
func ArtifactPath(root, kind, id string) string {
	return filepath.Join(root, ".hr", "artifacts", fmt.Sprintf("%s-%s.json", kind, id[:12]))
}

// Insert writes canonical to the artifact store, pretty-printed. It reports
// whether the artifact was new; re-emitting the same digest is a no-op, not
// an error, because emitting the same evidence twice is not a failure.
func Insert(root string, a Artifact, canonical []byte, _ string) (bool, error) {
	path := ArtifactPath(root, a.Kind, DigestID(canonical))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, canonical, "", "  "); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, append(pretty.Bytes(), '\n'), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
