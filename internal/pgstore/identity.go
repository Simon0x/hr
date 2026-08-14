package pgstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Identity struct {
	SpiffeID    string
	DisplayName string
	Departments []string
}

// UnscopedDepartment is the explicit grant to every department. It is a
// character no capability name can contain, so it cannot collide with one.
const UnscopedDepartment = "*"

// MayAct reports whether this identity is granted the named department. An
// empty grant is no departments, not all of them - an identity provisioned
// without a stated scope can read but never act, so forgetting the flag
// under-grants instead of silently handing over the whole system.
func (i Identity) MayAct(department string) bool {
	for _, d := range i.Departments {
		if d == UnscopedDepartment || strings.EqualFold(d, department) {
			return true
		}
	}
	return false
}

// GrantedFrom narrows requested departments to those this identity may act
// on, preserving the caller's order.
func (i Identity) GrantedFrom(requested []string) []string {
	out := make([]string, 0, len(requested))
	for _, d := range requested {
		if i.MayAct(d) {
			out = append(out, d)
		}
	}
	return out
}

// NewToken generates a fresh opaque bearer token. Only its SHA-256 hash is
// ever persisted (see CreateIdentity) - the raw value is returned once, at
// creation, and cannot be recovered from the database afterward.
func NewToken() (raw string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "hr_" + hex.EncodeToString(buf), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// CreateIdentity inserts a new identity and returns its raw bearer token -
// the only time it is ever available in plaintext.
func CreateIdentity(ctx context.Context, db querier, spiffeID, displayName string, departments []string) (token string, err error) {
	if len(departments) == 0 {
		return "", errors.New("an identity needs a department grant: name them, or pass the unscoped grant \"*\"")
	}
	token, err = NewToken()
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO identities (spiffe_id, display_name, token_hash, departments)
		VALUES ($1, $2, $3, $4)`,
		spiffeID, displayName, HashToken(token), departments,
	); err != nil {
		return "", err
	}
	return token, nil
}

// IdentityByToken resolves a raw bearer token to the identity that owns it.
// ok is false if the token is empty or matches no identity.
func IdentityByToken(ctx context.Context, db querier, raw string) (Identity, bool, error) {
	if raw == "" {
		return Identity{}, false, nil
	}
	var id Identity
	err := db.QueryRow(ctx,
		`SELECT spiffe_id, display_name, departments FROM identities WHERE token_hash = $1`,
		HashToken(raw),
	).Scan(&id.SpiffeID, &id.DisplayName, &id.Departments)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, false, nil
	}
	if err != nil {
		return Identity{}, false, fmt.Errorf("looking up identity: %w", err)
	}
	return id, true, nil
}

// HasAnyIdentity reports whether at least one identity has been created -
// used to give a clear first-run message instead of an opaque 401.
func HasAnyIdentity(ctx context.Context, db querier) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM identities)`).Scan(&exists)
	return exists, err
}

// KnownIdentitySet loads every provisioned identity's spiffe_id. Used to
// tell a human-owned exception (its created_by matches a real identity, so
// it's private to them) apart from one filed by a system actor like
// spiffe://hr.local/hr-server (unknown here, so it stays visible to everyone).
func KnownIdentitySet(ctx context.Context, db querier) (map[string]bool, error) {
	rows, err := db.Query(ctx, `SELECT spiffe_id FROM identities`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = true
	}
	return set, rows.Err()
}
