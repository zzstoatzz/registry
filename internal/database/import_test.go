package database_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestReadSeedFile_LocalFile(t *testing.T) {
	// Create a temporary seed file in extension wrapper format
	tempFile := "/tmp/test_seed.json"
	seedData := []model.ServerResponse{
		{
			Server: model.ServerDetail{
				Name:        "test-server-1",
				Description: "Test server 1",
				Repository: model.Repository{
					URL:    "https://github.com/test/repo1",
					Source: "github",
					ID:     "123",
				},
				VersionDetail: model.VersionDetail{
					Version: "1.0.0",
				},
			},
			XIOModelContextProtocolRegistry: map[string]interface{}{
				"id":           "test-id-1",
				"published_at": "2023-01-01T00:00:00Z",
				"is_latest":    true,
			},
		},
	}

	// Write seed data to temp file
	data, err := json.Marshal(seedData)
	assert.NoError(t, err)

	err = func() error {
		f, err := os.Create(tempFile)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(data)
		return err
	}()
	assert.NoError(t, err)
	defer os.Remove(tempFile)

	// Test reading the file
	result, err := database.ReadSeedFile(context.Background(), tempFile)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "test-server-1", result[0].ServerJSON.Name)
}

func TestReadSeedFile_DirectHTTPURL(t *testing.T) {
	// Create a test HTTP server that serves seed JSON directly in extension wrapper format
	seedData := []model.ServerResponse{
		{
			Server: model.ServerDetail{
				Name:        "test-server-1",
				Description: "Test server 1",
			},
			XIOModelContextProtocolRegistry: map[string]interface{}{
				"id":           "test-id-1",
				"published_at": "2023-01-01T00:00:00Z",
				"is_latest":    true,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(seedData); err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Test reading from HTTP URL ending in .json
	result, err := database.ReadSeedFile(context.Background(), server.URL+"/seed.json")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "test-server-1", result[0].ServerJSON.Name)
}

func TestReadSeedFile_RegistryURL(t *testing.T) {
	// Create mock registry responses
	server1 := model.ServerResponse{
		Server: model.ServerDetail{
			Name:        "Test Server 1",
			Description: "First test server",
			Packages: []model.Package{
				{
					PackageType: "javascript",
					Registry:    "npm",
					Identifier:  "test-package-1",
					Version:     "1.0.0",
				},
			},
		},
		XIOModelContextProtocolRegistry: map[string]interface{}{
			"id":           "server-1",
			"published_at": "2023-01-01T00:00:00Z",
			"is_latest":    true,
		},
	}
	server2 := model.ServerResponse{
		Server: model.ServerDetail{
			Name:        "Test Server 2",
			Description: "Second test server",
			Packages: []model.Package{
				{
					PackageType: "javascript",
					Registry:    "npm",
					Identifier:  "test-package-2",
					Version:     "2.0.0",
				},
			},
		},
		XIOModelContextProtocolRegistry: map[string]interface{}{
			"id":           "server-2",
			"published_at": "2023-01-01T00:00:00Z",
			"is_latest":    true,
		},
	}

	// Create a test HTTP server that simulates the registry API
	mux := http.NewServeMux()

	// Handle /v0/servers endpoint (paginated)
	mux.HandleFunc("/v0/servers", func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")

		type Metadata struct {
			NextCursor string `json:"next_cursor,omitempty"`
			Count      int    `json:"count,omitempty"`
		}

		type PaginatedResponse struct {
			Servers  []model.ServerResponse `json:"servers"`
			Metadata *Metadata             `json:"metadata,omitempty"`
		}

		var response PaginatedResponse
		switch cursor {
		case "":
			// First page
			response = PaginatedResponse{
				Servers: []model.ServerResponse{server1},
				Metadata: &Metadata{
					NextCursor: "next-cursor-1",
					Count:      1,
				},
			}
		case "next-cursor-1":
			// Second page
			response = PaginatedResponse{
				Servers: []model.ServerResponse{server2},
				Metadata: &Metadata{
					Count: 1,
					// No NextCursor means end of pagination
				},
			}
		default:
			// No more pages
			response = PaginatedResponse{
				Servers:  []model.ServerResponse{},
				Metadata: &Metadata{},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test reading from registry root URL (this should trigger pagination)
	result, err := database.ReadSeedFile(context.Background(), server.URL+"/v0/servers")
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	// Verify the servers were imported correctly
	assert.Equal(t, "Test Server 1", result[0].ServerJSON.Name)
	assert.Equal(t, "Test Server 2", result[1].ServerJSON.Name)

	// Verify packages were included
	assert.Len(t, result[0].ServerJSON.Packages, 1)
	assert.Equal(t, "javascript", result[0].ServerJSON.Packages[0].PackageType)
	assert.Equal(t, "npm", result[0].ServerJSON.Packages[0].Registry)
	assert.Equal(t, "test-package-1", result[0].ServerJSON.Packages[0].Identifier)
	assert.Len(t, result[1].ServerJSON.Packages, 1)
	assert.Equal(t, "javascript", result[1].ServerJSON.Packages[0].PackageType)
	assert.Equal(t, "npm", result[1].ServerJSON.Packages[0].Registry)
	assert.Equal(t, "test-package-2", result[1].ServerJSON.Packages[0].Identifier)

	// Verify metadata was extracted
	assert.Equal(t, "server-1", result[0].RegistryMetadata.ID)
	assert.Equal(t, "server-2", result[1].RegistryMetadata.ID)
}
