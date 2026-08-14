package contracts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const statementSchemaID = "hr/statement/v1"

var predicateTypePattern = regexp.MustCompile(`^https://hr\.dev/[a-z]+/v[0-9]+$`)

type Registry struct {
	Statement  *jsonschema.Schema
	Predicates map[string]*jsonschema.Schema
}

func lintSchemaFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	return AssertSupportedKeywords(doc, path)
}

func Load(root string) (*Registry, error) {
	compiler := jsonschema.NewCompiler()

	stmtPath := filepath.Join(root, "contracts", "statement.schema.json")
	if err := lintSchemaFile(stmtPath); err != nil {
		return nil, err
	}
	if err := addResourceFile(compiler, statementSchemaID, stmtPath); err != nil {
		return nil, fmt.Errorf("loading statement schema: %w", err)
	}
	stmtSchema, err := compiler.Compile(statementSchemaID)
	if err != nil {
		return nil, fmt.Errorf("compiling statement schema: %w", err)
	}

	predicatesDir := filepath.Join(root, "contracts", "predicates")
	paths, err := filepath.Glob(filepath.Join(predicatesDir, "*.schema.json"))
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", predicatesDir, err)
	}

	predicates := map[string]*jsonschema.Schema{}
	for _, p := range paths {
		name := filepath.Base(p)
		id, err := schemaID(p)
		if err != nil {
			return nil, fmt.Errorf("contracts/predicates/%s: %w", name, err)
		}
		if id == "" {
			return nil, fmt.Errorf("contracts/predicates/%s: no $id", name)
		}
		if err := lintSchemaFile(p); err != nil {
			return nil, err
		}
		if err := addResourceFile(compiler, id, p); err != nil {
			return nil, fmt.Errorf("loading contracts/predicates/%s: %w", name, err)
		}
		schema, err := compiler.Compile(id)
		if err != nil {
			return nil, fmt.Errorf("compiling contracts/predicates/%s: %w", name, err)
		}
		predicates[id] = schema
	}

	return &Registry{Statement: stmtSchema, Predicates: predicates}, nil
}

func (r *Registry) Lookup(predicateType string) (*jsonschema.Schema, bool) {
	s, ok := r.Predicates[predicateType]
	return s, ok
}

func ValidPredicateTypeFormat(predicateType string) bool {
	return predicateTypePattern.MatchString(predicateType)
}
