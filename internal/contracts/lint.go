package contracts

import "fmt"

var supportedKeywords = map[string]bool{
	"$schema": true, "$id": true, "title": true, "description": true,
	"type": true, "required": true, "properties": true, "additionalProperties": true,
	"items": true, "minItems": true, "minProperties": true, "minLength": true,
	"enum": true, "const": true, "pattern": true,
}

// AssertSupportedKeywords restores the guarantee the hand-rolled TS
// validator this project started with gave for free: an unrecognized
// keyword in one of hr's own schema files is a hard error at load time, not
// a silently-ignored annotation. Standard JSON Schema treats unknown
// keywords as harmless by design, so santhosh-tekuri/jsonschema/v6 (correctly)
// won't reject them on instance validation - this lints the schema authoring
// surface specifically, separate from instance validation.
func AssertSupportedKeywords(schema map[string]any, where string) error {
	for key := range schema {
		if !supportedKeywords[key] {
			return fmt.Errorf("%s: unsupported JSON Schema keyword %q — hr's schemas use a fixed keyword set; "+
				"add it to supportedKeywords and confirm the validator honors it, or express the rule differently", where, key)
		}
	}

	if props, ok := schema["properties"].(map[string]any); ok {
		for name, sub := range props {
			subSchema, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			if err := AssertSupportedKeywords(subSchema, fmt.Sprintf("%s.properties.%s", where, name)); err != nil {
				return err
			}
		}
	}

	if items, ok := schema["items"].(map[string]any); ok {
		if err := AssertSupportedKeywords(items, where+".items"); err != nil {
			return err
		}
	}

	if ap, ok := schema["additionalProperties"].(map[string]any); ok {
		if err := AssertSupportedKeywords(ap, where+".additionalProperties"); err != nil {
			return err
		}
	}

	return nil
}
