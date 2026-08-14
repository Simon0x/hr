package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/store"
)

type Job struct {
	ID            int64
	StepKey       string
	Department    string
	Because       string
	Input         string
	Risk          string
	Reversibility string
	Confidence    string
	Status        string
	Attempts      int
	ResultDigest  *string
	Detail        *string
	CreatedAt     time.Time
	ClaimedAt     *time.Time
}

type ClaimedJob struct {
	Job
	ClaimedBy      string
	LeaseToken     string
	LeaseExpiresAt time.Time
}

// owner is empty for system-derived jobs (visible to everyone) or a real
// identity's spiffe_id for a job manually run from the web UI (visible only
// to that identity) - see ListJobs.
func InsertJob(ctx context.Context, db querier, stepKey, department, because, input, risk, reversibility, confidence, owner string) (job Job, inserted bool, err error) {
	err = db.QueryRow(ctx, `
		INSERT INTO jobs (step_key, department, because, input, risk, reversibility, confidence, owner)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (step_key) WHERE status IN ('pending','claimed','running','escalated','quarantined') DO NOTHING
		RETURNING id, step_key, department, because, input, risk, reversibility, confidence, status, attempts`,
		stepKey, department, because, input, risk, reversibility, confidence, owner,
	).Scan(&job.ID, &job.StepKey, &job.Department, &job.Because, &job.Input,
		&job.Risk, &job.Reversibility, &job.Confidence, &job.Status, &job.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return job, true, nil
}

func ClaimJob(ctx context.Context, db querier, claimedBy string, departments []string, lease time.Duration) (*ClaimedJob, error) {
	var j ClaimedJob
	err := db.QueryRow(ctx, `
		UPDATE jobs SET status = 'claimed', claimed_by = $1,
		  lease_token = gen_random_uuid(), claimed_at = now(),
		  lease_expires_at = now() + make_interval(secs => $2)
		WHERE id = (
		  SELECT id FROM jobs
		  WHERE status = 'pending' AND department = ANY($3)
		  ORDER BY risk DESC, created_at
		  FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING id, step_key, department, because, input, risk, reversibility,
		          confidence, status, attempts, claimed_by, lease_token::text, lease_expires_at`,
		claimedBy, lease.Seconds(), departments,
	).Scan(&j.ID, &j.StepKey, &j.Department, &j.Because, &j.Input, &j.Risk,
		&j.Reversibility, &j.Confidence, &j.Status, &j.Attempts,
		&j.ClaimedBy, &j.LeaseToken, &j.LeaseExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func CompleteJob(ctx context.Context, db beginner, jobID int64, claimedBy, leaseToken, outcome string, artifact store.Artifact, canonical []byte, entry ledger.Entry, prior ...ledger.Entry) (ok bool, written ledger.Entry, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return false, ledger.Entry{}, err
	}
	defer tx.Rollback(ctx)

	var status string
	err = tx.QueryRow(ctx, `
		SELECT status FROM jobs
		WHERE id = $1 AND claimed_by = $2 AND lease_token = $3::uuid
		  AND status IN ('claimed','running')
		FOR UPDATE`,
		jobID, claimedBy, leaseToken,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ledger.Entry{}, nil
	}
	if err != nil {
		return false, ledger.Entry{}, err
	}

	// Prior invocations happened before the terminal entry and are appended
	// ahead of it, in the same transaction: either the whole job's record
	// lands or none of it does.
	for _, p := range prior {
		if _, err := Append(ctx, tx, p); err != nil {
			return false, ledger.Entry{}, err
		}
	}

	if outcome == "failed" {
		written, err = Append(ctx, tx, entry)
		if err != nil {
			return false, ledger.Entry{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE jobs SET status = 'failed', ledger_seq = $1 WHERE id = $2`,
			written.Seq, jobID,
		); err != nil {
			return false, ledger.Entry{}, err
		}
	} else {
		if _, err := InsertArtifact(ctx, tx, artifact, canonical, claimedBy); err != nil {
			return false, ledger.Entry{}, err
		}
		written, err = Append(ctx, tx, entry)
		if err != nil {
			return false, ledger.Entry{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE jobs SET status = 'done', result_digest = $1, ledger_seq = $2 WHERE id = $3`,
			artifact.ID, written.Seq, jobID,
		); err != nil {
			return false, ledger.Entry{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, ledger.Entry{}, err
	}
	return true, written, nil
}

// ListJobs returns jobs visible to viewer: every job with no owner (shared,
// system-derived) plus any owned by viewer itself.
func ListJobs(ctx context.Context, db querier, limit int, viewer string) ([]Job, error) {
	rows, err := db.Query(ctx, `
		SELECT j.id, j.step_key, j.department, j.because, j.input, j.risk, j.reversibility, j.confidence,
		       j.status, j.attempts, j.result_digest, j.created_at, j.claimed_at, NULLIF(le.detail, '')
		FROM jobs j
		LEFT JOIN ledger_entries le ON le.seq = j.ledger_seq
		WHERE j.owner = '' OR j.owner = $2
		ORDER BY j.created_at DESC LIMIT $1`, limit, viewer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.StepKey, &j.Department, &j.Because, &j.Input,
			&j.Risk, &j.Reversibility, &j.Confidence, &j.Status, &j.Attempts,
			&j.ResultDigest, &j.CreatedAt, &j.ClaimedAt, &j.Detail); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// QuarantineRepeatedFailures stops a step that keeps failing from being
// re-derived and reclaimed forever.
//
// It quarantines the step's *live* row, not the newest failed one. The
// repopulate loop reinserts a pending job after each failure, so by the time
// the threshold trips there is already a fresh active row; marking a failed
// row quarantined would add a second active row for one step key and collide
// with the unique index - failing exactly when the watchdog is needed.
func QuarantineRepeatedFailures(ctx context.Context, db querier, threshold int, window time.Duration, reason string) ([]Job, error) {
	rows, err := db.Query(ctx, `
		WITH recent_failures AS (
			SELECT step_key, COUNT(*) AS n
			FROM jobs
			WHERE status = 'failed' AND created_at > now() - make_interval(secs => $2)
			GROUP BY step_key
			HAVING COUNT(*) >= $1
		)
		UPDATE jobs SET status = 'quarantined', quarantined_reason = $3
		FROM recent_failures
		WHERE jobs.step_key = recent_failures.step_key
		  AND jobs.status IN ('pending', 'claimed', 'running')
		RETURNING jobs.id, jobs.step_key, jobs.department, jobs.because, jobs.input,
		          jobs.risk, jobs.reversibility, jobs.confidence, jobs.status, jobs.attempts`,
		threshold, window.Seconds(), reason,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.StepKey, &j.Department, &j.Because, &j.Input,
			&j.Risk, &j.Reversibility, &j.Confidence, &j.Status, &j.Attempts); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func ReapExpiredLeases(ctx context.Context, db querier, attemptsLimit int) (reset, escalated int, err error) {
	tag, err := db.Exec(ctx, `
		UPDATE jobs SET status = 'pending', claimed_by = NULL, lease_token = NULL,
		  claimed_at = NULL, lease_expires_at = NULL, attempts = attempts + 1
		WHERE status IN ('claimed','running') AND lease_expires_at < now()
		  AND attempts < $1`,
		attemptsLimit)
	if err != nil {
		return 0, 0, err
	}
	reset = int(tag.RowsAffected())

	tag, err = db.Exec(ctx, `
		UPDATE jobs SET status = 'escalated'
		WHERE status IN ('claimed','running') AND lease_expires_at < now()
		  AND attempts >= $1`,
		attemptsLimit)
	if err != nil {
		return reset, 0, err
	}
	escalated = int(tag.RowsAffected())

	return reset, escalated, nil
}
