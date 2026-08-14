package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var lowercaseHex = regexp.MustCompile(`^[a-f0-9]+$`)

var statementFields = map[string]bool{
	"_type": true, "subject": true, "predicateType": true, "predicate": true,
}

func ValidateStatement(reg *Registry, raw []byte, where string) []string {
	var problems []string

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return []string{fmt.Sprintf("%s: not valid JSON: %v", where, err)}
	}

	if err := reg.Statement.Validate(doc); err != nil {
		problems = append(problems, fmt.Sprintf("%s: %v", where, err))
	}

	problems = append(problems, assertInTotoShape(raw, where)...)

	var stmt struct {
		PredicateType string          `json:"predicateType"`
		Predicate     json.RawMessage `json:"predicate"`
	}
	if err := json.Unmarshal(raw, &stmt); err != nil {
		return problems
	}
	if stmt.PredicateType == "" {
		return problems
	}

	schema, ok := reg.Lookup(stmt.PredicateType)
	if !ok {
		problems = append(problems, fmt.Sprintf(
			"%s: unknown predicateType %q — refusing rather than guessing", where, stmt.PredicateType))
		return problems
	}

	predDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(stmt.Predicate))
	if err != nil {
		problems = append(problems, fmt.Sprintf("%s.predicate: not valid JSON: %v", where, err))
		return problems
	}
	if err := schema.Validate(predDoc); err != nil {
		problems = append(problems, fmt.Sprintf("%s.predicate: %v", where, err))
	}

	return problems
}

func assertInTotoShape(raw []byte, where string) []string {
	var problems []string

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil
	}

	var extras []string
	for k := range top {
		if !statementFields[k] {
			extras = append(extras, k)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		problems = append(problems, fmt.Sprintf(
			"%s: top-level field(s) %v - a Statement has exactly four, and there is no "+
				"extension point at this level. Move them into predicate.", where, extras))
	}

	var stmt struct {
		Subject []struct {
			Digest map[string]string `json:"digest"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(raw, &stmt); err != nil {
		return problems
	}
	for i, s := range stmt.Subject {
		if len(s.Digest) == 0 {
			problems = append(problems, fmt.Sprintf(
				"%s.subject[%d]: no digest - subjects are matched by digest alone, "+
					"so this can never be verified", where, i))
			continue
		}
		for alg, hex := range s.Digest {
			if !lowercaseHex.MatchString(hex) {
				problems = append(problems, fmt.Sprintf(
					"%s.subject[%d].digest.%s: %q is not lowercase hex", where, i, alg, hex))
			}
		}
	}

	return problems
}
