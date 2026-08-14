package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNone_IsHonestAboutNotConfining(t *testing.T) {
	argv := []string{"claude", "-p"}
	got, err := None{}.Wrap(argv, Policy{Writable: []string{"/x"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != strings.Join(argv, " ") {
		t.Errorf("None rewrote argv to %v", got)
	}
	var none None
	if none.Name() != "none" {
		t.Error("a provider that does not confine must not be named as if it does")
	}
}

func TestSelect_NamesWhatIsActuallyInForce(t *testing.T) {
	name := Select().Name()
	if Available() && name != "landlock" {
		t.Errorf("landlock is available but Select chose %q", name)
	}
	if !Available() && name != "none" {
		t.Errorf("no backend is available but Select chose %q", name)
	}
}

func TestLandlock_WrapReEntersHrAsTheHelper(t *testing.T) {
	if !Available() {
		t.Skip("no landlock on this host")
	}
	var ll Landlock
	got, err := ll.Wrap([]string{"claude", "-p"}, Policy{Writable: []string{"/a", "/b"}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, " confine ") {
		t.Fatalf("wrapped argv does not re-enter hr: %v", got)
	}
	for _, w := range []string{"/a", "/b"} {
		if !strings.Contains(joined, "--writable "+w) {
			t.Errorf("writable root %s missing from %v", w, got)
		}
	}
	if got[len(got)-2] != "claude" || got[len(got)-1] != "-p" {
		t.Errorf("the confined command must come last, got %v", got)
	}
}

// The claim the finding turns on: a process under confinement cannot write
// outside the roots it was given, whatever it tries.
func TestApply_ConfinesWritesToTheGivenRoots(t *testing.T) {
	if !Available() {
		t.Skip("no landlock on this host")
	}
	if os.Getenv("HR_SANDBOX_CHILD") == "1" {
		// Runs inside the confined child spawned below.
		allowed := os.Getenv("HR_SANDBOX_ALLOWED")
		if err := Apply(Policy{Writable: []string{allowed}}); err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(filepath.Join(allowed, "inside.txt"), []byte("x"), 0o644); err != nil {
			os.Exit(4)
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("HR_SANDBOX_DENIED"), "outside.txt"), []byte("x"), 0o644); err == nil {
			os.Exit(5) // the write escaped
		}
		os.Exit(0)
	}

	allowed, denied := t.TempDir(), t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run", "TestApply_ConfinesWritesToTheGivenRoots")
	cmd.Env = append(os.Environ(),
		"HR_SANDBOX_CHILD=1", "HR_SANDBOX_ALLOWED="+allowed, "HR_SANDBOX_DENIED="+denied)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("confined child failed (%v): %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(allowed, "inside.txt")); err != nil {
		t.Errorf("a write inside the writable root did not land: %v", err)
	}
	if _, err := os.Stat(filepath.Join(denied, "outside.txt")); err == nil {
		t.Error("a write outside the writable roots escaped confinement")
	}
}
