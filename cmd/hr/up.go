package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/contracts"
	"github.com/Simon0x/hr/internal/dispatch"
	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/hrclient"
	"github.com/Simon0x/hr/internal/hrserver"
	"github.com/Simon0x/hr/internal/pgstore"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func dockerAvailable() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found on PATH - install it first: https://docs.docker.com/get-docker/")
	}
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		return fmt.Errorf("docker compose not available - install Docker Desktop or the compose plugin")
	}
	return nil
}

func stackAddr() string {
	if a := os.Getenv("HR_SERVER_ADDR"); a != "" {
		return strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(a, "http://"), "https://"), "/")
	}
	return "localhost:7777"
}

func localPostgresDSN() string {
	if d := os.Getenv("HR_SERVER_DB"); d != "" {
		return d
	}
	return "postgres://hr:hr@localhost:7778/hr"
}

func stackHealthy(addr string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func startUnified(root, addr string) int {
	if err := ensurePostgres(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Println("shutting down...")
		cancel()
	}()

	pool, err := pgxpool.New(ctx, localPostgresDSN())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer pool.Close()

	if !waitPostgresReady(ctx, pool) {
		fmt.Fprintln(os.Stderr, "postgres did not become reachable within 60s")
		return 1
	}
	if err := pgstore.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	seats, err := dispatch.LoadSeats(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	schema, err := os.ReadFile(filepath.Join(root, "contracts", "statement.schema.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	listenAddr := envOr("HR_SERVER_LISTEN", ":7777")
	serverActor := envOr("HR_ACTOR", "spiffe://hr.local/hr-server")
	srv, _, err := hrserver.Serve(ctx, pool, reg, root, listenAddr, serverActor, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	url := "http://" + addr + "/"
	if !waitHealthy(addr, 30*time.Second) {
		fmt.Fprintf(os.Stderr, "hr did not become healthy at %s within 30s\n", addr)
		return 1
	}
	fmt.Printf("hr running at %s (Ctrl+C to stop)\n", url)
	if err := openBrowser(url); err != nil {
		fmt.Fprintf(os.Stderr, "could not open browser automatically: %v\n", err)
	}

	departments := make([]string, 0, len(seats))
	for dept := range seats {
		departments = append(departments, dept)
	}
	h, err := harness.Select(os.Getenv("HR_HARNESS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	warnIfNoToken()
	client := hrclient.New(addr, os.Getenv("HR_TOKEN"))
	go runWorkerLoop(ctx, root, client, reg, schema, workerActor(), departments, addr, 5*time.Second, seats, h)
	go runWatchdogLoop(ctx, pool, reg, watchdogActor(), hrserver.DefaultQuarantineThreshold, 10*time.Minute, 30*time.Second, logger)

	<-ctx.Done()
	logger.Println("draining in-flight requests...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
	return 0
}

func ensurePostgres(root string) error {
	if os.Getenv("HR_SERVER_DB") != "" {
		return nil
	}
	if err := dockerAvailable(); err != nil {
		return err
	}
	cmd := exec.Command("docker", "compose", "up", "-d", "postgres")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitPostgresReady(ctx context.Context, pool *pgxpool.Pool) bool {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if err := pool.Ping(ctx); err == nil {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(500 * time.Millisecond):
		}
	}
	return false
}

func warnIfNoToken() {
	if os.Getenv("HR_TOKEN") == "" {
		fmt.Fprintln(os.Stderr, "warning: HR_TOKEN not set - the built-in worker cannot claim jobs "+
			"until you run `hr identity create --name <you>`, set HR_TOKEN to the printed token, and restart")
	}
}

func waitHealthy(addr string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if stackHealthy(addr) {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

func attachWorkerOnly(root, addr string) int {
	url := "http://" + addr + "/"
	fmt.Printf("hr running at %s (Ctrl+C to stop)\n", url)
	if err := openBrowser(url); err != nil {
		fmt.Fprintf(os.Stderr, "could not open browser automatically: %v\n", err)
	}

	reg, err := contracts.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	seats, err := dispatch.LoadSeats(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	schema, err := os.ReadFile(filepath.Join(root, "contracts", "statement.schema.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	departments := make([]string, 0, len(seats))
	for dept := range seats {
		departments = append(departments, dept)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	h, err := harness.Select(os.Getenv("HR_HARNESS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	warnIfNoToken()
	client := hrclient.New(addr, os.Getenv("HR_TOKEN"))
	go runWorkerLoop(ctx, root, client, reg, schema, workerActor(), departments, addr, 5*time.Second, seats, h)

	if dsn := os.Getenv("HR_SERVER_DB"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			defer pool.Close()
			logger := log.New(os.Stdout, "", log.LstdFlags)
			go runWatchdogLoop(ctx, pool, reg, watchdogActor(), hrserver.DefaultQuarantineThreshold, 10*time.Minute, 30*time.Second, logger)
		}
	}

	<-ctx.Done()
	return 0
}
