package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Artifacts struct {
	In  []string `json:"in"`
	Out []string `json:"out"`
}

// Request is what a model was asked, on the entry for that invocation.
// The prompt is a digest, like policy and artifacts: enough to prove a
// claimed prompt is the one that ran, without copying step input into a
// committed file. ProcedureDigest pins the procedure's content, so editing
// departments/*.md cannot silently change what a past entry meant.
type Request struct {
	Harness         string   `json:"harness"`
	PromptDigest    string   `json:"promptDigest"`
	Tools           []string `json:"tools"`
	DeniedTools     []string `json:"deniedTools,omitempty"`
	Procedure       string   `json:"procedure,omitempty"`
	ProcedureDigest string   `json:"procedureDigest,omitempty"`
	// UntrustedInput marks an invocation whose prompt carried externally
	// derived text. Anything it produced - including a signed artifact -
	// rests on material hr did not author and could not vouch for.
	UntrustedInput bool `json:"untrustedInput,omitempty"`
}

// TextDigest hashes prompt and procedure content the same way the chain
// hashes entries, so one hash function checks anything in the ledger.
func TextDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

type Cost struct {
	Tokens   *float64 `json:"tokens,omitempty"`
	Seconds  *float64 `json:"seconds,omitempty"`
	Currency string   `json:"currency,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Model    string   `json:"model,omitempty"`
}

type Entry struct {
	Seq       int        `json:"seq"`
	Prev      string     `json:"prev"`
	At        string     `json:"at"`
	Kind      string     `json:"kind"`
	Actor     string     `json:"actor"`
	Artifacts *Artifacts `json:"artifacts,omitempty"`
	Goal      string     `json:"goal,omitempty"`
	Policy    string     `json:"policy,omitempty"`
	Outcome   string     `json:"outcome,omitempty"`
	Detail    string     `json:"detail,omitempty"`
	Cost      *Cost      `json:"cost,omitempty"`
	Request   *Request   `json:"request,omitempty"`
}

func Canonical(e Entry) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
