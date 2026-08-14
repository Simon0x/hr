# Contracts

The handoff schemas in [`../departments/README.md`](../departments/README.md),
made executable. Prose describes a contract; a schema is one.

## The shape is in-toto's, the content is ours

Every artifact is an
[in-toto Statement v1](https://github.com/in-toto/attestation/blob/main/spec/v1/statement.md):

```json
{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [{ "name": "CHANGE-a3f", "digest": { "gitcommit": "7fd1a60b..." } }],
  "predicateType": "https://hr.dev/verdict/v1",
  "predicate": { }
}
```

Four fields. Everything hr actually cares about lives in `predicate`, which the
standard never constrains - so adopting the envelope costs a naming convention
and buys a migration path to signed attestations that is a wrapper rather than a
rewrite.

Two rules come with it, and both were already ours:

- **A subject is matched by digest, never by name and never by a mutable ref.**
  `departments/README.md` requires this independently, for its own reasons: a
  handoff whose referent can change after it is written is not a record of
  anything.
- **An unknown `predicateType` fails closed.** A consumer that half-understands
  an artifact is worse than one that stops - it ships on a verdict it did not
  read.

## Why the shape is enforced rather than merely intended

`hr validate contracts` makes two claims. The first is that each artifact
matches its predicate schema. The second is that each artifact *would be
accepted by a real in-toto verifier* if the type URIs were swapped for the
standard ones.

The second claim is the load-bearing one, because "in the shape of in-toto"
degrades into "incompatible with in-toto" through changes that each look
harmless. `contracts/counterexamples/` holds five of them, and
`hr test contracts` asserts every one is caught:

| Drift | Why it is fatal |
|---|---|
| a field hoisted to the top level | a Statement has exactly four fields and no extension point there |
| subject named by branch | in-toto matches on digest alone, so it can never be verified |
| unknown predicate type | must refuse rather than guess |
| `acceptance` flattened to prose | loses the per-criterion state QA depends on |
| an invented acceptance state | `unverifiable` is the honest third state and gets quietly dropped |

Every one of those is a natural thing to write, which is the point. A migration
path nobody tests is a migration path nobody has.

## Versioning

The major lives in the `predicateType` URI. A minor change may add optional
fields; a breaking change increments the major, and a consumer meeting a major
it does not know refuses.

## The validator is a real JSON Schema implementation, on purpose

`hr` compiles to a single static binary, so unlike the schema validator this
project started with - hand-rolled specifically to avoid a dependency, because
the plugin is cloned into arbitrary repos and must run without an install step
- a dependency compiled into that binary costs nothing at runtime. That changes
the trade: hand-rolling a second, partial JSON Schema implementation is a real
correctness risk for no longer any purpose, so `hr` uses a spec-compliant
2020-12 validator instead. These schemas are ordinary draft 2020-12 documents;
any full JSON Schema implementation can point at them unchanged.

## Signing

Artifacts under `.hr/artifacts/` are [DSSE](https://github.com/secure-systems-lab/dsse)
envelopes: the Statement, base64'd, with an ed25519 signature over
`PAE(payloadType, payload)` rather than over the payload alone - so a signature
cannot be replayed against a document of a different type.

**The platform signs, never the agent.** This is the same move SLSA makes in
having the build platform attest rather than the build. The signature does not
claim the artifact is correct; it claims *the platform saw this artifact and its
gates were green*, which is a claim only the platform can make. An agent that
could sign its own verdict would be attesting to its own work.

Mechanically: `hr sign` reads the key from `HR_SIGNING_KEY` in the
environment and nowhere else, and the environment is CI. There is no key on a
workstation to leak, and no file for an agent holding `Bash` to read.

`hr validate attestation` **fails closed**. Unsigned, unknown keyid,
tampered payload, and a payload that is not a Statement are one answer: no. A
warning inside a green build is a warning nobody reads.
`hr test attestation` proves each of those rejections against an ephemeral
keypair that is discarded when the process exits - no test key is committed,
because a committed private key is indistinguishable from a real one to anything
that finds it later.

### Activating it

1. `hr keygen` — mints a pair and prints both halves.
2. Paste the public entry into `trust/keys.json`.
3. Set `HR_SIGNING_KEY` and `HR_SIGNING_KEYID` as repository secrets.

Until then the trust bundle is empty and verification passes only because there
is nothing to verify. The moment an artifact lands under `.hr/artifacts/`, it
refuses.

### What signing does not solve

Key rotation and revocation. `trust/keys.json` is a flat list with no validity
period, so a leaked key attests indefinitely and nothing distinguishes a
signature made before the leak from one made after. Real SVIDs from a SPIFFE
issuer fix this by being short-lived by construction. Until then it is a
**declared** blind spot - `SEC-001` in the register, `none` on the enforcement
ladder - which is honest, where an undeclared one would be a surprise with a
date on it.
