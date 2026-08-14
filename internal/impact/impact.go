package impact

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Simon0x/hr/internal/constraints"
	"github.com/bmatcuk/doublestar/v4"
)

var gateOrCoversRe = regexp.MustCompile(`(?m)^\s*//\s*@(gate|covers)\s+(\S+)\s*$`)

type Gate struct {
	Name   string
	Path   string
	Covers []string
}

func gitOutput(root string, args ...string) ([]string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func ChangedPaths(root, ref string) ([]string, error) {
	tracked, err := gitOutput(root, "diff", "--name-only", ref)
	if err != nil {
		return nil, err
	}
	untracked, err := gitOutput(root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, l := range tracked {
		set[l] = true
	}
	for _, l := range untracked {
		set[l] = true
	}
	var out []string
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// ReadGates finds every gate declared in cmd/hr/*.go. A gate is a
// `// @gate <name>` comment immediately followed by one or more
// `// @covers <glob>` comments, placed directly above the function that
// implements it — the Go equivalent of the old one-script-per-gate
// convention, since a single binary's subcommands no longer map one file
// per gate.
func ReadGates(root string) ([]Gate, error) {
	dir := filepath.Join(root, "cmd", "hr")
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	var gates []Gate
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		rel := "cmd/hr/" + filepath.Base(p)

		var current *Gate
		for _, m := range gateOrCoversRe.FindAllStringSubmatch(string(raw), -1) {
			switch m[1] {
			case "gate":
				if current != nil {
					gates = append(gates, *current)
				}
				current = &Gate{Name: m[2], Path: rel}
			case "covers":
				if current != nil {
					current.Covers = append(current.Covers, m[2])
				}
			}
		}
		if current != nil {
			gates = append(gates, *current)
		}
	}
	return gates, nil
}

type ConstraintRef struct {
	Text    string
	Binding string
}

type Row struct {
	Path        string
	Gates       []string
	Constraints []ConstraintRef
}

type BandInfo struct {
	Confidence string
	Status     string
	Paths      int
}

type Bands struct {
	Direct     BandInfo
	Transitive BandInfo
	Runtime    BandInfo
}

type Result struct {
	Base      string
	Changed   []string
	Covered   []Row
	Uncovered []Row
	GateOnly  []string
	Unscoped  []ConstraintRef
	Bands     Bands
}

func Derive(root, base string) (*Result, error) {
	paths, err := ChangedPaths(root, base)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return &Result{Base: base, Changed: []string{}}, nil
	}

	gates, err := ReadGates(root)
	if err != nil {
		return nil, err
	}
	agentsRaw, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		return nil, err
	}
	cs := constraints.Parse(string(agentsRaw))

	touchedGates := map[string]bool{}
	var covered, uncovered []Row

	for _, p := range paths {
		var coveringNames []string
		for _, g := range gates {
			for _, pattern := range g.Covers {
				if ok, _ := doublestar.Match(pattern, p); ok {
					coveringNames = append(coveringNames, g.Name)
					touchedGates[g.Name] = true
					break
				}
			}
		}

		var binds []ConstraintRef
		for _, c := range cs {
			if c.Binding == nil || c.Binding.Script == "" {
				continue
			}
			for _, gn := range coveringNames {
				if gn == c.Binding.Script {
					binds = append(binds, ConstraintRef{Text: c.Text, Binding: c.Binding.Raw})
					break
				}
			}
		}

		row := Row{Path: p, Gates: coveringNames, Constraints: binds}
		if len(coveringNames) > 0 {
			covered = append(covered, row)
		} else {
			uncovered = append(uncovered, row)
		}
	}

	var gateOnly []string
	for _, g := range gates {
		if !touchedGates[g.Name] {
			gateOnly = append(gateOnly, g.Name)
		}
	}

	var unscoped []ConstraintRef
	for _, c := range cs {
		if c.Binding == nil || c.Binding.Script == "" {
			raw := "unbound"
			if c.Binding != nil {
				raw = c.Binding.Raw
			}
			unscoped = append(unscoped, ConstraintRef{Text: c.Text, Binding: raw})
		}
	}

	return &Result{
		Base: base, Changed: paths, Covered: covered, Uncovered: uncovered,
		GateOnly: gateOnly, Unscoped: unscoped,
		Bands: Bands{
			Direct:     BandInfo{Confidence: "certain", Paths: len(paths)},
			Transitive: BandInfo{Confidence: "likely", Status: "not implemented - import graph is band 2"},
			Runtime:    BandInfo{Confidence: "unknown", Status: "no tool closes this - declared, per departments/engineering.md"},
		},
	}, nil
}
