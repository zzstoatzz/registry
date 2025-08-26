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

// LegacyPackage represents the legacy package format with registry_name and name fields
type LegacyPackage struct {
	RegistryName         string                 `json:"registry_name,omitempty"`
	Name                 string                 `json:"name,omitempty"`
	Version              string                 `json:"version,omitempty"`
	RunTimeHint          string                 `json:"runtime_hint,omitempty"`
	RuntimeArguments     []model.Argument       `json:"runtime_arguments,omitempty"`
	PackageArguments     []model.Argument       `json:"package_arguments,omitempty"`
	EnvironmentVariables []model.KeyValueInput  `json:"environment_variables,omitempty"`
	FileHashes           map[string]string `json:"file_hashes,omitempty"`
}

// OldServerFormat represents the legacy seed format
type OldServerFormat struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Status        string                     `json:"status,omitempty"`
	Repository    model.Repository           `json:"repository"`
	VersionDetail OldVersionDetail           `json:"version_detail"`
	Packages      []LegacyPackage            `json:"packages,omitempty"`
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
	var data []byte
	var err error

	if strings.HasPrefix(inputSource, "http://") || strings.HasPrefix(inputSource, "https://") {
		data, err = fetchFromHTTP(inputSource)
	} else {
		data, err = os.ReadFile(inputSource)
	}
	
	if err != nil {
		log.Fatalf("Failed to read input source: %v", err)
	}

	// Parse old format
	var oldServers []OldServerFormat
	if err := json.Unmarshal(data, &oldServers); err != nil {
		log.Fatalf("Failed to parse old format: %v", err)
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

func fetchFromHTTP(url string) ([]byte, error) {
	log.Printf("Fetching data from HTTP: %s", url)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
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

// convertLegacyPackage converts a legacy package to the new format
func convertLegacyPackage(legacy LegacyPackage) model.Package {
	// Constants for registry names
	const (
		registryNPM    = "npm"
		registryPyPI   = "pypi"
		registryDocker = "docker"
		registryNuGet  = "nuget"
		registryURL    = "url"
	)
	
	// Determine package_type and registry from legacy fields
	var packageType, registry, identifier string
	
	switch legacy.RegistryName {
	case registryNPM:
		packageType = "javascript"
		registry = registryNPM
		identifier = legacy.Name
	case registryPyPI:
		packageType = "python"
		registry = registryPyPI
		identifier = legacy.Name
	case registryDocker:
		packageType = "docker"
		registry = "docker-hub"
		identifier = legacy.Name
	case registryNuGet:
		packageType = "dotnet"
		registry = registryNuGet
		identifier = legacy.Name
	case registryURL:
		// For URL-based packages, determine type from the URL
		if strings.HasSuffix(legacy.Name, ".mcpb") {
			packageType = "mcpb"
		} else {
			packageType = "binary"
		}
		// Determine registry from URL
		switch {
		case strings.Contains(legacy.Name, "github.com") && strings.Contains(legacy.Name, "/releases/"):
			registry = "github-releases"
		case strings.Contains(legacy.Name, "gitlab.com") && strings.Contains(legacy.Name, "/releases/"):
			registry = "gitlab-releases"
		default:
			registry = "url"
		}
		identifier = legacy.Name
	default:
		// Unknown registry type
		packageType = "unknown"
		registry = legacy.RegistryName
		identifier = legacy.Name
	}

	pkg := model.Package{
		PackageType:          packageType,
		Registry:             registry,
		Identifier:           identifier,
		Version:              legacy.Version,
		RunTimeHint:          legacy.RunTimeHint,
		RuntimeArguments:     legacy.RuntimeArguments,
		PackageArguments:     legacy.PackageArguments,
		EnvironmentVariables: legacy.EnvironmentVariables,
		FileHashes:           legacy.FileHashes,
	}

	return pkg
}

func convertServer(old OldServerFormat) model.ServerResponse {
	// Convert packages from legacy to new format
	var packages []model.Package
	for _, legacyPkg := range old.Packages {
		packages = append(packages, convertLegacyPackage(legacyPkg))
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

	// Create registry metadata
	registryMetadata := model.RegistryMetadata{
		ID:          old.ID,
		IsLatest:    old.VersionDetail.IsLatest,
		ReleaseDate: old.VersionDetail.ReleaseDate,
	}

	// Publisher extensions (combine any additional fields)
	publisherExtensions := make(map[string]interface{})
	if old.Extensions != nil {
		for k, v := range old.Extensions {
			publisherExtensions[k] = v
		}
	}

	// Create the extension wrapper response
	response := model.ServerResponse{
		Server: server,
	}

	// Add registry metadata extension
	response.XIOModelContextProtocolRegistry = registryMetadata.CreateRegistryExtensions()["x-io.modelcontextprotocol.registry"]
	
	// Add publisher extensions if any
	if len(publisherExtensions) > 0 {
		response.XPublisher = publisherExtensions
	}

	return response
}