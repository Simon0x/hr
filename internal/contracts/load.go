package contracts

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func addResourceFile(compiler *jsonschema.Compiler, id, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}
	return compiler.AddResource(id, doc)
}

func schemaID(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var head struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return "", fmt.Errorf("parsing JSON: %w", err)
	}
	return head.ID, nil
}
