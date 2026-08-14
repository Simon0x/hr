package dispatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSeats_LoadsRealCapabilities(t *testing.T) {
	root := testRoot(t)

	seats, err := LoadSeats(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Discovery", "Product", "Engineering", "QA", "Release", "Ops"} {
		if _, ok := seats[name]; !ok {
			t.Errorf("expected %q to be registered, capabilities/ has %d entries", name, len(seats))
		}
	}

	discovery, ok := seats["Discovery"]
	if !ok {
		t.Fatal("Discovery not loaded")
	}
	if discovery.Procedure != "departments/discovery.md" {
		t.Errorf("Discovery.Procedure = %q", discovery.Procedure)
	}
	if discovery.FanOut == nil || !discovery.FanOut.Enabled {
		t.Fatal("expected Discovery.FanOut.Enabled")
	}
	if len(discovery.FanOut.Leads) != 3 {
		t.Errorf("Discovery.FanOut.Leads = %v, want 3 entries", discovery.FanOut.Leads)
	}
	if discovery.Verifier != "validate" {
		t.Errorf("Discovery.Verifier = %q, want validate", discovery.Verifier)
	}

	product, ok := seats["Product"]
	if !ok {
		t.Fatal("Product not loaded")
	}
	if product.FanOut != nil {
		t.Errorf("expected Product to have no fan-out config, got %+v", product.FanOut)
	}
}

func TestLoadSeats_MissingDirectoryIsAnError(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadSeats(root); err == nil {
		t.Fatal("expected an error when capabilities/ does not exist")
	}
}

func TestLoadSeats_RejectsDuplicateDepartmentName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "capabilities")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dup := `{"name": "Discovery", "procedure": "departments/discovery.md"}`
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte(dup), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.json"), []byte(dup), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSeats(root); err == nil {
		t.Fatal("expected an error on duplicate department name across files")
	}
}
