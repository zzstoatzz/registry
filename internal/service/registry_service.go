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

// List returns registry entries with cursor-based pagination in extension wrapper format
func (s *registryServiceImpl) List(cursor string, limit int) ([]model.ServerResponse, string, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If limit is not set or negative, use a default limit
	if limit <= 0 {
		limit = 30
	}

	// Use the database's List method with pagination
	serverRecords, nextCursor, err := s.db.List(ctx, nil, cursor, limit)
	if err != nil {
		return nil, "", err
	}

	// Convert ServerRecord to ServerResponse format
	result := make([]model.ServerResponse, len(serverRecords))
	for i, record := range serverRecords {
		result[i] = record.ToServerResponse()
	}

	return result, nextCursor, nil
}

// GetByID retrieves a specific server by its registry metadata ID in extension wrapper format
func (s *registryServiceImpl) GetByID(id string) (*model.ServerResponse, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the database's GetByID method to retrieve the server record
	serverRecord, err := s.db.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Convert ServerRecord to ServerResponse format
	response := serverRecord.ToServerResponse()
	return &response, nil
}

// validateMCPBPackage validates MCPB packages
func validateMCPBPackage(host string) error {
	allowedHosts := []string{
		"github.com",
		"www.github.com",
		"gitlab.com",
		"www.gitlab.com",
	}

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
	
	return nil
}

// validatePackage validates packages to ensure they meet requirements
func validatePackage(pkg *model.Package) error {
	registry := strings.ToLower(pkg.RegistryName)
	packageType := strings.ToLower(pkg.PackageType)
	
	// For direct download packages (github-releases, gitlab-releases, or direct URLs)
	if registry == "github-releases" || registry == "gitlab-releases" || 
	   strings.HasPrefix(pkg.Identifier, "http://") || strings.HasPrefix(pkg.Identifier, "https://") {
		parsedURL, err := url.Parse(pkg.Identifier)
		if err != nil {
			return fmt.Errorf("invalid package URL: %w", err)
		}
		
		host := strings.ToLower(parsedURL.Host)
		
		// For MCPB packages, validate they're from allowed hosts
		if packageType == "mcpb" {
			return validateMCPBPackage(host)
		}
		
		// For other URL-based packages, just ensure it's valid
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("package URL must be a valid absolute URL")
		}
		return nil
	}
	
	// For registry-based packages, no special validation needed
	// Registry types like "npm", "pypi", "docker-hub", "nuget" are all valid
	return nil
}

// Publish publishes a server with separated extensions
func (s *registryServiceImpl) Publish(req model.PublishRequest) (*model.ServerResponse, error) {
	// Create a timeout context for the database operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Validate the request
	if err := model.ValidatePublisherExtensions(req); err != nil {
		return nil, err
	}

	// Validate server name exists
	if _, err := model.ParseServerName(req.Server); err != nil {
		return nil, err
	}

	// Validate all packages
	for _, pkg := range req.Server.Packages {
		if err := validatePackage(&pkg); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}

	// Extract publisher extensions from request
	publisherExtensions := model.ExtractPublisherExtensions(req)

	// Publish to database
	serverRecord, err := s.db.Publish(ctx, req.Server, publisherExtensions)
	if err != nil {
		return nil, err
	}

	// Convert ServerRecord to ServerResponse format
	response := serverRecord.ToServerResponse()
	return &response, nil
}