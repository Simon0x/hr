package statement

import "encoding/json"

type Statement struct {
	Type          string          `json:"_type"`
	Subject       []Subject       `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

type Subject struct {
	Name   string            `json:"name,omitempty"`
	Digest map[string]string `json:"digest"`
}

const StatementType = "https://in-toto.io/Statement/v1"
const AttestationPayloadType = "application/vnd.in-toto+json"
