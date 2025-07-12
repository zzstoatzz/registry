package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type JSONSchema struct {
	Schema      string                 `json:"$schema,omitempty" yaml:"$schema,omitempty"`
	ID          string                 `json:"$id,omitempty" yaml:"$id,omitempty"`
	Title       string                 `json:"title,omitempty" yaml:"title,omitempty"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Type        interface{}            `json:"type,omitempty" yaml:"type,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required    []string               `json:"required,omitempty" yaml:"required,omitempty"`
	Ref         string                 `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Defs        map[string]interface{} `json:"$defs,omitempty" yaml:"$defs,omitempty"`
	AllOf       []interface{}          `json:"allOf,omitempty" yaml:"allOf,omitempty"`
	AnyOf       []interface{}          `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	Items       interface{}            `json:"items,omitempty" yaml:"items,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty" yaml:"enum,omitempty"`
	Format      string                 `json:"format,omitempty" yaml:"format,omitempty"`
	Default     interface{}            `json:"default,omitempty" yaml:"default,omitempty"`
	Example     interface{}            `json:"example,omitempty" yaml:"example,omitempty"`
	Examples    []interface{}          `json:"examples,omitempty" yaml:"examples,omitempty"`
	Pattern     string                 `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MinItems    *int                   `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems    *int                   `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	AdditionalProperties interface{}   `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
}

type OpenAPIComponents struct {
	Schemas map[string]interface{} `yaml:"schemas"`
}

type OpenAPIDocument struct {
	Components OpenAPIComponents `yaml:"components"`
}

var rootCmd = &cobra.Command{
	Use:   "server-json-to-openapi-sync",
	Short: "Convert server.json schema to OpenAPI components",
	Run: func(cmd *cobra.Command, args []string) {
		source, _ := cmd.Flags().GetString("source")
		output, _ := cmd.Flags().GetString("output")

		if err := convertSchema(source, output); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().StringP("source", "s", "", "Source JSON Schema file (required)")
	rootCmd.Flags().StringP("output", "o", "", "Output OpenAPI components file (required)")
	rootCmd.MarkFlagRequired("source")
	rootCmd.MarkFlagRequired("output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func convertSchema(sourcePath, outputPath string) error {
	// Read JSON Schema
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	var schema JSONSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("failed to parse JSON Schema: %w", err)
	}

	// Convert to OpenAPI components
	components := OpenAPIComponents{
		Schemas: make(map[string]interface{}),
	}

	// Process $defs
	if schema.Defs != nil {
		for name, def := range schema.Defs {
			components.Schemas[name] = convertSchemaToOpenAPI(def)
		}
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create OpenAPI document with header comment
	doc := OpenAPIDocument{
		Components: components,
	}

	// Write YAML with header
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Write header comment
	header := fmt.Sprintf(`# THIS FILE IS AUTO-GENERATED. DO NOT EDIT.
# Generated from %s
# Run 'go generate' to regenerate this file.

`, sourcePath)
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write YAML content
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	if err := encoder.Encode(doc); err != nil {
		return fmt.Errorf("failed to write YAML: %w", err)
	}

	fmt.Printf("Successfully converted %s to %s\n", sourcePath, outputPath)
	return nil
}

func convertSchemaToOpenAPI(schema interface{}) interface{} {
	switch v := schema.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Convert $ref paths from JSON Schema format to relative OpenAPI format
			if key == "$ref" {
				if ref, ok := value.(string); ok && strings.HasPrefix(ref, "#/$defs/") {
					result["$ref"] = "#/components/schemas/" + strings.TrimPrefix(ref, "#/$defs/")
					continue
				}
			}
			// Convert $defs to properties in nested schemas
			if key == "$defs" {
				// Skip $defs in nested schemas
				continue
			}
			// Recursively convert nested structures
			result[key] = convertSchemaToOpenAPI(value)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertSchemaToOpenAPI(item)
		}
		return result
	default:
		return v
	}
}