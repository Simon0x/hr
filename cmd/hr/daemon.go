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

	"github.com/Simon0x/hr/internal/daemon"
	"github.com/Simon0x/hr/internal/harness"
)

func cmdDaemon(root string, args []string) int {
	execute := hasFlag(args, "execute")
	once := hasFlag(args, "once")

	workers := 2
	if v, ok := flagValue(args, "workers"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			workers = n
		}
	}
	poll := 30 * time.Second
	if v, ok := flagValue(args, "poll"); ok {
		if dur, err := time.ParseDuration(v); err == nil {
			poll = dur
		}
	}

	h, err := harness.Select(os.Getenv("HR_HARNESS"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	d, err := daemon.Open(daemon.Config{
		Root: root, Execute: execute, Actor: hrActor(), Workers: workers, PollEvery: poll, Harness: h,
	}, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer d.Close()

	if !execute {
		logger.Println("watch-and-report only — pass --execute to actually dispatch")
	}

	if once {
		if err := d.RunOnce(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		logger.Println("shutting down...")
		cancel()
	}()

	if err := d.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
