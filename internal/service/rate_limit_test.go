package service

import (
	"testing"

	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/database"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestPublishRateLimit(t *testing.T) {
	memDB := database.NewMemoryDB()
	cfg := &config.Config{
		EnableRegistryValidation: false,
		PublishLimitPerDay:       2, // Set low limit for testing
	}
	service := NewRegistryService(memDB, cfg)

	testServer := apiv0.ServerJSON{
		Name:        "io.github.testuser/test-server",
		Description: "A test server",
		VersionDetail: model.VersionDetail{
			Version: "1.0.0",
		},
	}

	// First publish should work
	_, err := service.Publish(testServer, "testuser", false)
	assert.NoError(t, err, "First publish should succeed")

	// Second publish should work (different version)
	testServer.VersionDetail.Version = "1.0.1" 
	_, err = service.Publish(testServer, "testuser", false)
	assert.NoError(t, err, "Second publish should succeed")

	// Third publish should fail due to rate limit
	testServer.VersionDetail.Version = "1.0.2"
	_, err = service.Publish(testServer, "testuser", false)
	assert.Error(t, err, "Third publish should fail due to rate limit")
	assert.Contains(t, err.Error(), "publish rate limit exceeded", "Error should mention rate limit")

	// Admin with global permissions should bypass rate limit
	testServer.VersionDetail.Version = "1.0.3"
	_, err = service.Publish(testServer, "testuser", true)
	assert.NoError(t, err, "Admin publish should succeed despite rate limit")

	// Different user should be able to publish
	testServer.Name = "io.github.otheruser/test-server"
	testServer.VersionDetail.Version = "1.0.0"
	_, err = service.Publish(testServer, "otheruser", false)
	assert.NoError(t, err, "Different user should be able to publish")
}

func TestPublishRateLimitDisabled(t *testing.T) {
	memDB := database.NewMemoryDB()
	cfg := &config.Config{
		EnableRegistryValidation: false,
		PublishLimitPerDay:       -1, // Disabled
	}
	service := NewRegistryService(memDB, cfg)

	testServer := apiv0.ServerJSON{
		Name:        "io.github.testuser/test-server",
		Description: "A test server",
		VersionDetail: model.VersionDetail{
			Version: "1.0.0",
		},
	}

	// Should be able to publish many times when limit is disabled
	for i := 0; i < 5; i++ {
		testServer.VersionDetail.Version = "1.0." + string(rune(i+'0'))
		_, err := service.Publish(testServer, "testuser", false)
		assert.NoError(t, err, "Should be able to publish when rate limiting is disabled")
	}
}

func TestPublishingCompletelyDisabled(t *testing.T) {
	memDB := database.NewMemoryDB()
	cfg := &config.Config{
		EnableRegistryValidation: false,
		PublishLimitPerDay:       0, // Publishing disabled
	}
	service := NewRegistryService(memDB, cfg)

	testServer := apiv0.ServerJSON{
		Name:        "io.github.testuser/test-server",
		Description: "A test server",
		VersionDetail: model.VersionDetail{
			Version: "1.0.0",
		},
	}

	// Should not be able to publish when publishing is disabled
	_, err := service.Publish(testServer, "testuser", false)
	assert.Error(t, err, "Should not be able to publish when publishing is disabled")
	assert.Contains(t, err.Error(), "publishing is currently disabled", "Error should mention publishing is disabled")

	// Admin should still be able to publish
	_, err = service.Publish(testServer, "testuser", true)
	assert.NoError(t, err, "Admin should be able to publish even when publishing is disabled")
}