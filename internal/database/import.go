package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/registry/internal/model"
)

// ReadSeedFile reads seed data from various sources:
// 1. Local file paths (*.json files) - expects extension wrapper format
// 2. Direct HTTP URLs to seed.json files - expects extension wrapper format  
// 3. Registry root URLs (automatically appends /v0/servers and paginates)
// Only the extension wrapper format is supported (array of ServerResponse objects)
func ReadSeedFile(ctx context.Context, path string) ([]*model.ServerRecord, error) {
	var data []byte
	var err error

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// Handle HTTP URLs
		if strings.HasSuffix(path, "/v0/servers") || strings.Contains(path, "/v0/servers") {
			// This is a registry API endpoint - fetch paginated data
			return fetchFromRegistryAPI(ctx, path)
		}
		// This is a direct file URL
		data, err = fetchFromHTTP(ctx, path)
	} else {
		// Handle local file paths
		data, err = os.ReadFile(path)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read seed data from %s: %w", path, err)
	}

	// Parse extension wrapper format (only supported format)
	var serverResponses []model.ServerResponse
	if err := json.Unmarshal(data, &serverResponses); err != nil {
		return nil, fmt.Errorf("failed to parse seed data as extension wrapper format: %w", err)
	}

	if len(serverResponses) == 0 {
		return []*model.ServerRecord{}, nil
	}

	// Convert ServerResponse to ServerRecord
	var records []*model.ServerRecord
	for _, response := range serverResponses {
		record := convertServerResponseToRecord(response)
		records = append(records, record)
	}

	return records, nil
}

func fetchFromHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
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

func fetchFromRegistryAPI(ctx context.Context, baseURL string) ([]*model.ServerRecord, error) {
	var allRecords []*model.ServerRecord
	cursor := ""

	for {
		url := baseURL
		if cursor != "" {
			if strings.Contains(url, "?") {
				url += "&cursor=" + cursor
			} else {
				url += "?cursor=" + cursor
			}
		}

		data, err := fetchFromHTTP(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch page from registry API: %w", err)
		}

		var response struct {
			Servers  []model.ServerResponse `json:"servers"`
			Metadata *struct {
				NextCursor string `json:"next_cursor,omitempty"`
			} `json:"metadata,omitempty"`
		}

		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("failed to parse registry API response: %w", err)
		}

		// Convert and add servers
		for _, serverResponse := range response.Servers {
			record := convertServerResponseToRecord(serverResponse)
			allRecords = append(allRecords, record)
		}

		// Check if there's a next page
		if response.Metadata == nil || response.Metadata.NextCursor == "" {
			break
		}
		cursor = response.Metadata.NextCursor
	}

	return allRecords, nil
}

func convertServerResponseToRecord(response model.ServerResponse) *model.ServerRecord {
	// Extract registry metadata from the extension
	registryExt := response.XIOModelContextProtocolRegistry
	
	// Parse timestamps
	publishedAt, _ := time.Parse(time.RFC3339, getStringFromInterface(registryExt, "published_at"))
	updatedAt, _ := time.Parse(time.RFC3339, getStringFromInterface(registryExt, "updated_at"))

	registryMetadata := model.RegistryMetadata{
		ID:          getStringFromInterface(registryExt, "id"),
		IsLatest:    getBoolFromInterface(registryExt, "is_latest"),
		PublishedAt: publishedAt,
		UpdatedAt:   updatedAt,
		ReleaseDate: getStringFromInterface(registryExt, "release_date"),
	}

	// Publisher extensions
	publisherExtensions := make(map[string]interface{})
	if response.XPublisher != nil {
		if publisherMap, ok := response.XPublisher.(map[string]interface{}); ok {
			publisherExtensions = publisherMap
		}
	}

	return &model.ServerRecord{
		ServerJSON:          response.Server,
		RegistryMetadata:    registryMetadata,
		PublisherExtensions: publisherExtensions,
	}
}

func getStringFromInterface(data interface{}, key string) string {
	if dataMap, ok := data.(map[string]interface{}); ok {
		if value, exists := dataMap[key]; exists {
			if strValue, ok := value.(string); ok {
				return strValue
			}
		}
	}
	return ""
}

func getBoolFromInterface(data interface{}, key string) bool {
	if dataMap, ok := data.(map[string]interface{}); ok {
		if value, exists := dataMap[key]; exists {
			if boolValue, ok := value.(bool); ok {
				return boolValue
			}
		}
	}
	return false
}