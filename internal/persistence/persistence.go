// Package persistence is the Service Definition for durable hr state: the
// append-only ledger and the artifact store. It owns the vocabulary; the
// providers under it own the substrate.
//
// Both halves already had two implementations - files under .hr/ for local
// `hr run`, Postgres for the server - reachable only by importing one of
// them directly at each call site. That is the same leak the Harness
// interface was created to close: a consumer that names a concrete
// implementation cannot be pointed at another one.
package persistence

import (
	"context"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/store"
)

// Ledger is the append-only, hash-chained event log. Append assigns seq and
// prev itself - a caller cannot choose its own position in the chain.
type Ledger interface {
	Append(ctx context.Context, e ledger.Entry) (ledger.Entry, error)
	Read(ctx context.Context) ([]ledger.Entry, error)
}

// Artifacts is the content-addressed store of in-toto Statements. An
// artifact's id is derived from its canonical bytes, never taken from the
// struct, so the same evidence cannot be filed under two names. Insert
// reports whether the artifact was new; re-inserting the same digest is a
// no-op, never an error, because emitting the same evidence twice is not a
// failure.
type Artifacts interface {
	Load(ctx context.Context) ([]store.Artifact, error)
	Insert(ctx context.Context, a store.Artifact, canonical []byte, createdBy string) (bool, error)
}

// Store is both halves together, which is how every consumer wants them:
// an artifact and the entry recording that it landed are one fact.
type Store interface {
	Ledger
	Artifacts
}
