package hrserver

import (
	"context"
	"testing"
	"time"

	"github.com/Simon0x/hr/internal/pgstore"
)

func TestBroadcaster_PublishReachesSubscribers(t *testing.T) {
	b := NewBroadcaster()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	defer b.Unsubscribe(ch1)
	defer b.Unsubscribe(ch2)

	b.Publish()

	for _, ch := range []chan struct{}{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("subscriber did not receive publish")
		}
	}
}

func TestBroadcaster_UnsubscribeStopsDelivery(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	b.Publish()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("unsubscribed channel should not receive a publish")
		}
	default:
	}
}

func TestListenExceptions_ReceivesNotify(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broadcaster := NewBroadcaster()
	ch := broadcaster.Subscribe()
	defer broadcaster.Unsubscribe(ch)

	errCh := make(chan error, 1)
	go func() { errCh <- Listen(ctx, pool, pgstore.ExceptionsChannel, broadcaster) }()

	time.Sleep(200 * time.Millisecond)

	if err := pgstore.NotifyExceptionsChanged(context.Background(), pool); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ch:
	case err := <-errCh:
		t.Fatalf("listener failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for notification to reach the broadcaster")
	}
}
