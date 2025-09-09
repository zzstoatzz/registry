package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/validators"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
)

const maxServerVersionsPerServer = 10000

// registryServiceImpl implements the RegistryService interface using our Database
type registryServiceImpl struct {
	db  database.Database
	cfg *config.Config
}

// NewRegistryService creates a new registry service with the provided database
func NewRegistryService(db database.Database, cfg *config.Config) RegistryService {
	return &registryServiceImpl{
		db:  db,
		cfg: cfg,
	}
}

// List returns registry entries with cursor-based pagination and optional filtering
func (s *registryServiceImpl) List(filter *database.ServerFilter, cursor string, limit int) ([]apiv0.ServerJSON, string, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If limit is not set or negative, use a default limit
	if limit <= 0 {
		limit = 30
	}

	// Use the database's ListServers method with pagination and filtering
	serverRecords, nextCursor, err := s.db.List(ctx, filter, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	// Return ServerJSONs directly
	result := make([]apiv0.ServerJSON, len(serverRecords))
	for i, record := range serverRecords {
		result[i] = *record
	}

	return result, nextCursor, nil
}

// GetByID retrieves a specific server by its registry metadata ID in flattened format
func (s *registryServiceImpl) GetByID(id string) (*apiv0.ServerJSON, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverRecord, err := s.db.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Return the server record directly
	return serverRecord, nil
}

// Publish publishes a server with flattened _meta extensions
func (s *registryServiceImpl) Publish(req apiv0.ServerJSON) (*apiv0.ServerJSON, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate the request
	if err := validators.ValidatePublishRequest(req, s.cfg); err != nil {
		return nil, err
	}

	publishTime := time.Now()
	serverJSON := req

	// Check for duplicate remote URLs
	if err := s.validateNoDuplicateRemoteURLs(ctx, serverJSON); err != nil {
		return nil, err
	}

	filter := &database.ServerFilter{Name: &serverJSON.Name}
	existingServerVersions, _, err := s.db.List(ctx, filter, "", maxServerVersionsPerServer)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}

	// Validate version constraints
	if err := s.validateVersionConstraints(existingServerVersions, serverJSON.Version); err != nil {
		return nil, err
	}

	// Determine if this version should be marked as latest
	existingLatest := s.getCurrentLatestVersion(existingServerVersions)
	isNewLatest := true
	if existingLatest != nil {
		var existingPublishedAt time.Time
		if existingLatest.Meta != nil && existingLatest.Meta.Official != nil {
			existingPublishedAt = existingLatest.Meta.Official.PublishedAt
		}
		isNewLatest = CompareVersions(
			serverJSON.Version,
			existingLatest.Version,
			publishTime,
			existingPublishedAt,
		) > 0
	}

	// Create complete server with metadata
	server := serverJSON // Copy the input

	// Initialize meta if not present
	if server.Meta == nil {
		server.Meta = &apiv0.ServerMeta{}
	}

	// Determine the ID to use - reuse from first version or generate new
	serverID := s.determineServerID(existingServerVersions)

	// Set registry metadata
	server.Meta.Official = &apiv0.RegistryExtensions{
		ID:          serverID,
		PublishedAt: publishTime,
		UpdatedAt:   publishTime,
		IsLatest:    isNewLatest,
	}

	// Create server in database
	serverRecord, err := s.db.CreateServer(ctx, &server)
	if err != nil {
		return nil, err
	}

	// Mark previous latest as no longer latest
	if isNewLatest && existingLatest != nil {
		if err := s.markAsNotLatest(ctx, existingLatest); err != nil {
			return nil, err
		}
	}

	// Return the server record directly
	return serverRecord, nil
}

// validateNoDuplicateRemoteURLs checks that no other server is using the same remote URLs
func (s *registryServiceImpl) validateNoDuplicateRemoteURLs(ctx context.Context, serverDetail apiv0.ServerJSON) error {
	// Check each remote URL in the new server for conflicts
	for _, remote := range serverDetail.Remotes {
		// Use filter to find servers with this remote URL
		filter := &database.ServerFilter{RemoteURL: &remote.URL}

		conflictingServers, _, err := s.db.List(ctx, filter, "", 1000)
		if err != nil {
			return fmt.Errorf("failed to check remote URL conflict: %w", err)
		}

		// Check if any conflicting server has a different name
		for _, conflictingServer := range conflictingServers {
			if conflictingServer.Name != serverDetail.Name {
				return fmt.Errorf("remote URL %s is already used by server %s", remote.URL, conflictingServer.Name)
			}
		}
	}

	return nil
}

// getCurrentLatestVersion finds the current latest version from existing server versions
func (s *registryServiceImpl) getCurrentLatestVersion(existingServerVersions []*apiv0.ServerJSON) *apiv0.ServerJSON {
	for _, server := range existingServerVersions {
		if server.Meta != nil && server.Meta.Official != nil &&
			server.Meta.Official.IsLatest {
			return server
		}
	}
	return nil
}

// determineServerID determines the server ID to use for a new version
// It reuses the ID from the earliest version if one exists, otherwise generates a new one
func (s *registryServiceImpl) determineServerID(existingServerVersions []*apiv0.ServerJSON) string {
	if len(existingServerVersions) > 0 {
		// Find the earliest published version to get the consistent ID
		var earliestVersion *apiv0.ServerJSON
		var earliestTime time.Time

		for _, version := range existingServerVersions {
			if version.Meta != nil && version.Meta.Official != nil {
				versionTime := version.Meta.Official.PublishedAt
				if earliestVersion == nil || versionTime.Before(earliestTime) {
					earliestVersion = version
					earliestTime = versionTime
				}
			}
		}

		if earliestVersion != nil && earliestVersion.Meta != nil && earliestVersion.Meta.Official != nil {
			return earliestVersion.Meta.Official.ID
		}
	}

	// Generate new ID only if this is the first version
	return uuid.New().String()
}

// validateVersionConstraints checks version-related constraints
func (s *registryServiceImpl) validateVersionConstraints(existingServerVersions []*apiv0.ServerJSON, newVersion string) error {
	// Check we haven't exceeded the maximum versions allowed for a server
	if len(existingServerVersions) >= maxServerVersionsPerServer {
		return database.ErrMaxServersReached
	}

	// Check this isn't a duplicate version
	for _, server := range existingServerVersions {
		if server.Version == newVersion {
			return database.ErrInvalidVersion
		}
	}

	return nil
}

// markAsNotLatest marks a server version as no longer the latest
func (s *registryServiceImpl) markAsNotLatest(ctx context.Context, server *apiv0.ServerJSON) error {
	if server.Meta == nil || server.Meta.Official == nil {
		return nil
	}

	serverID := server.Meta.Official.ID
	if serverID == "" {
		return nil
	}

	// Create a copy of the server to avoid modifying the original
	serverCopy := *server
	if serverCopy.Meta == nil {
		serverCopy.Meta = &apiv0.ServerMeta{}
	}
	if serverCopy.Meta.Official == nil {
		serverCopy.Meta.Official = &apiv0.RegistryExtensions{}
	}

	// Update the copy to set is_latest = false
	serverCopy.Meta.Official.IsLatest = false
	serverCopy.Meta.Official.UpdatedAt = time.Now()
	
	// Use UpdateServer with the serverID which should match the current latest version
	_, err := s.db.UpdateServer(ctx, serverID, &serverCopy)
	return err
}

// EditServer updates an existing server with new details (admin operation)
func (s *registryServiceImpl) EditServer(id string, req apiv0.ServerJSON) (*apiv0.ServerJSON, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate the request
	if err := validators.ValidatePublishRequest(req, s.cfg); err != nil {
		return nil, err
	}

	serverJSON := req

	// Check for duplicate remote URLs
	if err := s.validateNoDuplicateRemoteURLs(ctx, serverJSON); err != nil {
		return nil, err
	}

	// Update server in database
	serverRecord, err := s.db.UpdateServer(ctx, id, &serverJSON)
	if err != nil {
		return nil, err
	}

	// Return the server record directly
	return serverRecord, nil
}
