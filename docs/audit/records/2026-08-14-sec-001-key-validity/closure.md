# SEC-001 closure: keys have a window, a revocation, and a rotation procedure

**Closed:** 2026-08-14

## What the finding claimed

`trust/keys.json` was a flat list of public keys with no validity period, no
revocation mechanism and no rotation procedure. A compromised key was removed
by editing a file and hoping every verifier had the new copy, and nothing
distinguished a signature made before a leak from one made after.

## Two approaches considered and rejected

**A key derived from a hardware id.** An HWID is an identifier, not a secret:
it is readable by any process on the host, often logged, frequently in an
inventory system. Deriving a private key from it is equivalent to publishing
the private key. It also defeats the finding it would be fixing - a key
deterministic from hardware cannot be rotated without replacing the hardware -
and it breaks the standing constraint that no agent ever holds the signing
key, since a key derivable on the host is derivable by whatever runs there.

**A key derived from an account credential.** Same flaw one layer up: the key
is only as secret as the credential, anyone holding it can mint signatures,
and it cannot rotate independently of the password. Per-identity keys are the
right shape, but issued, never derived - which is what SPIFFE provides and why
the original fix named it.

## What this pass did

The finding's three complaints are answerable without an issuer:

- Each `trust/keys.json` entry may carry `notBefore`, `notAfter` and
  `revokedAt` as RFC3339 instants. All optional; a key with none is valid for
  all time, so a bundle written before this keeps working.
- `LoadTrust` returns a `Bundle` whose lookups are answered **as of an
  instant** rather than as of now: `Bundle.At(keyid, when)`.
- The instant is **when the artifact was signed**, taken from the ledger's
  `emitted` entry for it. Not from the envelope: a signer states its own time,
  and a compromised key would state whatever time keeps it inside its window.
  The chain cannot be backdated without breaking every link after it, which is
  what makes it usable as a clock. An artifact with no `emitted` entry is
  refused rather than verified against now.
- Rotation and revocation procedures are written into `trust/keys.json`
  itself, where an operator meets them: rotate by adding the new key with
  `notBefore` and setting `notAfter` on the old one, keeping the old entry
  forever because deleting it strands everything it signed. Revoke by setting
  `revokedAt` to the earliest instant of suspected exposure, not to now.

## Evidence

`hr test attestation` grew from 7 cases to 12. The four that carry the
finding:

- **signed after revocation is refused**
- **signed before revocation still verifies** — the one that matters. Revoking
  a key must not retroactively invalidate a year of legitimate artifacts.
- expired key is refused; key used before it was valid is refused
- an unbounded key verifies at any instant, so existing bundles are unaffected

## What is left, and is not this finding

Issuance is still manual: `hr keygen`, paste the public half, set two CI
secrets. Short-lived SVIDs from a SPIFFE issuer would make rotation the
default rather than a procedure someone has to follow, and that remains the
better end state. What changed is that a leaked key can now be bounded and
revoked with correct history semantics, rather than deleted and hoped about.
