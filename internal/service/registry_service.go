package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
)

// registryServiceImpl implements the RegistryService interface using our Database
type registryServiceImpl struct {
	db database.Database
}

// NewRegistryServiceWithDB creates a new registry service with the provided database
//
//nolint:ireturn // Factory function intentionally returns interface for dependency injection
func NewRegistryServiceWithDB(db database.Database) RegistryService {
	return &registryServiceImpl{
		db: db,
	}
}

// GetAll returns all registry entries
func (s *registryServiceImpl) GetAll() ([]model.Server, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the database's List method with no filters to get all entries
	entries, _, err := s.db.List(ctx, nil, "", 30)
	if err != nil {
		return nil, err
	}

	// Convert from []*model.Server to []model.Server
	result := make([]model.Server, len(entries))
	for i, entry := range entries {
		result[i] = *entry
	}

	return result, nil
}

// List returns registry entries with cursor-based pagination
func (s *registryServiceImpl) List(cursor string, limit int) ([]model.Server, string, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If limit is not set or negative, use a default limit
	if limit <= 0 {
		limit = 30
	}

	// Use the database's List method with pagination
	entries, nextCursor, err := s.db.List(ctx, nil, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	// Convert from []*model.Server to []model.Server
	result := make([]model.Server, len(entries))
	for i, entry := range entries {
		result[i] = *entry
	}

	return result, nextCursor, nil
}

// GetByID retrieves a specific server detail by its ID
func (s *registryServiceImpl) GetByID(id string) (*model.ServerDetail, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the database's GetByID method to retrieve the server detail
	serverDetail, err := s.db.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return serverDetail, nil
}

// validateMCPBPackage validates MCPB packages to ensure they meet requirements
func validateMCPBPackage(pkg *model.Package) error {
	// Validate that the URL is from an allowlisted host
	parsedURL, err := url.Parse(pkg.Name)
	if err != nil {
		return fmt.Errorf("invalid MCPB package URL: %w", err)
	}

	// Allowlist of trusted hosts for MCPB packages
	allowedHosts := []string{
		"github.com",
		"www.github.com",
		"raw.githubusercontent.com",
		"gitlab.com",
		"www.gitlab.com",
	}

	host := strings.ToLower(parsedURL.Host)
	isAllowed := false
	for _, allowed := range allowedHosts {
		if host == allowed {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		return fmt.Errorf("MCPB packages must be hosted on allowlisted providers (GitHub or GitLab). Host '%s' is not allowed", host)
	}

	// Validate that file_hashes is provided for MCPB packages
	if len(pkg.FileHashes) == 0 {
		return fmt.Errorf("MCPB packages must include file_hashes for integrity verification")
	}

	// Validate that at least SHA-256 is provided
	if _, hasSHA256 := pkg.FileHashes["sha-256"]; !hasSHA256 {
		if _, hasSHA256Alt := pkg.FileHashes["sha256"]; !hasSHA256Alt {
			return fmt.Errorf("MCPB packages must include a SHA-256 hash")
		}
	}

	return nil
}

// Publish adds a new server detail to the registry
func (s *registryServiceImpl) Publish(serverDetail *model.ServerDetail) error {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if serverDetail == nil {
		return database.ErrInvalidInput
	}

	// Validate MCPB packages
	for _, pkg := range serverDetail.Packages {
		if strings.ToLower(pkg.RegistryName) == "mcpb" {
			if err := validateMCPBPackage(&pkg); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
		}
	}

	err := s.db.Publish(ctx, serverDetail)
	if err != nil {
		return err
	}

	return nil
}
