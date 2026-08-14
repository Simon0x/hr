package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Simon0x/hr/internal/dispatch"
	"github.com/Simon0x/hr/internal/harness"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/workflow"
	"github.com/fsnotify/fsnotify"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketReady   = []byte("ready")
	bucketBlocked = []byte("blocked")
)

type Config struct {
	Root      string
	Execute   bool
	Actor     string
	Workers   int
	PollEvery time.Duration
	Harness   harness.Harness
}

type Daemon struct {
	cfg        Config
	db         *bolt.DB
	queue      chan workflow.Step
	dispatched map[string]bool
	mu         sync.Mutex
	log        *log.Logger
}

func Open(cfg Config, logger *log.Logger) (*Daemon, error) {
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 30 * time.Second
	}
	if cfg.Harness == nil {
		cfg.Harness = harness.Claude{}
	}
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}

	if err := os.MkdirAll(filepath.Join(cfg.Root, ".hr"), 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cfg.Root, ".hr", "index.bbolt")
	db, err := bolt.Open(dbPath, 0o644, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketReady, bucketBlocked} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Daemon{
		cfg: cfg, db: db, queue: make(chan workflow.Step, 256),
		dispatched: map[string]bool{}, log: logger,
	}, nil
}

func (d *Daemon) Close() error { return d.db.Close() }

type Interrupted struct {
	StepKey   string
	ClaimedAt string
}

var stepKeyPattern = regexp.MustCompile(`^[a-f0-9]{12}$`)

// FindInterrupted replays the ledger for `claimed` action entries with no
// later terminal (ok/failed) entry carrying the same step key — a dispatch
// the daemon started and never finished, most likely because it crashed or
// was killed mid-invocation.
func (d *Daemon) FindInterrupted(ctx context.Context) ([]Interrupted, error) {
	entries, err := persistence.File{Root: d.cfg.Root}.Read(ctx)
	if err != nil {
		return nil, err
	}

	claimedAt := map[string]string{}
	completed := map[string]bool{}
	for _, e := range entries {
		if e.Kind != "action" {
			continue
		}
		fields := strings.Fields(e.Detail)
		if len(fields) == 0 || !stepKeyPattern.MatchString(fields[0]) {
			continue
		}
		key := fields[0]
		switch e.Outcome {
		case "claimed":
			claimedAt[key] = e.At
		case "ok", "failed", "blocked":
			completed[key] = true
		}
	}

	var out []Interrupted
	for key, at := range claimedAt {
		if !completed[key] {
			out = append(out, Interrupted{StepKey: key, ClaimedAt: at})
		}
	}
	return out, nil
}

// Refresh re-derives the workflow plan, rebuilds the bbolt read-model (a
// disposable cache, never the source of truth), and enqueues any ready step
// not already handled this session.
func (d *Daemon) Refresh(ctx context.Context) (*workflow.Plan, error) {
	plan, err := workflow.Derive(ctx, persistence.File{Root: d.cfg.Root}, d.cfg.Root)
	if err != nil {
		return nil, err
	}

	err = d.db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketReady, bucketBlocked} {
			if err := tx.DeleteBucket(name); err != nil && err != bolt.ErrBucketNotFound {
				return err
			}
			if _, err := tx.CreateBucket(name); err != nil {
				return err
			}
		}
		rb := tx.Bucket(bucketReady)
		for i, s := range plan.Ready {
			v, _ := json.Marshal(s)
			if err := rb.Put([]byte(fmt.Sprintf("%03d-%s", i, dispatch.StepKey(s))), v); err != nil {
				return err
			}
		}
		bb := tx.Bucket(bucketBlocked)
		for i, s := range plan.Blocked {
			v, _ := json.Marshal(s)
			if err := bb.Put([]byte(fmt.Sprintf("%03d", i)), v); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	for _, s := range plan.Ready {
		key := dispatch.StepKey(s)
		if d.dispatched[key] {
			continue
		}
		select {
		case d.queue <- s:
			d.dispatched[key] = true
		default:
			d.log.Printf("queue full — %s will be picked up on a later refresh", key)
		}
	}
	d.mu.Unlock()

	return plan, nil
}

func (d *Daemon) DispatchOne(ctx context.Context, step workflow.Step) {
	result, err := dispatch.One(ctx, persistence.File{Root: d.cfg.Root}, d.cfg.Root, step, d.cfg.Actor, d.cfg.Execute, d.cfg.Harness)
	if err != nil {
		d.log.Printf("%s — %s: error: %v", step.Department, step.Because, err)
		return
	}

	switch result.Verdict {
	case dispatch.VerdictNoSeat:
		d.log.Printf("%s: no procedure for this department", step.Department)
	case dispatch.VerdictBudgetRefused:
		d.log.Printf("%s — %s: budget refused", step.Department, step.Because)
	case dispatch.VerdictRefused:
		d.log.Printf("%s — %s: refused", step.Department, step.Because)
	case dispatch.VerdictEscalated:
		if result.ExceptionPath != "" {
			d.log.Printf("%s — %s: ESCALATED, filed %s", step.Department, step.Because, result.ExceptionPath)
		} else {
			d.log.Printf("%s — %s: ESCALATED, failed to file exception: %v", step.Department, step.Because, result.ExceptionErr)
		}
	case dispatch.VerdictDryRun:
		d.log.Printf("%s — %s: would invoke %s (dry run — pass --execute to act)", step.Department, step.Because, result.Seat.Procedure)
	case dispatch.VerdictInvoked:
		d.log.Printf("%s — %s: invoked %s, done", step.Department, step.Because, result.Seat.Procedure)
	case dispatch.VerdictFailed:
		d.log.Printf("%s — %s: FAILED invoking %s (exit %d)", step.Department, step.Because, result.Seat.Procedure, result.AgentExit)
	case dispatch.VerdictBlocked:
		if result.ExceptionPath != "" {
			d.log.Printf("%s — %s: BLOCKED on tool permission, filed %s", step.Department, step.Because, result.ExceptionPath)
		} else {
			d.log.Printf("%s — %s: BLOCKED on tool permission, failed to file exception: %v", step.Department, step.Because, result.ExceptionErr)
		}
	}
}

func (d *Daemon) worker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case step, ok := <-d.queue:
			if !ok {
				return
			}
			d.DispatchOne(ctx, step)
		}
	}
}

// Run is the persistent loop: replay for interrupted dispatches, then watch
// .hr/artifacts/ and re-derive on change or on a fallback poll interval,
// feeding a bounded worker pool. Blocks until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	interrupted, err := d.FindInterrupted(ctx)
	if err != nil {
		return err
	}
	for _, in := range interrupted {
		d.log.Printf("INTERRUPTED DISPATCH: step %s claimed at %s, never completed — needs review, not retried automatically", in.StepKey, in.ClaimedAt)
	}

	if _, err := d.Refresh(ctx); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for i := 0; i < d.cfg.Workers; i++ {
		wg.Add(1)
		go d.worker(ctx, &wg)
	}

	watchDir := filepath.Join(d.cfg.Root, ".hr", "artifacts")
	if err := os.MkdirAll(watchDir, 0o755); err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := watcher.Add(watchDir); err != nil {
		return err
	}

	ticker := time.NewTicker(d.cfg.PollEvery)
	defer ticker.Stop()

	d.log.Printf("watching %s (poll every %s, %d worker(s), execute=%v)", watchDir, d.cfg.PollEvery, d.cfg.Workers, d.cfg.Execute)

	for {
		select {
		case <-ctx.Done():
			close(d.queue)
			wg.Wait()
			return nil
		case ev, ok := <-watcher.Events:
			if !ok {
				continue
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				d.log.Printf("detected change: %s", filepath.Base(ev.Name))
				if _, err := d.Refresh(ctx); err != nil {
					d.log.Printf("refresh error: %v", err)
				}
			}
		case werr, ok := <-watcher.Errors:
			if !ok {
				continue
			}
			d.log.Printf("watcher error: %v", werr)
		case <-ticker.C:
			if _, err := d.Refresh(ctx); err != nil {
				d.log.Printf("refresh error: %v", err)
			}
		}
	}
}

// RunOnce does interrupted-detection, one refresh, and drains whatever was
// enqueued synchronously — a cron-friendly alternative to a persistent
// process, and what `hr daemon --once` uses.
func (d *Daemon) RunOnce(ctx context.Context) error {
	interrupted, err := d.FindInterrupted(ctx)
	if err != nil {
		return err
	}
	for _, in := range interrupted {
		d.log.Printf("INTERRUPTED DISPATCH: step %s claimed at %s, never completed — needs review, not retried automatically", in.StepKey, in.ClaimedAt)
	}

	if _, err := d.Refresh(ctx); err != nil {
		return err
	}

	for {
		select {
		case step := <-d.queue:
			d.DispatchOne(ctx, step)
		default:
			return nil
		}
	}
}
