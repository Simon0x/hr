package statement

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDigestMatchesStoredArtifacts(t *testing.T) {
	root := findRepoRoot(t)
	dir := filepath.Join(root, ".hr", "artifacts")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		t.Skip("no .hr/artifacts in this checkout")
	}
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		base := strings.TrimSuffix(name, ".json")
		idx := strings.LastIndex(base, "-")
		if idx < 0 {
			continue
		}
		wantDigest := base[idx+1:]
		if len(wantDigest) != 12 {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got, err := Digest(raw)
		if err != nil {
			t.Fatalf("%s: Digest: %v", name, err)
		}
		if got[:12] != wantDigest {
			t.Errorf("%s: canonicalised digest %s..., filename says %s", name, got[:12], wantDigest)
		}
		checked++
	}
	if checked == 0 {
		t.Skip("no artifacts to check")
	}
	t.Logf("verified %d stored artifact(s) against their filenames", checked)
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found upward)")
		}
		dir = parent
	}
}
