# Server Registry API Generation

This directory contains the source files and tools for generating the OpenAPI specification for the MCP Server Registry API.

## Files

- **`openapi-manual.yaml`** - Hand-written API endpoints and registry-specific extensions
- **`openapi-components-generated.yaml`** - Auto-generated from `../../server-json/schema.json` (DO NOT EDIT)
- **`Makefile`** - Build commands for generating and bundling the specifications

The generated `openapi.yaml` is output to the parent directory (`../openapi.yaml`).

## Architecture

The OpenAPI specification is split into two parts to maintain DRY principles:

1. **Generated Components**: Common schemas are automatically generated from the canonical `server-json/schema.json` file
2. **Manual Definitions**: API-specific endpoints, extensions, and registry-specific fields are maintained manually

This approach ensures that:
- Core data models stay in sync with the JSON Schema source of truth
- API-specific concerns remain separate and maintainable
- The final specification is a complete, self-contained OpenAPI document

## Workflow

### Regenerating the OpenAPI Specification

```bash
# From this directory
make all

# Or from the project root
go generate ./...
```

This will:
1. Generate `openapi-components-generated.yaml` from `../../server-json/schema.json`
2. Bundle `openapi-manual.yaml` with the generated components into `../openapi.yaml`

### Making Changes

- **To modify core data models**: Edit `../../server-json/schema.json` and run `make all`
- **To modify API endpoints or registry-specific fields**: Edit `openapi-manual.yaml` and run `make bundle`
- **Never edit** `openapi-components-generated.yaml` or `openapi.yaml` directly

### Validating Changes

```bash
make validate
```

## Dependencies

- Go 1.22+ (for the schema converter)
- Node.js and npm (for the OpenAPI bundler)
- `@redocly/cli` (installed automatically when running make commands)

## Schema Extensions

The manual file extends generated schemas with registry-specific fields:

- `Server` adds an `id` field for registry identification
- `VersionDetail` adds `release_date` and `is_latest` fields
- `ServerList` is a registry-specific schema for pagination