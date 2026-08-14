package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/hrserver"
	"github.com/Simon0x/hr/internal/persistence/pgprovider"
)

func watchdogActor() string {
	if a := os.Getenv("HR_ACTOR"); a != "" {
		return a
	}
	return "spiffe://hr.local/watchdog"
}

func cmdWatchdog(root string, args []string) int {
	dsn := os.Getenv("HR_SERVER_DB")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "HR_SERVER_DB is required (a Postgres connection string) - the watchdog talks to the database directly, not through hr-server")
		return 2
	}

	threshold := hrserver.DefaultQuarantineThreshold
	if v, ok := flagValue(args, "threshold"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			threshold = n
		}
	}
	window := 10 * time.Minute
	if v, ok := flagValue(args, "window"); ok {
		if d, err := time.ParseDuration(v); err == nil {
			window = d
		}
	}
	poll := 30 * time.Second
	if v, ok := flagValue(args, "poll"); ok {
		if d, err := time.ParseDuration(v); err == nil {
			poll = d
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer pool.Close()

	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	actor := watchdogActor()
	logger := log.New(os.Stdout, "", log.LstdFlags)
	runWatchdogLoop(ctx, pool, reg, actor, threshold, window, poll, logger)
	return 0
}

func runWatchdogLoop(ctx context.Context, pool *pgxpool.Pool, reg *contracts.Registry, actor string, threshold int, window, poll time.Duration, logger *log.Logger) {
	logger.Printf("hr watchdog — actor %s, quarantine threshold %d failures / %s, checking every %s", actor, threshold, window, poll)

	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := hrserver.Watchdog(ctx, pool, pgprovider.Postgres{Pool: pool}, reg, actor, threshold, window)
			if err != nil {
				logger.Printf("watchdog error: %v", err)
				continue
			}
			if n > 0 {
				logger.Printf("quarantined %d step(s) - repeated failure with no intervening success", n)
			}
		}
	}
}
