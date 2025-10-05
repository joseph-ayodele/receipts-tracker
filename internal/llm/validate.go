package llm

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

// ValidateJSONAgainstSchema validates "data" against "schemaMap".
func ValidateJSONAgainstSchema(schemaMap map[string]any, data []byte) error {
	b, err := json.Marshal(schemaMap)
	if err != nil {
		return fmt.Errorf("marshal schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", bytes.NewReader(b)); err != nil {
		return fmt.Errorf("add schema: %w", err)
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("unmarshal data: %w", err)
	}
	if err := schema.Validate(v); err != nil {
		return fmt.Errorf("json does not match schema: %w", err)
	}
	return nil
}
