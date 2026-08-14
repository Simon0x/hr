package pgstore

import "context"

const (
	ExceptionsChannel = "hr_exceptions_changed"
	JobsChannel       = "hr_jobs_changed"
)

func notify(ctx context.Context, db querier, channel string) error {
	_, err := db.Exec(ctx, "SELECT pg_notify($1, '')", channel)
	return err
}

func NotifyExceptionsChanged(ctx context.Context, db querier) error {
	return notify(ctx, db, ExceptionsChannel)
}

func NotifyJobsChanged(ctx context.Context, db querier) error {
	return notify(ctx, db, JobsChannel)
}
