package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/registry/internal/model"
)

// OldPackage represents the legacy package format
type OldPackage struct {
	RegistryName         string                     `json:"registry_name"`
	Name                 string                     `json:"name"`
	Version              string                     `json:"version,omitempty"`
	RunTimeHint          string                     `json:"runtime_hint,omitempty"`
	RuntimeArguments     []model.Argument           `json:"runtime_arguments,omitempty"`
	PackageArguments     []model.Argument           `json:"package_arguments,omitempty"`
	EnvironmentVariables []model.KeyValueInput      `json:"environment_variables,omitempty"`
}

// OldServerFormat represents the legacy seed format
type OldServerFormat struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Status        string                     `json:"status,omitempty"`
	Repository    model.Repository           `json:"repository"`
	VersionDetail OldVersionDetail           `json:"version_detail"`
	Packages      []OldPackage               `json:"packages,omitempty"`
	Remotes       []model.Remote             `json:"remotes,omitempty"`
	Extensions    map[string]interface{}     `json:"extensions,omitempty"`
}

// OldVersionDetail represents the legacy version format with registry metadata
type OldVersionDetail struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date,omitempty"`
	IsLatest    bool   `json:"is_latest,omitempty"`
}

func main() {
	if len(os.Args) < 3 {
		log.Println("Usage: migrate-seed <input-source> <output-file>")
		log.Println("  input-source: file path or HTTP URL")
		log.Println("  output-file: path for the migrated seed file")
		os.Exit(1)
	}

	inputSource := os.Args[1]
	outputFile := os.Args[2]

	log.Printf("Migrating seed from %s to %s", inputSource, outputFile)

	// Read source data
	data, err := readSource(inputSource)
	if err != nil {
		log.Fatalf("Failed to read source: %v", err)
	}

	// Parse old format
	var oldServers []OldServerFormat
	if err := json.Unmarshal(data, &oldServers); err != nil {
		log.Fatalf("Failed to parse source JSON: %v", err)
	}

	log.Printf("Found %d servers to migrate", len(oldServers))

	// Convert to new format
	var newServers []model.ServerResponse
	for _, old := range oldServers {
		converted := convertServer(old)
		newServers = append(newServers, converted)
	}

	// Write migrated data
	migratedData, err := json.MarshalIndent(newServers, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal migrated data: %v", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	if err := os.WriteFile(outputFile, migratedData, 0600); err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	log.Printf("Successfully migrated %d servers to %s", len(newServers), outputFile)
}

// readSource reads data from a file or HTTP URL
func readSource(source string) ([]byte, error) {
	// Check if source is an HTTP URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return readHTTP(source)
	}

	// Otherwise, treat as file path
	return os.ReadFile(source)
}

// readHTTP fetches data from an HTTP URL
func readHTTP(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MCP-Registry-Seed-Migrator/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// determinePackageType infers package type from registry name
func determinePackageType(registryName string) string {
	switch strings.ToLower(registryName) {
	case "npm":
		return "javascript"
	case "pypi":
		return "python"
	case "docker-hub":
		return "docker"
	case "nuget":
		return "dotnet"
	case "github-releases", "gitlab-releases":
		return "binary"
	default:
		return ""
	}
}

func convertServer(old OldServerFormat) model.ServerResponse {
	// Convert old packages to new format
	var packages []model.Package
	for _, oldPkg := range old.Packages {
		newPkg := model.Package{
			PackageType:          determinePackageType(oldPkg.RegistryName),
			RegistryName:         oldPkg.RegistryName,
			Identifier:           oldPkg.Name,
			Version:              oldPkg.Version,
			RunTimeHint:          oldPkg.RunTimeHint,
			RuntimeArguments:     oldPkg.RuntimeArguments,
			PackageArguments:     oldPkg.PackageArguments,
			EnvironmentVariables: oldPkg.EnvironmentVariables,
		}
		packages = append(packages, newPkg)
	}

	// Create pure MCP server specification
	server := model.ServerDetail{
		Name:        old.Name,
		Description: old.Description,
		Repository:  old.Repository,
		VersionDetail: model.VersionDetail{
			Version: old.VersionDetail.Version,
		},
		Packages: packages,
		Remotes:  old.Remotes,
	}

	// Set status if provided, otherwise default to active
	if old.Status != "" {
		server.Status = model.ServerStatus(old.Status)
	} else {
		server.Status = model.ServerStatusActive
	}

	// Create registry metadata from old version detail
	// Use zero time for seed data to indicate it's not from actual publish
	registryMetadata := model.RegistryMetadata{
		ID:          old.ID,
		PublishedAt: time.Time{}, // Zero time for seed data
		UpdatedAt:   time.Time{}, // Zero time for seed data
		IsLatest:    old.VersionDetail.IsLatest,
		ReleaseDate: old.VersionDetail.ReleaseDate,
	}

	// Create response with extension wrapper format
	response := model.ServerResponse{
		Server:                          server,
		XIOModelContextProtocolRegistry: registryMetadata.CreateRegistryExtensions()["x-io.modelcontextprotocol.registry"],
	}

	// Add any x-publisher extensions if present
	if extensions, ok := old.Extensions["x-publisher"]; ok {
		response.XPublisher = extensions
	}

	return response
}