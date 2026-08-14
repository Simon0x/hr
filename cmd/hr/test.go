package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Simon0x/hr/internal/attest"
	"github.com/Simon0x/hr/internal/policy"
	"github.com/Simon0x/hr/internal/statement"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

func cmdTest(root string, args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "contracts":
		return testContracts(root)
	case "attestation":
		return testAttestation(root)
	case "authority":
		return testAuthority(root)
	default:
		fmt.Fprintf(os.Stderr, "unknown test subcommand %q\n", args[0])
		return 2
	}
}

type counterexample struct {
	file   string
	expect string
}

var counterexamples = []counterexample{
	{"top-level-field.json", "there is no extension point at this level"},
	{"subject-by-branch.json", "matched by digest alone"},
	{"unknown-predicate.json", "refusing rather than guessing"},
	{"acceptance-flattened.json", "want array"},
	{"bad-acceptance-state.json", "must be one of 'met', 'unmet', 'unverifiable'"},
}

// @gate test:contracts
// @covers contracts/**
// @covers internal/contracts/**
// @covers cmd/hr/validate.go
func testContracts(root string) int {
	pass, fail := 0, 0
	dir := filepath.Join(root, "contracts", "counterexamples")

	for _, c := range counterexamples {
		path := filepath.Join(dir, c.file)
		if _, err := os.Stat(path); err != nil {
			fmt.Printf("MISSING  %s\n", c.file)
			fail++
			continue
		}

		result, err := runContractsValidation(root, []string{path})
		if err != nil {
			fmt.Printf("FAIL     %s: %v\n", c.file, err)
			fail++
			continue
		}
		if len(result.problems) == 0 {
			fmt.Printf("NOT REJECTED  %s\n", c.file)
			fail++
			continue
		}

		joined := strings.Join(result.problems, "\n")
		if !strings.Contains(joined, c.expect) {
			fmt.Printf("WRONG REASON  %s: got %q, want substring %q\n", c.file, joined, c.expect)
			fail++
			continue
		}
		fmt.Printf("ok       %s rejected: %s\n", c.file, c.expect)
		pass++
	}

	controlResult, err := runContractsValidation(root, discoverJSONTargets(root, nil,
		"contracts/examples", ".hr/artifacts", ".hr/memory"))
	if err != nil || len(controlResult.problems) > 0 {
		fmt.Printf("CONTROL FAILED  %v %v\n", err, controlResult.problems)
		fail++
	} else {
		fmt.Println("ok       control case (no args) validates cleanly")
		pass++
	}

	fmt.Printf("\n%d passed, %d failed\n", pass, fail)
	if fail > 0 {
		return 1
	}
	return 0
}

type authorityScenario struct {
	name      string
	facts     policy.Facts
	requested string
	want      string
}

var authorityScenarios = []authorityScenario{
	{"trivial to revert but it sends mail",
		policy.Facts{Action: "x", Actor: "spiffe://hr.local/x", Risk: "R0", Reversibility: "emit", Confidence: "likely", Observability: "logs"},
		"A2", "escalate"},
	{"expensive to undo, nobody outside is affected",
		policy.Facts{Action: "x", Actor: "spiffe://hr.local/x", Risk: "R1", Reversibility: "migrate", Confidence: "certain", Observability: "dashboards", RollbackRehearsed: true},
		"A2", "allow"},
	{"migrate whose down path was never run",
		policy.Facts{Action: "x", Actor: "spiffe://hr.local/x", Risk: "R1", Reversibility: "migrate", Confidence: "certain", Observability: "dashboards", RollbackRehearsed: false},
		"A2", "refuse"},
	{"silent failure mode caps autonomy",
		policy.Facts{Action: "x", Actor: "spiffe://hr.local/x", Risk: "R0", Reversibility: "revert", Confidence: "certain", Observability: "silent"},
		"A2", "escalate"},
	{"unknown confidence caps autonomy",
		policy.Facts{Action: "x", Actor: "spiffe://hr.local/x", Risk: "R0", Reversibility: "revert", Confidence: "unknown", Observability: "logs"},
		"A2", "escalate"},
	{"R4 never self-authorises",
		policy.Facts{Action: "x", Actor: "spiffe://hr.local/x", Risk: "R4", Reversibility: "revert", Confidence: "certain", Observability: "logs"},
		"A2", "escalate"},
	{"fully internal and reversible is allowed",
		policy.Facts{Action: "x", Actor: "spiffe://hr.local/x", Risk: "R0", Reversibility: "revert", Confidence: "certain", Observability: "logs"},
		"A2", "allow"},
	{"an action with no identity",
		policy.Facts{Action: "x", Actor: "", Risk: "R0", Reversibility: "revert", Confidence: "certain", Observability: "logs"},
		"A2", "refuse"},
}

// @gate test:authority
// @covers policies/**
// @covers internal/policy/**
// @covers cmd/hr/authority.go
func testAuthority(root string) int {
	p, raw, err := policy.LoadPolicy(filepath.Join(root, "policies", "default.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])[:16]

	pass, fail := 0, 0
	for _, s := range authorityScenarios {
		d := policy.Evaluate(p, digest, s.facts, s.requested)
		if d.Verdict == s.want {
			fmt.Printf("ok    %-47s%s\n", s.name, d.Verdict)
			pass++
		} else {
			fmt.Printf("FAIL  %-47s%s (want %s)\n", s.name, d.Verdict, s.want)
			fail++
		}
	}

	fmt.Printf("\n%d passed, %d failed\n", pass, fail)
	if fail > 0 {
		return 1
	}
	return 0
}

func check(pass, fail *int, name string, ok bool, detail string) {
	if ok {
		*pass++
		fmt.Printf("ok    %s\n", name)
	} else {
		*fail++
		fmt.Printf("FAIL  %s: %s\n", name, detail)
	}
}

// @gate test:attestation
// @covers internal/attest/**
// @covers cmd/hr/sign.go
// @covers cmd/hr/validate.go
func testAttestation(root string) int {
	pass, fail := 0, 0
	ctx := context.Background()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	const keyid = "spiffe://hr.local/test"
	now := time.Now()
	trust := attest.Bundle{Keys: map[string]attest.TrustKey{keyid: {KeyID: keyid, Public: pub}}}

	examplePath := filepath.Join(root, "contracts", "examples", "verdict.json")
	raw, err := os.ReadFile(examplePath)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	signed, err := attest.Sign(ctx, raw, keyid, priv)
	if err != nil {
		fmt.Println(err)
		return 1
	}

	result, err := attest.Verify(signed, trust, now)
	check(&pass, &fail, "round trip", err == nil && bytes.Equal(bytes.TrimSpace(result.Statement), bytes.TrimSpace(raw)),
		fmt.Sprintf("err=%v", err))

	forged := *signed
	tamperedRaw := bytes.Replace(raw, []byte(`"unverifiable"`), []byte(`"met"          `), 1)
	forged.Payload = base64.StdEncoding.EncodeToString(tamperedRaw)
	_, err = attest.Verify(&forged, trust, now)
	check(&pass, &fail, "tampered payload", err != nil && strings.Contains(err.Error(), "does not verify"), fmt.Sprintf("err=%v", err))

	// Validity windows and revocation, checked against when the artifact was
	// signed rather than against now. Revoking a key must kill what it signs
	// afterwards without killing the legitimate history behind it.
	revoked := attest.Bundle{Keys: map[string]attest.TrustKey{
		keyid: {KeyID: keyid, Public: pub, RevokedAt: now.Add(-time.Hour)},
	}}
	_, err = attest.Verify(signed, revoked, now)
	check(&pass, &fail, "signed after revocation is refused",
		err != nil && strings.Contains(err.Error(), "revoked"), fmt.Sprintf("err=%v", err))

	_, err = attest.Verify(signed, revoked, now.Add(-2*time.Hour))
	check(&pass, &fail, "signed before revocation still verifies",
		err == nil, fmt.Sprintf("err=%v", err))

	expired := attest.Bundle{Keys: map[string]attest.TrustKey{
		keyid: {KeyID: keyid, Public: pub, NotAfter: now.Add(-time.Hour)},
	}}
	_, err = attest.Verify(signed, expired, now)
	check(&pass, &fail, "expired key is refused",
		err != nil && strings.Contains(err.Error(), "expired"), fmt.Sprintf("err=%v", err))

	notyet := attest.Bundle{Keys: map[string]attest.TrustKey{
		keyid: {KeyID: keyid, Public: pub, NotBefore: now.Add(time.Hour)},
	}}
	_, err = attest.Verify(signed, notyet, now)
	check(&pass, &fail, "key used before it was valid is refused",
		err != nil && strings.Contains(err.Error(), "not valid until"), fmt.Sprintf("err=%v", err))

	// An unbounded key stays valid for all time, so a bundle written before
	// any of this existed keeps working.
	check(&pass, &fail, "an unbounded key verifies at any instant",
		func() bool {
			_, e1 := attest.Verify(signed, trust, now.Add(-100*time.Hour))
			_, e2 := attest.Verify(signed, trust, now.Add(100*time.Hour))
			return e1 == nil && e2 == nil
		}(), "")
	stripped := *signed
	stripped.Signatures = nil
	_, err = attest.Verify(&stripped, trust, now)
	check(&pass, &fail, "unsigned", err != nil && strings.Contains(err.Error(), "no signatures"), fmt.Sprintf("err=%v", err))

	foreign := *signed
	foreign.Signatures = append([]dsse.Signature{}, signed.Signatures...)
	foreign.Signatures[0].KeyID = "spiffe://hr.local/attacker"
	_, err = attest.Verify(&foreign, trust, now)
	check(&pass, &fail, "unknown keyid", err != nil && strings.Contains(err.Error(), "trust bundle"), fmt.Sprintf("err=%v", err))

	swapped := *signed
	swapped.PayloadType = "application/json"
	_, err = attest.Verify(&swapped, trust, now)
	check(&pass, &fail, "swapped payloadType", err != nil && strings.Contains(err.Error(), "payloadType"), fmt.Sprintf("err=%v", err))

	_, otherPriv, _ := ed25519.GenerateKey(nil)
	wrongKeySigned, _ := attest.Sign(ctx, raw, keyid, otherPriv)
	wrongKeySigned.Signatures[0].KeyID = keyid
	_, err = attest.Verify(wrongKeySigned, trust, now)
	check(&pass, &fail, "wrong key under trusted keyid", err != nil && strings.Contains(err.Error(), "does not verify"), fmt.Sprintf("err=%v", err))

	pae1 := dsse.PAE(statement.AttestationPayloadType, raw)
	pae2 := dsse.PAE("application/json", raw)
	check(&pass, &fail, "PAE binds payloadType", !bytes.Equal(pae1, pae2), "PAE output identical across payloadTypes")

	fmt.Printf("\n%d passed, %d failed\n", pass, fail)
	if fail > 0 {
		return 1
	}
	return 0
}
