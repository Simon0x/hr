package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Simon0x/hr/internal/impact"
)

type jsonConstraint struct {
	Text    string `json:"text"`
	Binding string `json:"binding"`
}

type jsonRow struct {
	Path        string           `json:"path"`
	Gates       []string         `json:"gates"`
	Constraints []jsonConstraint `json:"constraints"`
	Signals     []string         `json:"signals"`
}

type jsonBandInfo struct {
	Confidence string `json:"confidence"`
	Status     string `json:"status,omitempty"`
	Paths      int    `json:"paths,omitempty"`
}

type jsonBands struct {
	Direct     jsonBandInfo `json:"direct"`
	Transitive jsonBandInfo `json:"transitive"`
	Runtime    jsonBandInfo `json:"runtime"`
}

type jsonResult struct {
	Base      string           `json:"base"`
	Changed   []string         `json:"changed"`
	Covered   []jsonRow        `json:"covered"`
	Uncovered []jsonRow        `json:"uncovered"`
	GateOnly  []string         `json:"gateOnly"`
	Unscoped  []jsonConstraint `json:"unscoped"`
	Bands     jsonBands        `json:"bands"`
}

func toJSONRows(rows []impact.Row) []jsonRow {
	out := []jsonRow{}
	for _, r := range rows {
		cs := []jsonConstraint{}
		for _, c := range r.Constraints {
			cs = append(cs, jsonConstraint{Text: c.Text, Binding: c.Binding})
		}
		gates := r.Gates
		if gates == nil {
			gates = []string{}
		}
		out = append(out, jsonRow{Path: r.Path, Gates: gates, Constraints: cs, Signals: []string{}})
	}
	return out
}

func cmdImpact(root string, args []string) int {
	base := flagValueOr(args, "base", "HEAD")
	jsonOut := hasFlag(args, "json")

	result, err := impact.Derive(root, base)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if len(result.Changed) == 0 {
		if jsonOut {
			b, _ := json.MarshalIndent(map[string]any{
				"base": base, "changed": []string{}, "covered": []string{}, "uncovered": []string{},
				"gateOnly": []string{}, "bands": map[string]any{},
			}, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Printf("impact against %s — 0 path(s) changed\n", base)
		}
		return 0
	}

	if jsonOut {
		unscoped := []jsonConstraint{}
		for _, c := range result.Unscoped {
			unscoped = append(unscoped, jsonConstraint{Text: c.Text, Binding: c.Binding})
		}
		out := jsonResult{
			Base: result.Base, Changed: result.Changed,
			Covered: toJSONRows(result.Covered), Uncovered: toJSONRows(result.Uncovered),
			GateOnly: result.GateOnly, Unscoped: unscoped,
			Bands: jsonBands{
				Direct:     jsonBandInfo{Confidence: result.Bands.Direct.Confidence, Paths: result.Bands.Direct.Paths},
				Transitive: jsonBandInfo{Confidence: result.Bands.Transitive.Confidence, Status: result.Bands.Transitive.Status},
				Runtime:    jsonBandInfo{Confidence: result.Bands.Runtime.Confidence, Status: result.Bands.Runtime.Status},
			},
		}
		if out.GateOnly == nil {
			out.GateOnly = []string{}
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
	} else {
		printImpactText(result)
	}

	if len(result.Uncovered) > 0 {
		return 1
	}
	return 0
}

func printImpactText(r *impact.Result) {
	fmt.Printf("impact against %s — %d path(s) changed\n\n", r.Base, len(r.Changed))

	if len(r.Covered) > 0 {
		fmt.Println("covered")
		for _, row := range r.Covered {
			fmt.Printf("  %s\n", row.Path)
			for _, c := range row.Constraints {
				fmt.Printf("    %-26s %s\n", c.Binding, c.Text)
			}
			citedGates := map[string]bool{}
			for _, c := range row.Constraints {
				citedGates[c.Binding] = true
			}
			for _, g := range row.Gates {
				cited := false
				for _, c := range row.Constraints {
					if strings.Contains(c.Binding, g) {
						cited = true
						break
					}
				}
				if !cited {
					fmt.Printf("    %-26s (no constraint cites this gate)\n", g)
				}
			}
		}
		fmt.Println()
	}

	if len(r.Uncovered) > 0 {
		fmt.Println("uncovered  ← the finding")
		for _, row := range r.Uncovered {
			fmt.Printf("  %s\n", row.Path)
		}
		fmt.Println()
	}

	if len(r.GateOnly) > 0 {
		fmt.Printf("gate-only  %s  (noise; dropped)\n\n", strings.Join(r.GateOnly, ", "))
	}

	fmt.Println("signals    none declared — nothing here is watched after deploy")

	if len(r.Unscoped) > 0 {
		fmt.Println("\nunscoped constraints  (apply repo-wide, so they discriminate nowhere)")
		for _, c := range r.Unscoped {
			fmt.Printf("  %s\n", c.Binding)
		}
	}

	fmt.Println("\nbands")
	fmt.Printf("  %-12s%-9s%d path(s)\n", "direct", r.Bands.Direct.Confidence, r.Bands.Direct.Paths)
	fmt.Printf("  %-12s%-9s%s\n", "transitive", r.Bands.Transitive.Confidence, r.Bands.Transitive.Status)
	fmt.Printf("  %-12s%-9s%s\n", "runtime", r.Bands.Runtime.Confidence, r.Bands.Runtime.Status)
}
