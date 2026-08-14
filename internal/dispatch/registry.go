package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FanOutConfig struct {
	Enabled           bool     `json:"enabled"`
	Leads             []string `json:"leads,omitempty"`
	Concurrency       int      `json:"concurrency,omitempty"`
	LeadTokenEstimate float64  `json:"leadTokenEstimate,omitempty"`
}

type Seat struct {
	Name          string        `json:"name"`
	Version       string        `json:"version"`
	Procedure     string        `json:"procedure"`
	Risk          string        `json:"risk"`
	Reversibility string        `json:"reversibility"`
	FanOut        *FanOutConfig `json:"fanOut,omitempty"`
	Verifier      string        `json:"verifier,omitempty"`
}

func LoadSeats(root string) (map[string]Seat, error) {
	dir := filepath.Join(root, "capabilities")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("loading capability registry: %w", err)
	}

	out := map[string]Seat{}
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		var s Seat
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		if s.Name == "" {
			return nil, fmt.Errorf("%s: missing name", e.Name())
		}
		if s.Procedure == "" {
			return nil, fmt.Errorf("%s: missing procedure", e.Name())
		}
		if _, dup := out[s.Name]; dup {
			return nil, fmt.Errorf("%s: department %q already registered by another file", e.Name(), s.Name)
		}
		out[s.Name] = s
	}
	return out, nil
}
