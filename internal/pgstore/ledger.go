package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Simon0x/hr/internal/ledger"
)

const ledgerAppendLockKey = 727001

const entryColumns = `seq, prev, at, kind, actor, has_artifacts, artifacts_in, artifacts_out,
	goal, policy, outcome, detail, has_cost, cost_tokens, cost_seconds, cost_currency, cost_amount, cost_model,
	has_request, request_harness, request_prompt_digest, request_tools, request_denied_tools, request_procedure, request_procedure_digest, request_untrusted_input`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEntry(row rowScanner) (ledger.Entry, error) {
	var (
		e                                 ledger.Entry
		seq                               int64
		hasArtifacts, hasCost, hasRequest bool
		artifactsIn, artifactsOut         []string
		costTokens, costSeconds           *float64
		costCurrency, costModel           string
		costAmount                        *float64
		reqHarness, reqPromptDigest       string
		reqTools, reqDeniedTools          []string
		reqProcedure, reqProcedureDigest  string
		reqUntrustedInput                 bool
	)
	if err := row.Scan(&seq, &e.Prev, &e.At, &e.Kind, &e.Actor,
		&hasArtifacts, &artifactsIn, &artifactsOut,
		&e.Goal, &e.Policy, &e.Outcome, &e.Detail,
		&hasCost, &costTokens, &costSeconds, &costCurrency, &costAmount, &costModel,
		&hasRequest, &reqHarness, &reqPromptDigest, &reqTools, &reqDeniedTools, &reqProcedure, &reqProcedureDigest, &reqUntrustedInput,
	); err != nil {
		return ledger.Entry{}, err
	}
	e.Seq = int(seq)
	if hasArtifacts {
		e.Artifacts = &ledger.Artifacts{In: nonNil(artifactsIn), Out: nonNil(artifactsOut)}
	}
	if hasCost {
		e.Cost = &ledger.Cost{
			Tokens: costTokens, Seconds: costSeconds,
			Currency: costCurrency, Amount: costAmount, Model: costModel,
		}
	}
	if hasRequest {
		e.Request = &ledger.Request{
			Harness: reqHarness, PromptDigest: reqPromptDigest, Tools: nonNil(reqTools),
			DeniedTools: emptyToNil(reqDeniedTools),
			Procedure:   reqProcedure, ProcedureDigest: reqProcedureDigest,
			UntrustedInput: reqUntrustedInput,
		}
	}
	return e, nil
}

// emptyToNil keeps an absent deny list absent through a round trip: the
// column defaults to '{}', and turning that into an empty JSON array would
// make every pre-existing entry re-canonicalize differently and break the chain.
func emptyToNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func insertEntry(ctx context.Context, tx pgx.Tx, e ledger.Entry) error {
	hasArtifacts := e.Artifacts != nil
	artifactsIn, artifactsOut := []string{}, []string{}
	if hasArtifacts {
		artifactsIn, artifactsOut = nonNil(e.Artifacts.In), nonNil(e.Artifacts.Out)
	}

	hasCost := e.Cost != nil
	var costTokens, costSeconds, costAmount *float64
	var costCurrency, costModel string
	if hasCost {
		costTokens, costSeconds = e.Cost.Tokens, e.Cost.Seconds
		costCurrency, costAmount, costModel = e.Cost.Currency, e.Cost.Amount, e.Cost.Model
	}

	hasRequest := e.Request != nil
	var reqHarness, reqPromptDigest, reqProcedure, reqProcedureDigest string
	var reqUntrustedInput bool
	reqTools, reqDeniedTools := []string{}, []string{}
	if hasRequest {
		reqHarness, reqPromptDigest = e.Request.Harness, e.Request.PromptDigest
		reqTools = nonNil(e.Request.Tools)
		reqDeniedTools = nonNil(e.Request.DeniedTools)
		reqProcedure, reqProcedureDigest = e.Request.Procedure, e.Request.ProcedureDigest
		reqUntrustedInput = e.Request.UntrustedInput
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO ledger_entries
			(seq, prev, at, kind, actor, has_artifacts, artifacts_in, artifacts_out,
			 goal, policy, outcome, detail, has_cost, cost_tokens, cost_seconds, cost_currency, cost_amount, cost_model,
			 has_request, request_harness, request_prompt_digest, request_tools, request_denied_tools, request_procedure, request_procedure_digest, request_untrusted_input)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)`,
		e.Seq, e.Prev, e.At, e.Kind, e.Actor, hasArtifacts, artifactsIn, artifactsOut,
		e.Goal, e.Policy, e.Outcome, e.Detail, hasCost, costTokens, costSeconds, costCurrency, costAmount, costModel,
		hasRequest, reqHarness, reqPromptDigest, reqTools, reqDeniedTools, reqProcedure, reqProcedureDigest, reqUntrustedInput)
	return err
}

func Read(ctx context.Context, db querier) ([]ledger.Entry, error) {
	rows, err := db.Query(ctx, `SELECT `+entryColumns+` FROM ledger_entries ORDER BY seq`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ledger.Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func Append(ctx context.Context, db beginner, e ledger.Entry) (ledger.Entry, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return ledger.Entry{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(ledgerAppendLockKey)); err != nil {
		return ledger.Entry{}, err
	}

	row := tx.QueryRow(ctx, `SELECT `+entryColumns+` FROM ledger_entries ORDER BY seq DESC LIMIT 1`)
	tail, err := scanEntry(row)
	tailExists := true
	if errors.Is(err, pgx.ErrNoRows) {
		tailExists = false
	} else if err != nil {
		return ledger.Entry{}, err
	}

	prev := ledger.Genesis
	seq := 0
	if tailExists {
		digest, derr := ledger.Digest(tail)
		if derr != nil {
			return ledger.Entry{}, derr
		}
		prev = digest
		seq = tail.Seq + 1
	}

	e.Seq = seq
	e.Prev = prev
	if e.At == "" {
		e.At = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	}

	if err := insertEntry(ctx, tx, e); err != nil {
		return ledger.Entry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ledger.Entry{}, err
	}
	return e, nil
}
