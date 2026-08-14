// Package pgprovider is the multi-user provider for the persistence seam.
// It lives beside the definition rather than inside it: a Service Definition
// that imports a provider drags every backend's dependencies into everything
// that consumes the interface, and here it was a literal import cycle.
package pgprovider

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/persistence"
	"github.com/Simon0x/hr/internal/pgstore"
	"github.com/Simon0x/hr/internal/store"
)

// Postgres is the multi-user provider. Its Append serializes on an advisory
// lock so concurrent writers cannot fork the chain, which is the reason this
// backend exists at all.
type Postgres struct{ Pool *pgxpool.Pool }

var _ persistence.Store = Postgres{}

func (p Postgres) Append(ctx context.Context, e ledger.Entry) (ledger.Entry, error) {
	return pgstore.Append(ctx, p.Pool, e)
}

func (p Postgres) Read(ctx context.Context) ([]ledger.Entry, error) {
	return pgstore.Read(ctx, p.Pool)
}

func (p Postgres) Load(ctx context.Context) ([]store.Artifact, error) {
	return pgstore.LoadArtifacts(ctx, p.Pool)
}

func (p Postgres) Insert(ctx context.Context, a store.Artifact, canonical []byte, createdBy string) (bool, error) {
	// The file backend derives the id from content on read; deriving it here
	// too keeps one artifact from having two identities across backends.
	a.ID = store.DigestID(canonical)
	return pgstore.InsertArtifact(ctx, p.Pool, a, canonical, createdBy)
}
