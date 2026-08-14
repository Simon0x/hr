package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/hrserver"
	"github.com/Simon0x/hr/internal/pgstore"
)

const shutdownDrain = 10 * time.Second

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := envOr("HR_SERVER_LISTEN", ":7777")
	dsn := os.Getenv("HR_SERVER_DB")
	root := os.Getenv("HR_SERVER_ROOT")
	actor := envOr("HR_ACTOR", "spiffe://hr.local/hr-server")

	if dsn == "" {
		fmt.Fprintln(os.Stderr, "HR_SERVER_DB is required (a Postgres connection string)")
		os.Exit(2)
	}
	if root == "" {
		fmt.Fprintln(os.Stderr, "HR_SERVER_ROOT is required (a local checkout with contracts/, policies/, departments/)")
		os.Exit(2)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		logger.Println("shutting down...")
		cancel()
	}()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer pool.Close()

	if err := pgstore.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	srv, boundAddr, err := hrserver.Serve(ctx, pool, reg, root, addr, actor, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	logger.Printf("hr-server listening on %s", boundAddr)

	<-ctx.Done()
	logger.Println("draining in-flight requests...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownDrain)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
}
