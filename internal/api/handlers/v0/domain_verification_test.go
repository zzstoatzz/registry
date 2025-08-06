package v0_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	v0 "github.com/modelcontextprotocol/registry/internal/api/handlers/v0"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDomainVerificationIntegration(t *testing.T) {
	// Temporarily unset the environment variable to enable domain verification
	originalEnv := os.Getenv("DISABLE_DOMAIN_VERIFICATION")
	os.Unsetenv("DISABLE_DOMAIN_VERIFICATION")
	defer func() {
		if originalEnv != "" {
			os.Setenv("DISABLE_DOMAIN_VERIFICATION", originalEnv)
		}
	}()

	// Mock services
	registry := &MockRegistryService{}
	authSvc := &MockAuthService{}

	// Create a memory database for testing
	memDB := database.NewMemoryDB(make(map[string]*model.Server))
	defer memDB.Close()

	// Mock GetDatabase to return our test database
	registry.Mock.On("GetDatabase").Return(memDB)

	// Create handler
	handler := v0.PublishHandler(registry, authSvc)

	t.Run("domain verification fails for unverified domain", func(t *testing.T) {
		// Use a real domain that won't have our verification tokens
		serverDetail := model.ServerDetail{
			Server: model.Server{
				ID:          "test-id",
				Name:        "com.unverified-test-domain-12345/test-server",
				Description: "A test server",
				Repository: model.Repository{
					URL:    "https://github.com/example/test-server",
					Source: "github",
					ID:     "example/test-server",
				},
				VersionDetail: model.VersionDetail{
					Version:     "1.0.0",
					ReleaseDate: "2025-05-25T00:00:00Z",
					IsLatest:    true,
				},
			},
		}

		// Prepare request
		jsonBytes, err := json.Marshal(serverDetail)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(jsonBytes))
		req.Header.Set("Authorization", "Bearer test_token")
		req.Header.Set("Content-Type", "application/json")

		// Mock auth service to succeed (we want to test domain verification, not auth)
		authSvc.Mock.On("ValidateAuth", mock.Anything, mock.Anything).Return(true, nil)

		// We don't expect registry.Publish to be called because domain verification should fail first

		// Execute request
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify response
		assert.Equal(t, http.StatusForbidden, w.Code)

		// The current implementation returns a simple error message
		// when domain verification data is not found in the database
		expectedError := "Failed to retrieve domain verification from database"
		assert.Contains(t, w.Body.String(), expectedError)
	})

	t.Run("test domains are bypassed", func(t *testing.T) {
		// Use a .test domain which should be bypassed
		serverDetail := model.ServerDetail{
			Server: model.Server{
				ID:          "test-id",
				Name:        "com.example.test/test-server",
				Description: "A test server",
				Repository: model.Repository{
					URL:    "https://github.com/example/test-server",
					Source: "github",
					ID:     "example/test-server",
				},
				VersionDetail: model.VersionDetail{
					Version:     "1.0.0",
					ReleaseDate: "2025-05-25T00:00:00Z",
					IsLatest:    true,
				},
			},
		}

		// Prepare request
		jsonBytes, err := json.Marshal(serverDetail)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(jsonBytes))
		req.Header.Set("Authorization", "Bearer test_token")
		req.Header.Set("Content-Type", "application/json")

		// Mock auth service and registry service to succeed
		authSvc.Mock.On("ValidateAuth", mock.Anything, mock.Anything).Return(true, nil)
		registry.Mock.On("Publish", mock.AnythingOfType("*model.ServerDetail")).Return(nil)

		// Execute request
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify response - should succeed because .test domains are bypassed
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Server publication successful", response["message"])
	})

	t.Run("github.io domains are bypassed", func(t *testing.T) {
		// Use a .github.io domain which should be bypassed
		serverDetail := model.ServerDetail{
			Server: model.Server{
				ID:          "test-id",
				Name:        "io.github.testuser/test-server",
				Description: "A test server",
				Repository: model.Repository{
					URL:    "https://github.com/testuser/test-server",
					Source: "github",
					ID:     "testuser/test-server",
				},
				VersionDetail: model.VersionDetail{
					Version:     "1.0.0",
					ReleaseDate: "2025-05-25T00:00:00Z",
					IsLatest:    true,
				},
			},
		}

		// Prepare request
		jsonBytes, err := json.Marshal(serverDetail)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(jsonBytes))
		req.Header.Set("Authorization", "Bearer test_token")
		req.Header.Set("Content-Type", "application/json")

		// Mock auth service and registry service to succeed
		authSvc.Mock.On("ValidateAuth", mock.Anything, mock.Anything).Return(true, nil)
		registry.Mock.On("Publish", mock.AnythingOfType("*model.ServerDetail")).Return(nil)

		// Execute request
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify response - should succeed because .github.io domains are bypassed
		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Server publication successful", response["message"])
	})
}
