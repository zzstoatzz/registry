//nolint:testpackage
package service

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/database"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishMaintainsConsistentIDsAcrossVersions(t *testing.T) {
	// Create in-memory database and service
	db := database.NewMemoryDB()
	cfg := &config.Config{
		EnableRegistryValidation: false,
	}
	svc := NewRegistryService(db, cfg)

	// Create base server JSON for version 1.0.0
	server1 := apiv0.ServerJSON{
		Schema:      "https://static.modelcontextprotocol.io/schemas/2025-07-09/server.schema.json",
		Name:        "com.example/test-server",
		Description: "Test server",
		Status:      "active",
		Version:     "1.0.0",
		Repository: model.Repository{
			URL:    "https://github.com/test/repo",
			Source: "github",
		},
		Remotes: []model.Transport{
			{
				Type: "streamable-http",
				URL:  "https://example.com/test-server/mcp",
			},
		},
	}

	// Publish version 1.0.0
	published1, err := svc.Publish(server1)
	require.NoError(t, err, "Failed to publish version 1.0.0")
	require.NotNil(t, published1.Meta)
	require.NotNil(t, published1.Meta.Official)

	// Store the ID from the first version
	firstVersionID := published1.Meta.Official.ID
	assert.NotEmpty(t, firstVersionID, "First version should have an ID")
	assert.True(t, published1.Meta.Official.IsLatest, "First version should be marked as latest")

	// Wait a bit to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create version 1.0.1 (patch update)
	server2 := server1
	server2.Version = "1.0.1"
	server2.Description = "Test server - updated"

	// Publish version 1.0.1
	published2, err := svc.Publish(server2)
	require.NoError(t, err, "Failed to publish version 1.0.1")
	require.NotNil(t, published2.Meta)
	require.NotNil(t, published2.Meta.Official)

	// Check that the ID is the same as the first version
	assert.Equal(t, firstVersionID, published2.Meta.Official.ID,
		"Version 1.0.1 should have the same ID as version 1.0.0")
	assert.True(t, published2.Meta.Official.IsLatest, "Version 1.0.1 should be marked as latest")

	// Wait a bit to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create version 2.0.0 (major update)
	server3 := server1
	server3.Version = "2.0.0"
	server3.Description = "Test server - major update"

	// Publish version 2.0.0
	published3, err := svc.Publish(server3)
	require.NoError(t, err, "Failed to publish version 2.0.0")
	require.NotNil(t, published3.Meta)
	require.NotNil(t, published3.Meta.Official)

	// Check that the ID is still the same as the first version
	assert.Equal(t, firstVersionID, published3.Meta.Official.ID,
		"Version 2.0.0 should have the same ID as version 1.0.0")
	assert.True(t, published3.Meta.Official.IsLatest, "Version 2.0.0 should be marked as latest")

	// Now let's retrieve all versions and verify they all have the same ID
	filter := &database.ServerFilter{Name: &server1.Name}
	allVersions, _, err := svc.List(filter, "", 10)
	require.NoError(t, err, "Failed to list server versions")
	assert.Len(t, allVersions, 3, "Should have 3 versions")

	// Verify all versions have the same ID
	for i, version := range allVersions {
		require.NotNil(t, version.Meta, "Version %d should have meta", i)
		require.NotNil(t, version.Meta.Official, "Version %d should have official meta", i)
		assert.Equal(t, firstVersionID, version.Meta.Official.ID,
			"All versions should have the same ID (version index %d)", i)
	}

	// Verify only the latest version is marked as latest
	latestCount := 0
	for _, version := range allVersions {
		if version.Meta.Official.IsLatest {
			latestCount++
			assert.Equal(t, "2.0.0", version.Version,
				"Only version 2.0.0 should be marked as latest")
		}
	}
	assert.Equal(t, 1, latestCount, "Exactly one version should be marked as latest")
}

func TestPublishNewServerGetsNewID(t *testing.T) {
	// Create in-memory database and service
	db := database.NewMemoryDB()
	cfg := &config.Config{
		EnableRegistryValidation: false,
	}
	svc := NewRegistryService(db, cfg)

	// Create first server
	server1 := apiv0.ServerJSON{
		Schema:      "https://static.modelcontextprotocol.io/schemas/2025-07-09/server.schema.json",
		Name:        "com.example/server-one",
		Description: "First test server",
		Status:      "active",
		Version:     "1.0.0",
		Repository: model.Repository{
			URL:    "https://github.com/test/repo1",
			Source: "github",
		},
		Remotes: []model.Transport{
			{
				Type: "streamable-http",
				URL:  "https://example.com/server1/mcp",
			},
		},
	}

	// Create second server with different name
	server2 := apiv0.ServerJSON{
		Schema:      "https://static.modelcontextprotocol.io/schemas/2025-07-09/server.schema.json",
		Name:        "com.example/server-two",
		Description: "Second test server",
		Status:      "active",
		Version:     "1.0.0",
		Repository: model.Repository{
			URL:    "https://github.com/test/repo2",
			Source: "github",
		},
		Remotes: []model.Transport{
			{
				Type: "streamable-http",
				URL:  "https://example.com/server2/mcp",
			},
		},
	}

	// Publish both servers
	published1, err := svc.Publish(server1)
	require.NoError(t, err, "Failed to publish first server")
	require.NotNil(t, published1.Meta)
	require.NotNil(t, published1.Meta.Official)

	published2, err := svc.Publish(server2)
	require.NoError(t, err, "Failed to publish second server")
	require.NotNil(t, published2.Meta)
	require.NotNil(t, published2.Meta.Official)

	// Verify that different servers get different IDs
	assert.NotEqual(t, published1.Meta.Official.ID, published2.Meta.Official.ID,
		"Different servers should have different IDs")
}
