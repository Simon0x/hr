// Package storetest is the shared behaviour suite for persistence providers.
// It lives in its own package so both providers can run it without the
// definition importing a backend - which was a literal import cycle - and so
// a new provider proves the same behaviour rather than its own.
package storetest

import (
	"context"
	"testing"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/Simon0x/hr/internal/store"
)

func sampleArtifact() (store.Artifact, []byte) {
	canonical := []byte(`{"_type":"x","subject":[],"predicateType":"https://hr.dev/signal/v1","predicate":{}}`)
	return store.Artifact{
		Kind: "signal", PredicateType: "https://hr.dev/signal/v1",
		Subject: []statement.Subject{{Name: "s", Digest: map[string]string{"sha256": "deadbeef"}}},
	}, canonical
}

// Run exercises every behaviour the seam promises. A provider that passes it
// is interchangeable with the others; one tested only on its own terms is
// two implementations wearing one name.
func Run(t *testing.T, newStore func(t *testing.T) persistence.Store) {
	t.Helper()

	t.Run("append builds a chain and reads it back", func(t *testing.T) {
		st, ctx := newStore(t), context.Background()
		for i := 0; i < 3; i++ {
			if _, err := st.Append(ctx, ledger.Entry{
				Kind: "action", Actor: "spiffe://hr.local/test", Outcome: "ok",
			}); err != nil {
				t.Fatalf("append %d: %v", i, err)
			}
		}
		entries, err := st.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 3 {
			t.Fatalf("read %d entries, want 3", len(entries))
		}
		if msg, ok := ledger.VerifyChain(entries); !ok {
			t.Fatalf("chain invalid: %s", msg)
		}
	})

	t.Run("append assigns position itself", func(t *testing.T) {
		st, ctx := newStore(t), context.Background()
		written, err := st.Append(ctx, ledger.Entry{
			Seq: 99, Prev: "deadbeef", Kind: "action", Actor: "a", Outcome: "ok",
		})
		if err != nil {
			t.Fatal(err)
		}
		if written.Seq != 0 || written.Prev != ledger.Genesis {
			t.Errorf("caller-supplied seq/prev survived: seq=%d prev=%s", written.Seq, written.Prev)
		}
	})

	t.Run("insert is idempotent on digest", func(t *testing.T) {
		st, ctx := newStore(t), context.Background()
		a, canonical := sampleArtifact()
		wantID := store.DigestID(canonical)

		first, err := st.Insert(ctx, a, canonical, "spiffe://hr.local/test")
		if err != nil {
			t.Fatal(err)
		}
		second, err := st.Insert(ctx, a, canonical, "spiffe://hr.local/test")
		if err != nil {
			t.Fatalf("re-emitting the same evidence must not error: %v", err)
		}
		if !first || second {
			t.Errorf("inserted flags = %v then %v, want true then false", first, second)
		}

		loaded, err := st.Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(loaded) != 1 {
			t.Fatalf("loaded %d artifacts after inserting one digest twice, want 1", len(loaded))
		}
		// The id comes from the bytes, not from the struct - otherwise one
		// artifact has two identities depending on where it landed.
		if loaded[0].ID != wantID {
			t.Errorf("round-tripped id = %q, want the content digest %q", loaded[0].ID, wantID)
		}
	})
}
