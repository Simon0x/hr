package hrserver

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Broadcaster struct {
	mu   sync.Mutex
	subs map[chan struct{}]bool
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subs: map[chan struct{}]bool{}}
}

func (b *Broadcaster) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[ch] = true
	b.mu.Unlock()
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *Broadcaster) Publish() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func Listen(ctx context.Context, pool *pgxpool.Pool, channel string, b *Broadcaster) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return err
	}

	for {
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		b.Publish()
	}
}
