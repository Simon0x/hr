package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

func LoadArtifacts(ctx context.Context, db querier) ([]store.Artifact, error) {
	rows, err := db.Query(ctx, `SELECT digest, kind, predicate_type, subject, predicate, created_by FROM artifacts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.Artifact
	for rows.Next() {
		var digest, kind, predicateType, createdBy string
		var subjectRaw, predicateRaw []byte
		if err := rows.Scan(&digest, &kind, &predicateType, &subjectRaw, &predicateRaw, &createdBy); err != nil {
			return nil, err
		}
		var subject []statement.Subject
		if err := json.Unmarshal(subjectRaw, &subject); err != nil {
			return nil, err
		}
		var predicate map[string]any
		if err := json.Unmarshal(predicateRaw, &predicate); err != nil {
			return nil, err
		}
		out = append(out, store.Artifact{
			ID: digest, Kind: kind, PredicateType: predicateType,
			Subject: subject, Predicate: predicate, CreatedBy: createdBy,
		})
	}
	return out, rows.Err()
}

func GetArtifact(ctx context.Context, db querier, digest string) (store.Artifact, bool, error) {
	var a store.Artifact
	var subjectRaw, predicateRaw []byte
	err := db.QueryRow(ctx,
		`SELECT digest, kind, predicate_type, subject, predicate, created_by FROM artifacts WHERE digest = $1`,
		digest,
	).Scan(&a.ID, &a.Kind, &a.PredicateType, &subjectRaw, &predicateRaw, &a.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Artifact{}, false, nil
	}
	if err != nil {
		return store.Artifact{}, false, err
	}
	if err := json.Unmarshal(subjectRaw, &a.Subject); err != nil {
		return store.Artifact{}, false, err
	}
	if err := json.Unmarshal(predicateRaw, &a.Predicate); err != nil {
		return store.Artifact{}, false, err
	}
	return a, true, nil
}

// ListArtifactHistory returns artifacts newest-first for browsing the audit
// trail beyond the last-50-jobs window, optionally filtered by kind and
// paginated via before (the created_at of the last item on the previous
// page; zero value starts from the most recent). Excludes kind "exception" -
// those can be private to an identity (see OpenExceptions) unlike the rest
// of the GOAL->...->RELEASE chain, which is shared project state; browse
// exceptions via GET /v1/exceptions instead.
func ListArtifactHistory(ctx context.Context, db querier, limit int, before *time.Time, kind string) ([]store.Artifact, error) {
	rows, err := db.Query(ctx, `
		SELECT digest, kind, predicate_type, subject, predicate, created_by, created_at
		FROM artifacts
		WHERE kind != 'exception'
		  AND ($2 = '' OR kind = $2)
		  AND ($3::timestamptz IS NULL OR created_at < $3)
		ORDER BY created_at DESC
		LIMIT $1`,
		limit, kind, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.Artifact
	for rows.Next() {
		var a store.Artifact
		var subjectRaw, predicateRaw []byte
		if err := rows.Scan(&a.ID, &a.Kind, &a.PredicateType, &subjectRaw, &predicateRaw, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(subjectRaw, &a.Subject); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(predicateRaw, &a.Predicate); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func InsertArtifact(ctx context.Context, db querier, a store.Artifact, canonical []byte, createdBy string) (bool, error) {
	subjectRaw, err := json.Marshal(a.Subject)
	if err != nil {
		return false, err
	}
	predicateRaw, err := json.Marshal(a.Predicate)
	if err != nil {
		return false, err
	}
	tag, err := db.Exec(ctx,
		`INSERT INTO artifacts (digest, kind, predicate_type, subject, predicate, canonical, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (digest) DO NOTHING`,
		a.ID, a.Kind, a.PredicateType, string(subjectRaw), string(predicateRaw), canonical, createdBy)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
