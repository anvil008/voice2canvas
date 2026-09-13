// Package a2ui contains the small amount of A2UI protocol handling owned by
// the backend. The actual v0.9.1 schemas are vendored alongside this file so
// validation does not depend on a network request at runtime.
package a2ui

import (
	"bytes"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	Version = "v0.9.1"

	// BasicCatalogID is the catalog identifier required by PROTOCOL.md. The
	// upstream v0.9.1 schema currently uses a v0_9 URL internally; both URLs
	// are registered by NewValidator so its references remain resolvable.
	BasicCatalogID = "https://a2ui.org/specification/v0_9_1/catalogs/basic/catalog.json"
	// ExtendedCatalogID is the v2ui catalog used for generated dashboard cards.
	// It extends the basic catalog with charts, stats, gauges, and tables.
	ExtendedCatalogID = "https://v2ui.local/catalogs/extended/v1"

	serverSchemaID       = "https://a2ui.org/specification/v0_9/server_to_client.json"
	catalogSchemaID      = "https://a2ui.org/specification/v0_9/catalog.json"
	basicCatalogSchemaID = "https://a2ui.org/specification/v0_9/catalogs/basic/catalog.json"
	commonSchemaID       = "https://a2ui.org/specification/v0_9/common_types.json"
	extendedSchemaID     = "https://v2ui.local/catalogs/extended/v1/components.schema.json"
)

var extendedComponentNames = []string{
	"Stat",
	"StatGroup",
	"LineChart",
	"BarChart",
	"Gauge",
	"ProgressBar",
	"KeyValueList",
	"Badge",
	"DataTable",
}

//go:embed schema/*.json
var schemaFiles embed.FS

// Validator validates individual A2UI server-to-client messages.
type Validator struct {
	schema             *jsonschema.Schema
	extendedComponents map[string]*jsonschema.Schema
}

// NewValidator compiles the vendored A2UI schema graph.
func NewValidator() (*Validator, error) {
	compiler := jsonschema.NewCompiler()

	resources := map[string]string{
		serverSchemaID:       "schema/server_to_client.json",
		catalogSchemaID:      "schema/catalog.json",
		basicCatalogSchemaID: "schema/catalog.json",
		commonSchemaID:       "schema/common_types.json",
		extendedSchemaID:     "schema/extended-v1.schema.json",
	}
	for location, filename := range resources {
		if err := addJSONResource(compiler, location, filename); err != nil {
			return nil, err
		}
	}

	compiled, err := compiler.Compile(serverSchemaID)
	if err != nil {
		return nil, fmt.Errorf("compile A2UI v0.9.1 schema: %w", err)
	}
	extended := make(map[string]*jsonschema.Schema, len(extendedComponentNames))
	for _, name := range extendedComponentNames {
		componentSchema, compileErr := compiler.Compile(extendedSchemaID + "#/$defs/" + name)
		if compileErr != nil {
			return nil, fmt.Errorf("compile extended component %s schema: %w", name, compileErr)
		}
		extended[name] = componentSchema
	}
	return &Validator{schema: compiled, extendedComponents: extended}, nil
}

func addJSONResource(compiler *jsonschema.Compiler, location, filename string) error {
	data, err := schemaFiles.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read vendored A2UI schema %s: %w", filename, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parse vendored A2UI schema %s: %w", filename, err)
	}
	if err := compiler.AddResource(location, doc); err != nil {
		return fmt.Errorf("register vendored A2UI schema %s: %w", filename, err)
	}
	return nil
}

// ValidateMessage validates one JSON object before it is sent to the client.
func (v *Validator) ValidateMessage(message map[string]any) error {
	if v == nil || v.schema == nil {
		return fmt.Errorf("A2UI validator is not initialized")
	}
	validationMessage, err := v.validateExtendedComponents(message)
	if err != nil {
		return err
	}
	if err := v.schema.Validate(validationMessage); err != nil {
		return err
	}
	return nil
}

// validateExtendedComponents validates each extended component against its
// named definition, then substitutes a valid basic Text component before the
// outer v0.9.1 message schema runs. Basic and unknown component names remain
// untouched, so the vendored basic catalog continues to accept or reject them.
func (v *Validator) validateExtendedComponents(message map[string]any) (map[string]any, error) {
	update, ok := message["updateComponents"].(map[string]any)
	if !ok {
		return message, nil
	}
	components, ok := update["components"].([]any)
	if !ok {
		return message, nil
	}

	rewritten := make([]any, len(components))
	copy(rewritten, components)
	changed := false
	for index, rawComponent := range components {
		component, ok := rawComponent.(map[string]any)
		if !ok {
			continue
		}
		name, _ := component["component"].(string)
		schema, extended := v.extendedComponents[name]
		if !extended {
			continue
		}
		if err := schema.Validate(component); err != nil {
			return nil, fmt.Errorf("extended component %q at index %d: %w", name, index, err)
		}
		id, _ := component["id"].(string)
		rewritten[index] = map[string]any{
			"id":        id,
			"component": "Text",
			"text":      "",
			"variant":   "body",
		}
		changed = true
	}
	if !changed {
		return message, nil
	}

	rewrittenUpdate := make(map[string]any, len(update))
	for key, value := range update {
		rewrittenUpdate[key] = value
	}
	rewrittenUpdate["components"] = rewritten
	rewrittenMessage := make(map[string]any, len(message))
	for key, value := range message {
		rewrittenMessage[key] = value
	}
	rewrittenMessage["updateComponents"] = rewrittenUpdate
	return rewrittenMessage, nil
}

// ValidateMessages validates messages in wire order and identifies the first
// invalid index in its error.
func (v *Validator) ValidateMessages(messages []map[string]any) error {
	for index, message := range messages {
		if err := v.ValidateMessage(message); err != nil {
			return fmt.Errorf("A2UI message %d: %w", index, err)
		}
	}
	return nil
}

// ValidationSummary makes jsonschema errors useful in a repair prompt without
// depending on the package's internal error formatting.
func ValidationSummary(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "unknown A2UI validation error"
	}
	return message
}

// SortedKeys is a tiny helper used by diagnostics and tests when rendering
// deterministic details from arbitrary JSON objects.
func SortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
