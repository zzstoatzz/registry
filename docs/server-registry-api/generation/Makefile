# Makefile for generating OpenAPI documentation

.PHONY: all generate bundle clean

# Default target
all: generate bundle

# Generate OpenAPI components from JSON Schema
generate:
	@echo "Generating OpenAPI components from JSON Schema..."
	@cd ../../../tools/server-json-to-openapi-sync && go run . \
		-s ../../docs/server-json/schema.json \
		-o ../../docs/server-registry-api/generation/openapi-components-generated.yaml

# Bundle the manual and generated files into final openapi.yaml
bundle: generate
	@echo "Bundling OpenAPI files..."
	@npx @redocly/cli bundle openapi-manual.yaml -o ../openapi.yaml

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@rm -f openapi-components-generated.yaml ../openapi.yaml

# Install required tools
install-tools:
	@echo "Installing required tools..."
	@npm install -g @redocly/cli

# Validate the final OpenAPI spec
validate: bundle
	@echo "Validating OpenAPI spec..."
	@npx @redocly/cli lint ../openapi.yaml