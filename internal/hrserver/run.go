package hrserver

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/persistence/pgprovider"
	"github.com/Simon0x/hr/internal/pgstore"
)

func Serve(ctx context.Context, pool *pgxpool.Pool, reg *contracts.Registry, root, addr, actor string, logger *log.Logger) (*Server, net.Addr, error) {
	exceptionsB := NewBroadcaster()
	jobsB := NewBroadcaster()
	st := pgprovider.Postgres{Pool: pool}
	srv := &Server{Pool: pool, Store: st, Registry: reg, Exceptions: exceptionsB, Jobs: jobsB, Root: root, StartedAt: time.Now()}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}

	srv.httpSrv = &http.Server{Handler: srv.Handler()}
	go func() {
		if err := srv.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Printf("http server error: %v", err)
		}
	}()
	go func() {
		if err := Listen(ctx, pool, pgstore.ExceptionsChannel, exceptionsB); err != nil && ctx.Err() == nil {
			logger.Printf("listen error: %v", err)
		}
	}()
	go func() {
		if err := Listen(ctx, pool, pgstore.JobsChannel, jobsB); err != nil && ctx.Err() == nil {
			logger.Printf("listen error: %v", err)
		}
	}()

	go runRepopulateAndReap(ctx, root, pool, st, reg, actor, logger)

	return srv, ln.Addr(), nil
}

func runRepopulateAndReap(ctx context.Context, root string, pool *pgxpool.Pool, st persistence.Store, reg *contracts.Registry, actor string, logger *log.Logger) {
	repopulateTicker := time.NewTicker(5 * time.Second)
	defer repopulateTicker.Stop()
	reapTicker := time.NewTicker(30 * time.Second)
	defer reapTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-repopulateTicker.C:
			n, err := Repopulate(ctx, root, pool, st, reg, actor)
			if err != nil {
				logger.Printf("repopulate error: %v", err)
				continue
			}
			if n > 0 {
				logger.Printf("repopulate: %d new job(s)/exception(s)", n)
			}
		case <-reapTicker.C:
			reset, escalated, err := pgstore.ReapExpiredLeases(ctx, pool, 3)
			if err != nil {
				logger.Printf("reap error: %v", err)
				continue
			}
			if reset > 0 || escalated > 0 {
				logger.Printf("reap: %d reset to pending, %d escalated", reset, escalated)
				_ = pgstore.NotifyJobsChanged(ctx, pool)
			}
		}
	}
}
