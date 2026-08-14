package pgstore

import (
	"context"
	"testing"
)

func TestListJobs_SharedJobsVisibleToEveryone_OwnedJobsPrivate(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if _, inserted, err := InsertJob(ctx, pool, "step-shared", "QA", "system work", "in", "R0", "revert", "likely", ""); err != nil || !inserted {
		t.Fatalf("insert shared job: inserted=%v err=%v", inserted, err)
	}
	if _, inserted, err := InsertJob(ctx, pool, "step-alice", "QA", "alice's work", "in", "R0", "revert", "likely", "spiffe://hr.local/alice"); err != nil || !inserted {
		t.Fatalf("insert alice's job: inserted=%v err=%v", inserted, err)
	}

	aliceJobs, err := ListJobs(ctx, pool, 50, "spiffe://hr.local/alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(aliceJobs) != 2 {
		t.Fatalf("alice sees %d jobs, want 2 (the shared one and her own)", len(aliceJobs))
	}

	bobJobs, err := ListJobs(ctx, pool, 50, "spiffe://hr.local/bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(bobJobs) != 1 || bobJobs[0].StepKey != "step-shared" {
		t.Fatalf("bob sees %+v, want only step-shared - alice's job must stay private", bobJobs)
	}
}
