package pgstore

import (
	"context"

	"github.com/Simon0x/hr/internal/memory"
)

func LoadMemories(ctx context.Context, db querier) ([]memory.Memory, error) {
	rows, err := db.Query(ctx, `
		SELECT digest, claim, confidence, source, owner, learned_at, true_from,
		       true_until, last_verified, verify_every, scope, data_class,
		       supersedes, quarantined, promoted_to
		FROM memories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []memory.Memory
	for rows.Next() {
		var (
			digest, claim, confidence, source, owner                  string
			learnedAt, trueFrom, trueUntil, lastVerified, verifyEvery string
			dataClass, supersedes, promotedTo                         string
			scope                                                     []string
			quarantined                                               bool
		)
		if err := rows.Scan(&digest, &claim, &confidence, &source, &owner, &learnedAt,
			&trueFrom, &trueUntil, &lastVerified, &verifyEvery, &scope, &dataClass,
			&supersedes, &quarantined, &promotedTo,
		); err != nil {
			return nil, err
		}

		scopeAny := make([]any, len(scope))
		for i, s := range scope {
			scopeAny[i] = s
		}

		out = append(out, memory.Memory{
			Digest: digest,
			Predicate: map[string]any{
				"claim":        claim,
				"confidence":   confidence,
				"source":       source,
				"owner":        owner,
				"learnedAt":    learnedAt,
				"trueFrom":     trueFrom,
				"trueUntil":    trueUntil,
				"lastVerified": lastVerified,
				"verifyEvery":  verifyEvery,
				"scope":        scopeAny,
				"dataClass":    dataClass,
				"supersedes":   supersedes,
				"quarantined":  quarantined,
				"promotedTo":   promotedTo,
			},
		})
	}
	return out, rows.Err()
}
