package hrserver

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/pgstore"
	"github.com/Simon0x/hr/internal/workflow"
)

const DefaultQuarantineThreshold = 3

var quarantineOptions = []string{
	"resolve the underlying block, then manually re-run this step",
	"revise the input so it re-enters the queue with a stronger case",
	"abandon this step",
}

func Watchdog(ctx context.Context, pool *pgxpool.Pool, st persistence.Store, reg *contracts.Registry, actor string, threshold int, window time.Duration) (int, error) {
	if threshold <= 0 {
		threshold = DefaultQuarantineThreshold
	}
	reason := fmt.Sprintf("failed %d or more times within %s with no intervening success", threshold, window)

	quarantined, err := pgstore.QuarantineRepeatedFailures(ctx, pool, threshold, window, reason)
	if err != nil {
		return 0, err
	}

	for _, job := range quarantined {
		step := workflow.Step{
			Department: job.Department, Because: job.Because, Input: job.Input,
			Risk: job.Risk, Reversibility: job.Reversibility, Confidence: job.Confidence,
		}
		action := fmt.Sprintf("%s: %s", job.Department, job.Because)
		if _, err := FileException(ctx, pool, st, reg, step, job.StepKey, job.Risk, action, actor,
			reason,
			"this is a system defect, not evidence against the claim - repeated automatic retries were spending against a wall that will not clear itself. It will not be retried automatically again.",
			"system-defect", quarantineOptions,
		); err != nil {
			return len(quarantined), err
		}
		_ = pgstore.NotifyJobsChanged(ctx, pool)
	}

	return len(quarantined), nil
}
