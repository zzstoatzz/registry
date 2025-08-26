package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestLegacySeed creates a test seed file in the old format
func createTestLegacySeed(t *testing.T) string {
	t.Helper()

	legacyServers := []OldServerFormat{
		{
			ID:          "4e9cf4cf-71f6-4aca-bae8-2d10a29ca2e0",
			Name:        "io.github.21st-dev/magic-mcp",
			Description: "It's like v0 but in your Cursor/WindSurf/Cline. 21st dev Magic MCP server for working with your frontend like Magic",
			Repository: model.Repository{
				URL:    "https://github.com/21st-dev/magic-mcp",
				Source: "github",
			},
			VersionDetail: OldVersionDetail{
				Version:     "0.0.1-seed",
				ReleaseDate: "2025-05-16T18:56:49Z",
				IsLatest:    true,
			},
			Packages: []model.Package{
				{
					PackageType:  "javascript",
					RegistryName: "npm",
					Identifier:   "@21st-dev/magic",
					Version:      "0.0.46",
				},
			},
		},
		{
			ID:          "d3669201-252f-403c-944b-c3ec0845782b",
			Name:        "io.github.adfin-engineering/mcp-server-adfin",
			Description: "A Model Context Protocol Server for connecting with Adfin APIs",
			Repository: model.Repository{
				URL:    "https://github.com/adfin-engineering/mcp-server-adfin",
				Source: "github",
			},
			VersionDetail: OldVersionDetail{
				Version:     "0.0.1-seed",
				ReleaseDate: "2025-05-16T18:56:52Z",
				IsLatest:    true,
			},
			Packages: []model.Package{
				{
					PackageType:  "python",
					RegistryName: "pypi",
					Identifier:   "adfinmcp",
					Version:      "0.1.0",
				},
			},
		},
	}

	// Write to temporary file
	tempFile, err := os.CreateTemp("", "legacy-seed-*.json")
	require.NoError(t, err)

	data, err := json.MarshalIndent(legacyServers, "", "  ")
	require.NoError(t, err)

	_, err = tempFile.Write(data)
	require.NoError(t, err)

	err = tempFile.Close()
	require.NoError(t, err)

	return tempFile.Name()
}

func TestMigrateSeed_ConvertsLegacyFormat(t *testing.T) {
	// Create test legacy seed file
	legacyFile := createTestLegacySeed(t)
	defer os.Remove(legacyFile)

	// Create output file path
	outputFile := filepath.Join(t.TempDir(), "migrated-output.json")

	// Run migration
	err := migrateSeed(legacyFile, outputFile)
	require.NoError(t, err, "Migration should succeed")

	// Read the output file
	outputData, err := os.ReadFile(outputFile)
	require.NoError(t, err, "Should be able to read output file")

	// Parse as ServerResponse array (new format)
	var migratedServers []model.ServerResponse
	err = json.Unmarshal(outputData, &migratedServers)
	require.NoError(t, err, "Output should be valid JSON in ServerResponse format")

	// Verify we have the expected number of servers
	assert.Len(t, migratedServers, 2, "Should have migrated 2 servers")

	// Verify first server structure
	server1 := migratedServers[0]
	assert.Equal(t, "io.github.21st-dev/magic-mcp", server1.Server.Name)
	assert.Equal(t, "It's like v0 but in your Cursor/WindSurf/Cline. 21st dev Magic MCP server for working with your frontend like Magic", server1.Server.Description)
	assert.Equal(t, "https://github.com/21st-dev/magic-mcp", server1.Server.Repository.URL)
	assert.Equal(t, "0.0.1-seed", server1.Server.VersionDetail.Version)
	assert.Len(t, server1.Server.Packages, 1)
	assert.Equal(t, "javascript", server1.Server.Packages[0].PackageType)
	assert.Equal(t, "npm", server1.Server.Packages[0].RegistryName)
	assert.Equal(t, "@21st-dev/magic", server1.Server.Packages[0].Identifier)
	assert.Equal(t, "0.0.46", server1.Server.Packages[0].Version)

	// Verify registry metadata extension
	assert.NotNil(t, server1.XIOModelContextProtocolRegistry, "Should have registry metadata extension")
	registryMeta1, ok := server1.XIOModelContextProtocolRegistry.(map[string]interface{})
	require.True(t, ok, "Registry metadata should be a map")
	assert.Equal(t, "4e9cf4cf-71f6-4aca-bae8-2d10a29ca2e0", registryMeta1["id"])
	assert.Equal(t, true, registryMeta1["is_latest"])
	assert.Equal(t, "2025-05-16T18:56:49Z", registryMeta1["release_date"])

	// Verify second server structure
	server2 := migratedServers[1]
	assert.Equal(t, "io.github.adfin-engineering/mcp-server-adfin", server2.Server.Name)
	assert.Equal(t, "A Model Context Protocol Server for connecting with Adfin APIs", server2.Server.Description)
	assert.Equal(t, "python", server2.Server.Packages[0].PackageType)
	assert.Equal(t, "pypi", server2.Server.Packages[0].RegistryName)
	assert.Equal(t, "adfinmcp", server2.Server.Packages[0].Identifier)
	assert.Equal(t, "0.1.0", server2.Server.Packages[0].Version)

	// Verify registry metadata for second server
	registryMeta2, ok := server2.XIOModelContextProtocolRegistry.(map[string]interface{})
	require.True(t, ok, "Registry metadata should be a map")
	assert.Equal(t, "d3669201-252f-403c-944b-c3ec0845782b", registryMeta2["id"])
	assert.Equal(t, true, registryMeta2["is_latest"])
	assert.Equal(t, "2025-05-16T18:56:52Z", registryMeta2["release_date"])

	// Verify no x-publisher extensions (should be empty/nil for seed data)
	assert.Nil(t, server1.XPublisher, "Seed data should not have x-publisher extensions")
	assert.Nil(t, server2.XPublisher, "Seed data should not have x-publisher extensions")
}

// migrateSeed is a testable version of the main migration logic
func migrateSeed(inputFile, outputFile string) error {
	// Read source data
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return err
	}

	// Parse old format
	var oldServers []OldServerFormat
	if err := json.Unmarshal(data, &oldServers); err != nil {
		return err
	}

	// Convert to new format
	var newServers []model.ServerResponse
	for _, old := range oldServers {
		converted := convertServer(old)
		newServers = append(newServers, converted)
	}

	// Write migrated data
	migratedData, err := json.MarshalIndent(newServers, "", "  ")
	if err != nil {
		return err
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return err
	}

	return os.WriteFile(outputFile, migratedData, 0600)
}

func TestMigrationCLI_CommandLine(t *testing.T) {
	// Create test legacy seed file
	legacyFile := createTestLegacySeed(t)
	defer os.Remove(legacyFile)

	// Create output file path
	outputFile := filepath.Join(t.TempDir(), "cli-migrated-output.json")

	// Simulate command line arguments
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"migrate-seed", legacyFile, outputFile}

	// Capture any panics or exits from main()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main() panicked: %v", r)
		}
	}()

	// Test the main function (Note: this would normally call os.Exit, but in tests it should return)
	main()

	// Verify the output file was created and has correct content
	require.FileExists(t, outputFile, "Output file should be created")

	outputData, err := os.ReadFile(outputFile)
	require.NoError(t, err, "Should be able to read output file")

	var migratedServers []model.ServerResponse
	err = json.Unmarshal(outputData, &migratedServers)
	require.NoError(t, err, "Output should be valid JSON in ServerResponse format")

	// Basic verification
	assert.Len(t, migratedServers, 2, "Should have migrated 2 servers")
	assert.Equal(t, "io.github.21st-dev/magic-mcp", migratedServers[0].Server.Name)
	assert.Equal(t, "io.github.adfin-engineering/mcp-server-adfin", migratedServers[1].Server.Name)
}

func TestMigrationCLI_ErrorHandling(t *testing.T) {
	t.Run("Invalid JSON input", func(t *testing.T) {
		// Create invalid JSON file
		invalidFile, err := os.CreateTemp("", "invalid-*.json")
		require.NoError(t, err)
		defer os.Remove(invalidFile.Name())
		
		_, err = invalidFile.WriteString(`{"invalid": json}`)
		require.NoError(t, err)
		invalidFile.Close()

		outputFile := filepath.Join(t.TempDir(), "output.json")
		err = migrateSeed(invalidFile.Name(), outputFile)
		assert.Error(t, err, "Should error on invalid JSON")
	})

	t.Run("Missing input file", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "output.json")
		err := migrateSeed("/nonexistent/file.json", outputFile)
		assert.Error(t, err, "Should error on missing input file")
	})

	t.Run("Empty input file", func(t *testing.T) {
		// Create empty JSON array
		emptyFile, err := os.CreateTemp("", "empty-*.json")
		require.NoError(t, err)
		defer os.Remove(emptyFile.Name())
		
		_, err = emptyFile.WriteString(`[]`)
		require.NoError(t, err)
		emptyFile.Close()

		outputFile := filepath.Join(t.TempDir(), "output.json")
		err = migrateSeed(emptyFile.Name(), outputFile)
		assert.NoError(t, err, "Should handle empty array")

		// Verify output
		outputData, err := os.ReadFile(outputFile)
		require.NoError(t, err)

		var migratedServers []model.ServerResponse
		err = json.Unmarshal(outputData, &migratedServers)
		require.NoError(t, err)
		assert.Len(t, migratedServers, 0, "Should have 0 servers")
	})
}