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

// validateJavaScriptPackage validates JavaScript/NPM packages
func validateJavaScriptPackage(host string, scheme string) error {
	if !strings.Contains(host, "npmjs.com") && !strings.HasPrefix(scheme, "npm") {
		return fmt.Errorf("javascript packages must be from npmjs.com or use npm:// scheme")
	}
	return nil
}

// validatePythonPackage validates Python/PyPI packages
func validatePythonPackage(host string, scheme string) error {
	if !strings.Contains(host, "pypi.org") && !strings.HasPrefix(scheme, "pypi") {
		return fmt.Errorf("python packages must be from pypi.org or use pypi:// scheme")
	}
	return nil
}

// validateDockerPackage validates Docker images
func validateDockerPackage(host string, scheme string) error {
	if strings.HasPrefix(scheme, "docker") {
		return nil
	}
	
	knownRegistries := []string{"docker.io", "ghcr.io", "quay.io"}
	for _, registry := range knownRegistries {
		if strings.Contains(host, registry) {
			return nil
		}
	}
	
	// Allow any host for docker images as they can be from private registries
	// Just validate it has a proper scheme
	if scheme != "https" && scheme != "http" {
		return fmt.Errorf("docker images must use docker:// scheme or be from a valid registry")
	}
	return nil
}

// validateMCPBPackage validates MCPB packages
func validateMCPBPackage(host string, pkg *model.Package) error {
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
	// Validate that the URL is properly formatted
	parsedURL, err := url.Parse(pkg.Location.URL)
	if err != nil {
		return fmt.Errorf("invalid package URL: %w", err)
	}

	host := strings.ToLower(parsedURL.Host)
	packageType := strings.ToLower(pkg.Location.Type)

	// Validate based on package type
	switch packageType {
	case "javascript":
		return validateJavaScriptPackage(host, parsedURL.Scheme)
	case "python":
		return validatePythonPackage(host, parsedURL.Scheme)
	case "docker":
		return validateDockerPackage(host, parsedURL.Scheme)
	case "mcpb":
		return validateMCPBPackage(host, pkg)
	default:
		// For unknown types, just ensure it's a valid URL
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return fmt.Errorf("package URL must be a valid absolute URL")
		}
	}

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