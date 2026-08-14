package exceptions

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/store"
)

const DefaultActor = "spiffe://hr.local/exceptions"

type Exception struct {
	Digest            string   `json:"digest"`
	Department        string   `json:"department"`
	Because           string   `json:"because"`
	Class             string   `json:"class"`
	Options           []string `json:"options"`
	Recommendation    string   `json:"recommendation"`
	Uncertainty       string   `json:"uncertainty,omitempty"`
	Consequence       string   `json:"consequence"`
	Deadline          string   `json:"deadline,omitempty"`
	RequiredAuthority string   `json:"requiredAuthority,omitempty"`
	StepKey           string   `json:"stepKey,omitempty"`
	// Owner is whoever filed this exception (artifact.CreatedBy). A system
	// actor (e.g. spiffe://hr.local/hr-server) leaves it visible to everyone;
	// a real identity's spiffe_id makes it private - see pgstore.OpenExceptions.
	Owner string `json:"-"`
}

var consequenceRank = map[string]int{"R4": 4, "R3": 3, "R2": 2, "R1": 1, "R0": 0}

func SortOpen(out []Exception) {
	sort.Slice(out, func(i, j int) bool {
		ri, rj := consequenceRank[out[i].Consequence], consequenceRank[out[j].Consequence]
		if ri != rj {
			return ri > rj
		}
		di, dj := out[i].Deadline, out[j].Deadline
		if di == "" || dj == "" {
			return dj == "" && di != ""
		}
		return di < dj
	})
}

func Open(ctx context.Context, st persistence.Store, root string) ([]Exception, error) {
	artifacts, err := st.Load(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := st.Read(ctx)
	if err != nil {
		return nil, err
	}

	resolved := map[string]bool{}
	for _, e := range entries {
		if e.Kind != "intervention" || e.Artifacts == nil {
			continue
		}
		for _, in := range e.Artifacts.In {
			resolved[in] = true
		}
	}

	var out []Exception
	for _, a := range artifacts {
		if a.Kind != "exception" || resolved[a.ID] {
			continue
		}
		out = append(out, FromArtifact(a))
	}

	SortOpen(out)
	return out, nil
}

func ByDigest(ctx context.Context, st persistence.Store, root, digest string) (Exception, bool, error) {
	all, err := Open(ctx, st, root)
	if err != nil {
		return Exception{}, false, err
	}
	for _, e := range all {
		if e.Digest == digest {
			return e, true, nil
		}
	}
	return Exception{}, false, nil
}

func ValidOption(exc Exception, option string) bool {
	for _, o := range exc.Options {
		if o == option {
			return true
		}
	}
	return false
}

func Resolve(ctx context.Context, st persistence.Store, root, actor string, exc Exception, option string) error {
	if !ValidOption(exc, option) {
		return fmt.Errorf("%q is not one of this exception's options", option)
	}
	if actor == "" {
		actor = DefaultActor
	}
	_, err := st.Append(ctx, ledger.Entry{
		Kind:      "intervention",
		Actor:     actor,
		Artifacts: &ledger.Artifacts{In: []string{exc.Digest}, Out: []string{}},
		Outcome:   "ok",
		Detail:    option,
	})
	return err
}

func FromArtifact(a store.Artifact) Exception {
	e := Exception{Digest: a.ID, Owner: a.CreatedBy}
	if len(a.Subject) > 0 {
		e.Department = strings.TrimSuffix(a.Subject[0].Name, "-exception")
	}
	if v, ok := a.Predicate["because"].(string); ok {
		e.Because = v
	}
	if v, ok := a.Predicate["class"].(string); ok {
		e.Class = v
	}
	if v, ok := a.Predicate["recommendation"].(string); ok {
		e.Recommendation = v
	}
	if v, ok := a.Predicate["uncertainty"].(string); ok {
		e.Uncertainty = v
	}
	if v, ok := a.Predicate["consequence"].(string); ok {
		e.Consequence = v
	}
	if v, ok := a.Predicate["deadline"].(string); ok {
		e.Deadline = v
	}
	if v, ok := a.Predicate["requiredAuthority"].(string); ok {
		e.RequiredAuthority = v
	}
	if v, ok := a.Predicate["stepKey"].(string); ok {
		e.StepKey = v
	}
	if raw, ok := a.Predicate["options"].([]any); ok {
		for _, o := range raw {
			if s, ok := o.(string); ok {
				e.Options = append(e.Options, s)
			}
		}
	}
	return e
}
