package hrserver

import (
	"context"
	"testing"
	"time"

	"github.com/Simon0x/hr/internal/exceptions"
	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence/pgprovider"
	"github.com/Simon0x/hr/internal/pgstore"
	"github.com/Simon0x/hr/internal/store"
)

// The loop a human needs: a step that keeps failing is quarantined so it
// stops burning agent calls, and resolving the exception that reports it puts
// the step back in the queue. Without the second half, the quarantine trades
// unbounded retry for unbounded block.
func TestQuarantinedStep_ResumesWhenItsExceptionIsResolved(t *testing.T) {
	pool := testPool(t)
	root := testRoot(t)
	reg := testRegistry(t, root)
	ctx := context.Background()
	st := pgprovider.Postgres{Pool: pool}

	const stepKey = "quarantine01"
	if _, _, err := pgstore.InsertJob(ctx, pool, stepKey, "QA", "verify", "in", "R1", "revert", "likely", ""); err != nil {
		t.Fatal(err)
	}

	// Fail it enough times to trip the watchdog's threshold.
	for i := 0; i < 3; i++ {
		claimed, err := pgstore.ClaimJob(ctx, pool, "spiffe://hr.local/w", []string{"QA"}, time.Minute)
		if err != nil || claimed == nil {
			t.Fatalf("claim %d: %v %v", i, claimed, err)
		}
		if ok, _, err := pgstore.CompleteJob(ctx, pool, claimed.ID, "spiffe://hr.local/w", claimed.LeaseToken,
			"failed", store.Artifact{}, nil, ledger.Entry{Kind: "action", Actor: "spiffe://hr.local/w", Outcome: "failed", Detail: "boom"}); err != nil || !ok {
			t.Fatalf("fail %d: ok=%v err=%v", i, ok, err)
		}
		if _, _, err := pgstore.InsertJob(ctx, pool, stepKey, "QA", "verify", "in", "R1", "revert", "likely", ""); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Watchdog(ctx, pool, st, reg, "spiffe://hr.local/watchdog", 3, 10*time.Minute); err != nil {
		t.Fatalf("watchdog: %v", err)
	}

	jobs, err := pgstore.ListJobs(ctx, pool, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if statusOf(jobs, stepKey) != "quarantined" {
		t.Fatalf("step is %q after repeated failures, want quarantined: %+v", statusOf(jobs, stepKey), jobs)
	}

	open, err := pgstore.OpenExceptions(ctx, pool, "spiffe://hr.local/watchdog")
	if err != nil {
		t.Fatal(err)
	}
	var exc exceptions.Exception
	for _, e := range open {
		if e.StepKey == stepKey {
			exc = e
		}
	}
	if exc.Digest == "" {
		t.Fatalf("the quarantine filed no exception carrying the step key: %+v", open)
	}

	if _, err := pgstore.ResolveException(ctx, pool, exc, "spiffe://hr.local/human", exc.Options[0]); err != nil {
		t.Fatal(err)
	}

	jobs, err = pgstore.ListJobs(ctx, pool, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(jobs, stepKey); got != "pending" {
		t.Errorf("step is %q after its exception was resolved, want pending — a human fixing the cause must get a resumed step, not a stuck one", got)
	}
}

func statusOf(jobs []pgstore.Job, stepKey string) string {
	for _, j := range jobs {
		if j.StepKey == stepKey {
			return j.Status
		}
	}
	return ""
}
