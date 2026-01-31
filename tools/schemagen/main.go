// schemagen generates JSON Schema from the protocol package types.
//
// Usage: go run ./tools/schemagen > schema/protocol.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/invopop/jsonschema"
	"github.com/llimllib/hatchat/server/protocol"
)

func main() {
	// Use a single reflector so refs work properly
	r := jsonschema.Reflector{
		DoNotReference:             false, // Allow $refs
		ExpandedStruct:             false,
		AllowAdditionalProperties:  false,
		RequiredFromJSONSchemaTags: true,
	}

	// Collect all types we want to expose
	types := []any{
		protocol.User{},
		protocol.Room{},
		protocol.Message{},
		protocol.InitRequest{},
		protocol.SendMessageRequest{},
		protocol.HistoryRequest{},
		protocol.InitResponse{},
		protocol.HistoryResponse{},
		protocol.ErrorResponse{},
		protocol.Envelope{},
	}

	// Reflect all types to build up definitions
	allDefs := make(map[string]any)

	for _, t := range types {
		schema := r.Reflect(t)
		typeName := reflect.TypeOf(t).Name()

		// Marshal to JSON and back to get a clean map
		schemaBytes, _ := json.Marshal(schema)
		var schemaMap map[string]any
		_ = json.Unmarshal(schemaBytes, &schemaMap)

		// Copy nested definitions
		if defs, ok := schemaMap["$defs"].(map[string]any); ok {
			for name, def := range defs {
				allDefs[name] = def
			}
		}

		// The main type might be a $ref or a direct definition
		// Clean it up and add as a definition
		delete(schemaMap, "$schema")
		delete(schemaMap, "$id")
		delete(schemaMap, "$defs")

		// If the schema is just a $ref to itself, use the definition
		if ref, ok := schemaMap["$ref"].(string); ok {
			// Extract the definition name from the ref
			if def, exists := allDefs[typeName]; exists {
				allDefs[typeName] = def
			} else {
				// The ref might be something like #/$defs/TypeName
				// In this case the type should already be in allDefs
				_ = ref // Suppress unused warning
			}
		} else {
			allDefs[typeName] = schemaMap
		}
	}

	// Create the final combined schema
	combined := map[string]any{
		"$schema":     "https://json-schema.org/draft/2020-12/schema",
		"$id":         "hatchat-protocol",
		"title":       "Hatchat WebSocket Protocol",
		"description": "JSON Schema for all WebSocket messages in the Hatchat chat application",
		"$defs":       allDefs,
	}

	data, err := json.MarshalIndent(combined, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}
