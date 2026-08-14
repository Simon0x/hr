package pgstore

import (
	"context"
	"fmt"

	"github.com/Simon0x/hr/internal/exceptions"
	"github.com/Simon0x/hr/internal/ledger"
)

// OpenExceptions returns exceptions visible to viewer: every exception filed
// by a system actor (its owner matches no real identity, e.g.
// spiffe://hr.local/hr-server), plus any filed by viewer itself. One filed
// by a different real identity is not returned - it is private to them.
func OpenExceptions(ctx context.Context, db querier, viewer string) ([]exceptions.Exception, error) {
	artifacts, err := LoadArtifacts(ctx, db)
	if err != nil {
		return nil, err
	}
	entries, err := Read(ctx, db)
	if err != nil {
		return nil, err
	}
	known, err := KnownIdentitySet(ctx, db)
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

	var out []exceptions.Exception
	for _, a := range artifacts {
		if a.Kind != "exception" || resolved[a.ID] {
			continue
		}
		exc := exceptions.FromArtifact(a)
		if known[exc.Owner] && exc.Owner != viewer {
			continue
		}
		out = append(out, exc)
	}

	exceptions.SortOpen(out)
	return out, nil
}

func ExceptionByDigest(ctx context.Context, db querier, digest, viewer string) (exceptions.Exception, bool, error) {
	all, err := OpenExceptions(ctx, db, viewer)
	if err != nil {
		return exceptions.Exception{}, false, err
	}
	for _, e := range all {
		if e.Digest == digest {
			return e, true, nil
		}
	}
	return exceptions.Exception{}, false, nil
}

func ResolveException(ctx context.Context, db beginner, exc exceptions.Exception, actor, option string) (ledger.Entry, error) {
	if !exceptions.ValidOption(exc, option) {
		return ledger.Entry{}, fmt.Errorf("%q is not one of this exception's options", option)
	}
	entry, err := Append(ctx, db, ledger.Entry{
		Kind:      "intervention",
		Actor:     actor,
		Artifacts: &ledger.Artifacts{In: []string{exc.Digest}, Out: []string{}},
		Outcome:   "ok",
		Detail:    option,
	})
	if err != nil {
		return entry, err
	}

	if exc.StepKey != "" {
		if _, err := db.Exec(ctx, `
			UPDATE jobs SET status = 'pending', quarantined_reason = NULL
			WHERE step_key = $1 AND status = 'quarantined'`,
			exc.StepKey,
		); err != nil {
			return entry, fmt.Errorf("resolved but could not un-quarantine %s: %w", exc.StepKey, err)
		}
		_ = NotifyJobsChanged(ctx, db)
	}

	return entry, nil
}
