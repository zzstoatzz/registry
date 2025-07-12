//go:build generate
// +build generate

// Package main provides the central entry point for all code generation in this project.
//
// This file uses Go's built-in code generation features to automate the creation of
// derived files from source schemas. Currently, it handles:
//
// 1. Converts JSON Schema (docs/server-json/schema.json) → OpenAPI components
// 2. Merges generated components + manual API definitions → final openapi.yaml
//
// Usage:
//   go generate -tags generate ./...
//
// This will run all //go:generate directives found in the project. The generate build
// tag ensures this file is only processed when explicitly running generation commands.
//
// Why use generate.go?
// - Standard Go convention that developers expect to work
// - Central location to discover all generation tasks
// - Easy integration with CI/CD pipelines
// - IDE/tool support for go:generate directives
//
// The actual generation logic is implemented in:
// - tools/server-json-to-openapi-sync/ (schema converter)
// - docs/server-registry-api/generation/Makefile (orchestration)
package main

// Generate OpenAPI components from JSON Schema and bundle into final OpenAPI spec
//go:generate make -C docs/server-registry-api/generation all