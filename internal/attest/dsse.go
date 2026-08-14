package attest

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/Simon0x/hr/internal/statement"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type ed25519Signer struct {
	keyid string
	priv  ed25519.PrivateKey
}

func (s ed25519Signer) Sign(_ context.Context, data []byte) ([]byte, error) {
	return ed25519.Sign(s.priv, data), nil
}

func (s ed25519Signer) KeyID() (string, error) { return s.keyid, nil }

func Sign(ctx context.Context, raw []byte, keyid string, priv ed25519.PrivateKey) (*dsse.Envelope, error) {
	signer, err := dsse.NewEnvelopeSigner(ed25519Signer{keyid: keyid, priv: priv})
	if err != nil {
		return nil, err
	}
	return signer.SignPayload(ctx, statement.AttestationPayloadType, raw)
}

type VerifyResult struct {
	Statement []byte
	KeyID     string
}

// Only checks Signatures[0]; unknown keyid is a hard failure, not skipped.
// Verify checks a signature against the trust bundle as it stood at
// signedAt. The instant comes from the ledger's `emitted` entry for the
// artifact, not from the envelope: a signer states its own time, and a
// compromised key would state whatever time keeps it inside its window. The
// chain cannot be backdated without breaking every link after it, which is
// what makes it usable as the clock here.
func Verify(env *dsse.Envelope, trust Bundle, signedAt time.Time) (*VerifyResult, error) {
	if env.PayloadType != statement.AttestationPayloadType {
		return nil, fmt.Errorf("unexpected payloadType %q", env.PayloadType)
	}
	if len(env.Signatures) == 0 {
		return nil, errors.New("no signatures - an unsigned envelope proves nothing")
	}

	payload, err := env.DecodeB64Payload()
	if err != nil {
		return nil, fmt.Errorf("payload is not valid base64: %w", err)
	}
	signed := dsse.PAE(statement.AttestationPayloadType, payload)

	sig := env.Signatures[0]
	key, err := trust.At(sig.KeyID, signedAt)
	if err != nil {
		return nil, err
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sig.Sig)
	if err != nil {
		return nil, fmt.Errorf("signature by %q is not valid base64: %w", sig.KeyID, err)
	}
	if !ed25519.Verify(key, signed, sigBytes) {
		return nil, fmt.Errorf("signature by %q does not verify", sig.KeyID)
	}

	return &VerifyResult{Statement: payload, KeyID: sig.KeyID}, nil
}
