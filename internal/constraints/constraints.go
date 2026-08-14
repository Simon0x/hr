package constraints

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

var rungs = []string{"compile", "db", "guard", "codegen", "test", "gate", "check", "none"}
var atGateOrAbove = map[string]bool{"compile": true, "db": true, "guard": true, "codegen": true, "test": true, "gate": true}

var sectionRe = regexp.MustCompile(`(?m)^## `)
var notConstraintsRe = regexp.MustCompile(`^(Coverage|Documentation|Deep-dive|Never run these|General)\b`)
var rungRe = regexp.MustCompile("`(compile|db|guard|codegen|test|gate|check|none)(?::\\s*([^`]+))?`")
var wantRe = regexp.MustCompile("`want:\\s*([a-z]+)`")
var scriptRe = regexp.MustCompile(`^[a-z]+:[a-z-]+$`)
var boldRe = regexp.MustCompile(`(?s)\*\*(.+?)\*\*`)
var wsRe = regexp.MustCompile(`\s+`)

type Binding struct {
	Rung   string
	Script string
	Raw    string
}

type Constraint struct {
	Text    string
	Binding *Binding
	Want    string
}

func splitSections(markdown string) []string {
	locs := sectionRe.FindAllStringIndex(markdown, -1)
	var sections []string
	for i, loc := range locs {
		end := len(markdown)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		sections = append(sections, markdown[loc[1]:end])
	}
	return sections
}

func splitBullets(section string) []string {
	parts := strings.Split(section, "\n- ")
	if len(parts) <= 1 {
		return nil
	}
	var bullets []string
	for _, p := range parts[1:] {
		bullets = append(bullets, "- "+p)
	}
	return bullets
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func Parse(markdown string) []Constraint {
	var out []Constraint
	for _, section := range splitSections(markdown) {
		strict := strings.HasPrefix(section, "Constraints")
		if !strict && notConstraintsRe.MatchString(section) {
			continue
		}

		for _, bullet := range splitBullets(section) {
			matches := rungRe.FindAllStringSubmatch(bullet, -1)
			if !strict && len(matches) == 0 {
				continue
			}

			var binding *Binding
			if len(matches) > 0 {
				live := matches[len(matches)-1]
				script := ""
				trimmed := strings.TrimSpace(live[2])
				if trimmed != "" && scriptRe.MatchString(trimmed) {
					script = trimmed
				}
				binding = &Binding{
					Rung:   live[1],
					Script: script,
					Raw:    strings.ReplaceAll(live[0], "`", ""),
				}
			}

			text := ""
			if bm := boldRe.FindStringSubmatch(bullet); bm != nil {
				text = wsRe.ReplaceAllString(bm[1], " ")
			} else {
				line := strings.TrimPrefix(firstLine(bullet), "- ")
				if len(line) > 60 {
					line = line[:60]
				}
				text = line
			}

			want := ""
			if wm := wantRe.FindStringSubmatch(bullet); wm != nil {
				want = wm[1]
			}

			out = append(out, Constraint{Text: text, Binding: binding, Want: want})
		}
	}
	return out
}

type Coverage struct {
	Counts  map[string]int
	Unbound int
	Total   int
	High    int
	Percent int
}

func ComputeCoverage(cs []Constraint) Coverage {
	counts := map[string]int{}
	for _, r := range rungs {
		counts[r] = 0
	}
	unbound := 0
	for _, c := range cs {
		if c.Binding == nil {
			unbound++
			continue
		}
		counts[c.Binding.Rung]++
	}
	total := len(cs) - unbound
	high := 0
	for r := range atGateOrAbove {
		high += counts[r]
	}
	percent := 0
	if total > 0 {
		percent = int(math.Round(float64(high) / float64(total) * 100))
	}
	return Coverage{Counts: counts, Unbound: unbound, Total: total, High: high, Percent: percent}
}

func CoverageLine(c Coverage) string {
	var parts []string
	for _, r := range rungs {
		parts = append(parts, fmt.Sprintf("`%s` %d", r, c.Counts[r]))
	}
	return strings.Join(parts, " · ") + fmt.Sprintf(" - **at gate or above: %d%%**", c.Percent)
}
