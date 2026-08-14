package attest

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type trustEntry struct {
	KeyID     string `json:"keyid"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"publicKey"`
	Note      string `json:"note,omitempty"`
	// NotBefore, NotAfter and RevokedAt are RFC3339 instants bounding when
	// this key could legitimately sign. All three are optional: a key with
	// none is valid for all time, which is what every key was before this
	// existed and what a first key still is until someone bounds it.
	NotBefore string `json:"notBefore,omitempty"`
	NotAfter  string `json:"notAfter,omitempty"`
	RevokedAt string `json:"revokedAt,omitempty"`
}

// TrustKey is one public half and the window it was allowed to sign in.
type TrustKey struct {
	KeyID     string
	Public    ed25519.PublicKey
	NotBefore time.Time
	NotAfter  time.Time
	RevokedAt time.Time
}

// Bundle is the trusted set. Lookups are answered as of an instant rather
// than as of now, because the question an attested ledger asks is "was this
// key allowed to sign when it signed", not "is it allowed today". Revoking a
// key must invalidate what it signed afterwards without invalidating a year
// of legitimate artifacts behind it.
type Bundle struct {
	Keys map[string]TrustKey
}

// ErrNoKeys distinguishes an empty trust bundle from a wrong one.
var ErrNoKeys = errors.New("trust bundle is empty")

// At returns the key for keyid if it was valid at when.
func (b Bundle) At(keyid string, when time.Time) (ed25519.PublicKey, error) {
	k, ok := b.Keys[keyid]
	if !ok {
		return nil, fmt.Errorf("keyid %q is not in the trust bundle", keyid)
	}
	if !k.RevokedAt.IsZero() && !when.Before(k.RevokedAt) {
		return nil, fmt.Errorf("keyid %q was revoked at %s and this was signed at %s — "+
			"anything it signed before the revocation still verifies, this did not",
			keyid, k.RevokedAt.Format(time.RFC3339), when.Format(time.RFC3339))
	}
	if !k.NotBefore.IsZero() && when.Before(k.NotBefore) {
		return nil, fmt.Errorf("keyid %q was not valid until %s, and this was signed at %s",
			keyid, k.NotBefore.Format(time.RFC3339), when.Format(time.RFC3339))
	}
	if !k.NotAfter.IsZero() && when.After(k.NotAfter) {
		return nil, fmt.Errorf("keyid %q expired at %s and this was signed at %s",
			keyid, k.NotAfter.Format(time.RFC3339), when.Format(time.RFC3339))
	}
	return k.Public, nil
}

func LoadTrust(root string) (Bundle, error) {
	path := filepath.Join(root, "trust", "keys.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Bundle{Keys: map[string]TrustKey{}}, nil
	}
	if err != nil {
		return Bundle{}, err
	}

	var bundle struct {
		Keys []trustEntry `json:"keys"`
	}
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return Bundle{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	out := Bundle{Keys: make(map[string]TrustKey, len(bundle.Keys))}
	for _, e := range bundle.Keys {
		pub, err := parseSPKIPublicKey(e.PublicKey)
		if err != nil {
			return Bundle{}, fmt.Errorf("trust/keys.json: keyid %q: %w", e.KeyID, err)
		}
		k := TrustKey{KeyID: e.KeyID, Public: pub}
		for _, f := range []struct {
			name  string
			raw   string
			field *time.Time
		}{
			{"notBefore", e.NotBefore, &k.NotBefore},
			{"notAfter", e.NotAfter, &k.NotAfter},
			{"revokedAt", e.RevokedAt, &k.RevokedAt},
		} {
			if f.raw == "" {
				continue
			}
			t, perr := time.Parse(time.RFC3339, f.raw)
			if perr != nil {
				return Bundle{}, fmt.Errorf("trust/keys.json: keyid %q: %s is not RFC3339: %w", e.KeyID, f.name, perr)
			}
			*f.field = t
		}
		if !k.NotAfter.IsZero() && !k.NotBefore.IsZero() && k.NotAfter.Before(k.NotBefore) {
			return Bundle{}, fmt.Errorf("trust/keys.json: keyid %q expires before it becomes valid", e.KeyID)
		}
		out.Keys[e.KeyID] = k
	}
	return out, nil
}

func LoadSigningKey() (keyid string, priv ed25519.PrivateKey, err error) {
	pemKey := os.Getenv("HR_SIGNING_KEY")
	keyid = os.Getenv("HR_SIGNING_KEYID")
	if pemKey == "" || keyid == "" {
		return "", nil, errors.New(
			"HR_SIGNING_KEY and HR_SIGNING_KEYID must be set. Signing happens in CI, " +
				"not on a workstation - see contracts/README.md. Run `hr keygen` to mint " +
				"a pair for a new trust domain.")
	}

	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return "", nil, errors.New("HR_SIGNING_KEY is not valid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", nil, fmt.Errorf("HR_SIGNING_KEY: %w", err)
	}
	ed, ok := key.(ed25519.PrivateKey)
	if !ok {
		return "", nil, fmt.Errorf("HR_SIGNING_KEY is a %T, not an ed25519 private key", key)
	}
	return keyid, ed, nil
}

func parseSPKIPublicKey(pemStr string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("not valid PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("is a %T, not an ed25519 public key", key)
	}
	return pub, nil
}
