package main

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Simon0x/hr/internal/attest"
	"github.com/Simon0x/hr/internal/statement"
)

func cmdSign(root string, args []string) int {
	var file string
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			file = a
			break
		}
	}
	if file == "" {
		fmt.Fprintln(os.Stderr, "usage: hr sign <statement.json> [--out <envelope.json>]")
		return 2
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		fmt.Fprintf(os.Stderr, "not valid JSON: %v\n", err)
		return 2
	}
	_, hasSig := probe["signatures"]
	_, hasPT := probe["payloadType"]
	if hasSig || hasPT {
		fmt.Fprintf(os.Stderr, "%s is already an envelope, not a Statement\n", file)
		return 2
	}

	canonical, err := statement.Canonical(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	keyid, priv, err := attest.LoadSigningKey()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	envelope, err := attest.Sign(context.Background(), canonical, keyid, priv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	out, _ := json.MarshalIndent(envelope, "", "  ")

	if outPath, ok := flagValue(args, "out"); ok {
		if err := os.WriteFile(outPath, append(out, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		fmt.Fprintf(os.Stderr, "signed by %s -> %s\n", keyid, outPath)
		return 0
	}

	fmt.Println(string(out))
	return 0
}

var keyidPattern = regexp.MustCompile(`^spiffe://[a-z0-9.-]+/\S+$`)

func cmdKeygen(root string, args []string) int {
	keyid := "spiffe://hr.local/ci"
	for _, a := range args {
		if !strings.HasPrefix(a, "--") {
			keyid = a
			break
		}
	}
	if !keyidPattern.MatchString(keyid) {
		fmt.Fprintf(os.Stderr, "keyid %q does not look like spiffe://<trust-domain>/<path>\n", keyid)
		return 2
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}))

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))

	type trustEntryOut struct {
		KeyID     string `json:"keyid"`
		Algorithm string `json:"algorithm"`
		PublicKey string `json:"publicKey"`
		Note      string `json:"note"`
	}
	entry := trustEntryOut{
		KeyID: keyid, Algorithm: "ed25519", PublicKey: pubPEM,
		Note: "rotation and revocation are a declared blind spot - see AGENTS.md",
	}
	entryJSON, _ := json.MarshalIndent(entry, "", "  ")

	fmt.Println("# Add to trust/keys.json under .keys[]:")
	fmt.Println()
	fmt.Println(string(entryJSON))
	fmt.Println()
	fmt.Println("# Set as the CI secret HR_SIGNING_KEY (and HR_SIGNING_KEYID):")
	fmt.Println()
	fmt.Printf("HR_SIGNING_KEYID=%s\n", keyid)
	fmt.Print(privPEM)
	fmt.Println()

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "The private key above was not written to disk. Do not add it to the repo.")
	return 0
}
