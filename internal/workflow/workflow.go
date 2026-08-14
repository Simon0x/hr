package workflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

type Step struct {
	Department    string `json:"department"`
	Because       string `json:"because"`
	Input         string `json:"input"`
	Blocked       string `json:"blocked,omitempty"`
	Risk          string `json:"risk,omitempty"`
	Reversibility string `json:"reversibility,omitempty"`
	Confidence    string `json:"confidence,omitempty"`
}

type Plan struct {
	Artifacts int    `json:"artifacts"`
	Ready     []Step `json:"ready"`
	Blocked   []Step `json:"blocked"`
}

var evidenceRungs = []string{"none", "desk", "analogue", "stated", "behaviour", "commitment", "revenue"}
var gateRungIndex = indexOfStr(evidenceRungs, "behaviour")
var findingIDRe = regexp.MustCompile(`(?m)^### ([A-Z]+-\d+):`)

func indexOfStr(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}

func str(p map[string]any, key string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return ""
}

func strMap(p map[string]any, key string) map[string]string {
	out := map[string]string{}
	if m, ok := p[key].(map[string]any); ok {
		for k, v := range m {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

type chainLink struct {
	Link   string
	Status string
	Rung   string
}

func chainOf(p map[string]any) []chainLink {
	var out []chainLink
	arr, _ := p["chain"].([]any)
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, chainLink{Link: str(m, "link"), Status: str(m, "status"), Rung: str(m, "rung")})
	}
	return out
}

type acceptanceItem struct {
	State string
}

func acceptanceOf(p map[string]any) []acceptanceItem {
	var out []acceptanceItem
	arr, _ := p["acceptance"].([]any)
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, acceptanceItem{State: str(m, "state")})
	}
	return out
}

func confidenceFor(rung string) string {
	if rung == "revenue" || rung == "commitment" {
		return "certain"
	}
	if rung == "behaviour" {
		return "likely"
	}
	return "unknown"
}

func of(artifacts []store.Artifact, kind string) []store.Artifact {
	var out []store.Artifact
	for _, a := range artifacts {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	return out
}

func has(artifacts []store.Artifact, kind string, pred func(store.Artifact) bool) bool {
	for _, a := range of(artifacts, kind) {
		if pred(a) {
			return true
		}
	}
	return false
}

func firstSubjectDigest(subs []statement.Subject) string {
	if len(subs) == 0 {
		return ""
	}
	for _, v := range subs[0].Digest {
		return v
	}
	return ""
}

func subjectHasDigest(subs []statement.Subject, digest string) bool {
	if digest == "" {
		return false
	}
	for _, s := range subs {
		for _, v := range s.Digest {
			if v == digest {
				return true
			}
		}
	}
	return false
}

func goalRisk(artifacts []store.Artifact, findingID, hypothesisID, specID string) string {
	fID, hID := findingID, hypothesisID
	if specID != "" {
		for _, s := range of(artifacts, "spec") {
			if str(s.Predicate, "id") == specID {
				serves := strMap(s.Predicate, "serves")
				if fID == "" {
					fID = serves["findingId"]
				}
				if hID == "" {
					hID = serves["hypothesisId"]
				}
				break
			}
		}
	}
	for _, g := range of(artifacts, "goal") {
		serves := strMap(g.Predicate, "serves")
		if fID != "" && serves["findingId"] == fID {
			return str(g.Predicate, "risk")
		}
		if hID != "" && serves["hypothesisId"] == hID {
			return str(g.Predicate, "risk")
		}
	}
	return ""
}

func changeFor(artifacts []store.Artifact, v store.Artifact) *store.Artifact {
	for i, c := range artifacts {
		if c.Kind != "change" {
			continue
		}
		sha := firstSubjectDigest(c.Subject)
		if subjectHasDigest(v.Subject, sha) {
			return &artifacts[i]
		}
	}
	return nil
}

func OpenRegisterFindings(root string) (map[string]bool, error) {
	path := filepath.Join(root, "docs", "audit", "REGISTER.md")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, m := range findingIDRe.FindAllStringSubmatch(string(raw), -1) {
		out[m[1]] = true
	}
	return out, nil
}

func Derive(ctx context.Context, st persistence.Store, root string) (*Plan, error) {
	artifacts, err := st.Load(ctx)
	if err != nil {
		return nil, err
	}
	openFindings, err := OpenRegisterFindings(root)
	if err != nil {
		return nil, err
	}
	return DeriveFrom(artifacts, openFindings)
}

func DeriveFrom(artifacts []store.Artifact, openFindings map[string]bool) (*Plan, error) {
	ready := []Step{}
	blocked := []Step{}

	for _, goal := range of(artifacts, "goal") {
		serves := strMap(goal.Predicate, "serves")
		findingID := serves["findingId"]

		if findingID != "" && openFindings[findingID] {
			already := has(artifacts, "change", func(c store.Artifact) bool {
				return strMap(c.Predicate, "serves")["findingId"] == findingID
			})
			if !already {
				ready = append(ready, Step{
					Department: "Engineering",
					Because:    fmt.Sprintf("closes open finding %s — the second queue, no spec needed", findingID),
					Input:      fmt.Sprintf("goal %s — %s", goal.ID, str(goal.Predicate, "outcome")),
					Risk:       str(goal.Predicate, "risk"),
				})
			}
			continue
		}

		if findingID != "" && !openFindings[findingID] {
			blocked = append(blocked, Step{
				Department: "Engineering",
				Because:    fmt.Sprintf("names finding %s, which is not open in the register", findingID),
				Input:      fmt.Sprintf("goal %s", goal.ID),
				Blocked:    "re-ground the finding or drop the reference — a closed finding is not a queue",
			})
		} else if len(of(artifacts, "problem")) == 0 {
			ready = append(ready, Step{
				Department: "Discovery",
				Because:    "a goal exists with no problem stated under it",
				Input:      fmt.Sprintf("goal %s — %s", goal.ID, str(goal.Predicate, "outcome")),
			})
		}
	}

	for _, problem := range of(artifacts, "problem") {
		if len(of(artifacts, "hypothesis")) == 0 {
			ready = append(ready, Step{
				Department: "Discovery",
				Because:    "a problem is stated but no hypothesis has been chained from it",
				Input:      fmt.Sprintf("problem %s — %s", problem.ID, str(problem.Predicate, "who")),
			})
		}
	}

	for _, h := range of(artifacts, "hypothesis") {
		served := has(artifacts, "spec", func(s store.Artifact) bool {
			return strMap(s.Predicate, "serves")["hypothesisId"] == h.ID
		})
		if served {
			continue
		}

		var idxs []int
		for _, l := range chainOf(h.Predicate) {
			if l.Status == "breaks" {
				continue
			}
			idxs = append(idxs, indexOfStr(evidenceRungs, l.Rung))
		}
		weakest := 0
		if len(idxs) > 0 {
			weakest = idxs[0]
			for _, x := range idxs[1:] {
				if x < weakest {
					weakest = x
				}
			}
		}

		var blockedLinks []string
		for _, l := range chainOf(h.Predicate) {
			if l.Status == "blocked" {
				blockedLinks = append(blockedLinks, l.Link)
			}
		}

		breakStr := str(h.Predicate, "break")
		switch {
		case breakStr != "":
			blocked = append(blocked, Step{
				Department: "Discovery",
				Because:    fmt.Sprintf("link %q breaks", breakStr),
				Input:      fmt.Sprintf("hypothesis %s", h.ID),
				Blocked:    "re-chain or re-target; only a terminal break stops the search",
			})
		case len(blockedLinks) > 0:
			blocked = append(blocked, Step{
				Department: "Discovery",
				Because:    fmt.Sprintf("link %q blocked on tooling, not evidence", blockedLinks[0]),
				Input:      fmt.Sprintf("hypothesis %s", h.ID),
				Blocked:    "not a mechanism break - resolve the exception, then re-run this lead",
			})
		case weakest < gateRungIndex:
			blocked = append(blocked, Step{
				Department: "Product",
				Because:    fmt.Sprintf("hypothesis rests on `%s`", evidenceRungs[weakest]),
				Input:      fmt.Sprintf("hypothesis %s", h.ID),
				Blocked:    "no SPEC below `behaviour` — the only permitted output is a cheaper experiment",
			})
		default:
			ready = append(ready, Step{
				Department: "Product",
				Because:    fmt.Sprintf("hypothesis holds at `%s`", evidenceRungs[weakest]),
				Input:      fmt.Sprintf("hypothesis %s — %s", h.ID, str(h.Predicate, "claim")),
				Confidence: confidenceFor(evidenceRungs[weakest]),
			})
		}
	}

	for _, spec := range of(artifacts, "spec") {
		specID := str(spec.Predicate, "id")
		already := has(artifacts, "change", func(c store.Artifact) bool {
			return strMap(c.Predicate, "serves")["specId"] == specID
		})
		if already {
			continue
		}

		serves := strMap(spec.Predicate, "serves")
		hypID := serves["hypothesisId"]
		link := serves["link"]

		var rung string
		for _, h := range of(artifacts, "hypothesis") {
			if h.ID != hypID {
				continue
			}
			for _, l := range chainOf(h.Predicate) {
				if l.Link == link {
					rung = l.Rung
					break
				}
			}
			break
		}

		step := Step{
			Department: "Engineering",
			Because:    "a spec has no change serving it",
			Input:      fmt.Sprintf("%s — %s", specID, str(spec.Predicate, "intent")),
			Risk:       goalRisk(artifacts, serves["findingId"], serves["hypothesisId"], ""),
		}
		if rung != "" {
			step.Confidence = confidenceFor(rung)
		}
		ready = append(ready, step)
	}

	for _, change := range of(artifacts, "change") {
		sha := firstSubjectDigest(change.Subject)
		graded := has(artifacts, "verdict", func(v store.Artifact) bool {
			return subjectHasDigest(v.Subject, sha)
		})
		if graded {
			continue
		}

		serves := strMap(change.Predicate, "serves")
		rung := str(change.Predicate, "rung")
		ready = append(ready, Step{
			Department:    "QA",
			Because:       "a change has no verdict against it",
			Input:         fmt.Sprintf("change %s at `%s`", change.ID, rung),
			Risk:          goalRisk(artifacts, serves["findingId"], "", serves["specId"]),
			Reversibility: rung,
		})
	}

	for _, v := range of(artifacts, "verdict") {
		hasRelease := has(artifacts, "release", func(r store.Artifact) bool {
			return str(r.Predicate, "servesVerdict") == v.ID
		})
		if hasRelease {
			continue
		}

		var unmetCount, unverifiableCount int
		for _, a := range acceptanceOf(v.Predicate) {
			switch a.State {
			case "unmet":
				unmetCount++
			case "unverifiable":
				unverifiableCount++
			}
		}

		change := changeFor(artifacts, v)
		var risk, reversibility string
		var serves map[string]string
		if change != nil {
			serves = strMap(change.Predicate, "serves")
			risk = goalRisk(artifacts, serves["findingId"], "", serves["specId"])
			reversibility = str(change.Predicate, "rung")
		}

		if unmetCount == 0 && unverifiableCount == 0 {
			ready = append(ready, Step{
				Department: "Release", Because: "every criterion met and no verdict has been acted on",
				Input: fmt.Sprintf("verdict %s", v.ID), Risk: risk, Reversibility: reversibility,
			})
			continue
		}

		fixInFlight := false
		if change != nil {
			fixInFlight = has(artifacts, "change", func(c store.Artifact) bool {
				if c.ID == change.ID {
					return false
				}
				cs := strMap(c.Predicate, "serves")
				matches := (serves["specId"] != "" && cs["specId"] == serves["specId"]) ||
					(serves["findingId"] != "" && cs["findingId"] == serves["findingId"])
				if !matches {
					return false
				}
				csha := firstSubjectDigest(c.Subject)
				return !has(artifacts, "verdict", func(v2 store.Artifact) bool {
					return subjectHasDigest(v2.Subject, csha)
				})
			})
		}
		if fixInFlight {
			continue
		}

		if unmetCount > 0 {
			ready = append(ready, Step{
				Department: "Engineering",
				Because:    fmt.Sprintf("verdict %s has %d unmet criterion/criteria — evidence says no", v.ID, unmetCount),
				Input:      fmt.Sprintf("verdict %s", v.ID), Risk: risk, Reversibility: reversibility,
			})
		}
		if unverifiableCount > 0 {
			ready = append(ready, Step{
				Department: "Product",
				Because:    fmt.Sprintf("verdict %s has %d unverifiable criterion/criteria — names a Product gap", v.ID, unverifiableCount),
				Input:      fmt.Sprintf("verdict %s", v.ID), Risk: risk,
			})
		}
	}

	for _, r := range of(artifacts, "release") {
		rehearsed, _ := r.Predicate["rollbackRehearsed"].(bool)
		if !rehearsed {
			blocked = append(blocked, Step{
				Department: "Release",
				Because:    "the rollback path has not been rehearsed",
				Input:      fmt.Sprintf("release %s", r.ID),
				Blocked:    "a down path that has never been run is a down path that does not exist",
			})
		} else {
			ready = append(ready, Step{
				Department: "Ops",
				Because:    "a release is live and nothing is watching it yet",
				Input:      fmt.Sprintf("release %s — %s", r.ID, str(r.Predicate, "artifactDigest")),
			})
		}
	}

	return &Plan{Artifacts: len(artifacts), Ready: ready, Blocked: blocked}, nil
}
