package persistence

import (
	"context"

	"github.com/Simon0x/hr/internal/ledger"
	"github.com/Simon0x/hr/internal/store"
)

// File is the single-user provider: a JSONL ledger and a directory of
// artifacts under .hr/. It ignores ctx because the filesystem calls beneath
// it do; the parameter is the seam's, not this backend's.
type File struct{ Root string }

var _ Store = File{}

func (f File) Append(_ context.Context, e ledger.Entry) (ledger.Entry, error) {
	return ledger.Append(f.Root, e)
}

func (f File) Read(_ context.Context) ([]ledger.Entry, error) {
	return ledger.Read(f.Root)
}

func (f File) Load(_ context.Context) ([]store.Artifact, error) {
	return store.LoadArtifacts(f.Root)
}

func (f File) Insert(_ context.Context, a store.Artifact, canonical []byte, createdBy string) (bool, error) {
	return store.Insert(f.Root, a, canonical, createdBy)
}
