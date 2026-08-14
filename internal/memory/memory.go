package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Simon0x/hr/internal/statement"
)

func Dir(root string) string          { return filepath.Join(root, ".hr", "memory") }
func ForgottenDir(root string) string { return filepath.Join(Dir(root), "forgotten") }

type Memory struct {
	Digest    string
	Predicate map[string]any
}

func Load(root string) ([]Memory, error) {
	dir := Dir(root)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []Memory
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var stmt statement.Statement
		if err := json.Unmarshal(raw, &stmt); err != nil {
			continue
		}
		var pred map[string]any
		_ = json.Unmarshal(stmt.Predicate, &pred)
		digest := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "memory-"), ".json")
		out = append(out, Memory{Digest: digest, Predicate: pred})
	}
	return out, nil
}

var wordRe = regexp.MustCompile(`[a-z][a-z0-9-]{2,}`)

func Words(s string) []string {
	lower := strings.ToLower(s)
	var out []string
	for _, m := range wordRe.FindAllString(lower, -1) {
		if len(m) <= 30 {
			out = append(out, m)
		}
	}
	return out
}

func scopeString(p map[string]any) string {
	arr, _ := p["scope"].([]any)
	var parts []string
	for _, s := range arr {
		if str, ok := s.(string); ok {
			parts = append(parts, str)
		}
	}
	return strings.Join(parts, " ")
}

var daysPattern = regexp.MustCompile(`^(\d+)`)

func parseDays(s string) int {
	m := daysPattern.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func IsStale(m Memory, now time.Time) (bool, string) {
	ve, _ := m.Predicate["verifyEvery"].(string)
	if ve == "" {
		return false, ""
	}
	days := parseDays(ve)
	if days == 0 {
		return false, ""
	}
	lastVerified, _ := m.Predicate["lastVerified"].(string)
	t, err := time.Parse(time.RFC3339, lastVerified)
	if err != nil {
		return false, ""
	}
	if now.Sub(t) > time.Duration(days)*24*time.Hour {
		return true, "stale — not verified since " + lastVerified + ", due every " + ve
	}
	return false, ""
}

func IsExpired(m Memory, now time.Time) bool {
	tu, _ := m.Predicate["trueUntil"].(string)
	if tu == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, tu)
	if err != nil {
		return false
	}
	return t.Before(now)
}

type Hit struct {
	Digest    string
	Predicate map[string]any
}

type Withheld struct {
	Digest string
	Claim  string
	Why    string
}

type RecallResult struct {
	Matched  []string
	Hits     []Hit
	Withheld []Withheld
}

func Recall(root, question, scope string, now time.Time) (*RecallResult, error) {
	memories, err := Load(root)
	if err != nil {
		return nil, err
	}
	return RecallFrom(memories, question, scope, now), nil
}

func RecallFrom(memories []Memory, question, scope string, now time.Time) *RecallResult {
	superseded := map[string]bool{}
	for _, m := range memories {
		if s, ok := m.Predicate["supersedes"].(string); ok && s != "" {
			superseded[s] = true
		}
	}

	vocab := map[string]bool{}
	for _, m := range memories {
		claim, _ := m.Predicate["claim"].(string)
		for _, w := range Words(claim + " " + scopeString(m.Predicate)) {
			vocab[w] = true
		}
	}

	var matched []string
	seen := map[string]bool{}
	for _, w := range Words(question) {
		if vocab[w] && !seen[w] {
			matched = append(matched, w)
			seen[w] = true
		}
	}
	if matched == nil {
		matched = []string{}
	}
	if len(matched) == 0 {
		return &RecallResult{Matched: matched, Hits: []Hit{}, Withheld: []Withheld{}}
	}

	type scored struct {
		m     Memory
		score int
	}
	var candidates []scored
	for _, m := range memories {
		if superseded[m.Digest] || IsExpired(m, now) {
			continue
		}
		if scope != "" {
			arr, _ := m.Predicate["scope"].([]any)
			found := false
			for _, s := range arr {
				if str, ok := s.(string); ok && strings.Contains(str, scope) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		claim, _ := m.Predicate["claim"].(string)
		memWords := map[string]bool{}
		for _, w := range Words(claim + " " + scopeString(m.Predicate)) {
			memWords[w] = true
		}
		score := 0
		for _, w := range matched {
			if memWords[w] {
				score++
			}
		}
		if score == 0 {
			continue
		}
		candidates = append(candidates, scored{m, score})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	live := []Hit{}
	withheld := []Withheld{}
	for _, c := range candidates {
		quarantined, _ := c.m.Predicate["quarantined"].(bool)
		isStale, reason := IsStale(c.m, now)
		claim, _ := c.m.Predicate["claim"].(string)
		switch {
		case quarantined:
			withheld = append(withheld, Withheld{Digest: c.m.Digest, Claim: claim, Why: "quarantined"})
		case isStale:
			withheld = append(withheld, Withheld{Digest: c.m.Digest, Claim: claim, Why: reason})
		default:
			live = append(live, Hit{Digest: c.m.Digest, Predicate: c.m.Predicate})
		}
	}

	return &RecallResult{Matched: matched, Hits: live, Withheld: withheld}
}
