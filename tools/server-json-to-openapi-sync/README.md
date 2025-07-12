# server-json-to-openapi-sync

A tool that converts JSON Schema definitions to OpenAPI component schemas.

## Purpose

This tool bridges the gap between our canonical `server.json` schema (JSON Schema format) and our OpenAPI specifications. It ensures that data models stay synchronized between the two formats without manual duplication.

## How it works

1. **Input**: Reads a JSON Schema file (e.g., `docs/server-json/schema.json`)
2. **Transform**: Converts JSON Schema `$defs` to OpenAPI `components/schemas`
3. **Output**: Generates an OpenAPI components file with converted schemas

### Key transformations:
- `$ref: "#/$defs/Foo"` → `$ref: "#/components/schemas/Foo"`
- Preserves all schema properties (type, required, properties, etc.)
- Maintains descriptions, examples, and validation rules

## Usage

```bash
go run . -s <source-json-schema> -o <output-openapi-components>

# Example:
go run . -s ../../docs/server-json/schema.json \
         -o ../../docs/server-registry-api/generation/openapi-components-generated.yaml
```

## Integration

This tool is typically called via:
1. `go generate ./...` from the project root
2. `make generate` in `docs/server-registry-api/generation/`

The generated components are then bundled with manual API definitions to create the final OpenAPI specification.

## Why not use existing tools?

- Most JSON Schema → OpenAPI converters handle full documents, not just components
- We need precise control over the transformation to maintain compatibility
- This approach allows mixing generated schemas with manual API-specific overrides