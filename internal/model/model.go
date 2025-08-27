package model

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// AuthMethod represents the authentication method used
type AuthMethod string

const (
	// GitHub OAuth authentication (access token)
	AuthMethodGitHubAT AuthMethod = "github-at"
	// GitHub Actions OIDC authentication
	AuthMethodGitHubOIDC AuthMethod = "github-oidc"
	// DNS-based public/private key authentication
	AuthMethodDNS AuthMethod = "dns"
	// HTTP-based public/private key authentication
	AuthMethodHTTP AuthMethod = "http"
	// No authentication - should only be used for local development and testing
	AuthMethodNone AuthMethod = "none"
)

// ServerStatus represents the lifecycle status of a server
type ServerStatus string

const (
	// ServerStatusActive represents an actively maintained server (as asserted by the publisher)
	ServerStatusActive ServerStatus = "active"
	// ServerStatusDeprecated represents a server that is no longer actively maintained
	ServerStatusDeprecated ServerStatus = "deprecated"
)

// Repository represents a source code repository as defined in the spec
type Repository struct {
	URL    string `json:"url" bson:"url"`
	Source string `json:"source" bson:"source"`
	ID     string `json:"id,omitempty" bson:"id,omitempty"`
}

// create an enum for Format
type Format string

const (
	FormatString   Format = "string"
	FormatNumber   Format = "number"
	FormatBoolean  Format = "boolean"
	FormatFilePath Format = "file_path"
)

type Input struct {
	Description string   `json:"description,omitempty" bson:"description,omitempty"`
	IsRequired  bool     `json:"is_required,omitempty" bson:"is_required,omitempty"`
	Format      Format   `json:"format,omitempty" bson:"format,omitempty"`
	Value       string   `json:"value,omitempty" bson:"value,omitempty"`
	IsSecret    bool     `json:"is_secret,omitempty" bson:"is_secret,omitempty"`
	Default     string   `json:"default,omitempty" bson:"default,omitempty"`
	Choices     []string `json:"choices,omitempty" bson:"choices,omitempty"`
}

type InputWithVariables struct {
	Input     `json:",inline" bson:",inline"`
	Variables map[string]Input `json:"variables,omitempty" bson:"variables,omitempty"`
}

type KeyValueInput struct {
	InputWithVariables `json:",inline" bson:",inline"`
	Name               string `json:"name" bson:"name"`
}
type ArgumentType string

const (
	ArgumentTypePositional ArgumentType = "positional"
	ArgumentTypeNamed      ArgumentType = "named"
)

// RuntimeArgument defines a type that can be either a PositionalArgument or a NamedArgument
type Argument struct {
	InputWithVariables `json:",inline" bson:",inline"`
	Type               ArgumentType `json:"type" bson:"type"`
	Name               string       `json:"name,omitempty" bson:"name,omitempty"`
	IsRepeated         bool         `json:"is_repeated,omitempty" bson:"is_repeated,omitempty"`
	ValueHint          string       `json:"value_hint,omitempty" bson:"value_hint,omitempty"`
}

type Package struct {
	// RegistryType indicates how to download packages (e.g., "npm", "pypi", "docker-hub", "github-releases")
	RegistryType         string            `json:"registry_type,omitempty" bson:"registry_type,omitempty"`
	// RegistryBaseURL is the base URL of the package registry
	RegistryBaseURL      string            `json:"registry_base_url,omitempty" bson:"registry_base_url,omitempty"`
	// Identifier is the package identifier - either a package name (for registries) or URL (for direct downloads)
	Identifier           string            `json:"identifier,omitempty" bson:"identifier,omitempty"`
	Version              string            `json:"version,omitempty" bson:"version,omitempty"`
	FileHashes           map[string]string `json:"file_hashes,omitempty" bson:"file_hashes,omitempty"`
	RunTimeHint          string            `json:"runtime_hint,omitempty" bson:"runtime_hint,omitempty"`
	RuntimeArguments     []Argument        `json:"runtime_arguments,omitempty" bson:"runtime_arguments,omitempty"`
	PackageArguments     []Argument        `json:"package_arguments,omitempty" bson:"package_arguments,omitempty"`
	EnvironmentVariables []KeyValueInput   `json:"environment_variables,omitempty" bson:"environment_variables,omitempty"`
}

// Remote represents a remote connection endpoint
type Remote struct {
	TransportType string          `json:"transport_type" bson:"transport_type"`
	URL           string          `json:"url" format:"uri" bson:"url"`
	Headers       []KeyValueInput `json:"headers,omitempty" bson:"headers,omitempty"`
}

// VersionDetail represents the version details of a server (pure MCP spec, no registry metadata)
type VersionDetail struct {
	Version string `json:"version" bson:"version"`
}

// ServerDetail represents complete server information as defined in the MCP spec (pure, no registry metadata)
type ServerDetail struct {
	Schema        string        `json:"$schema,omitempty" bson:"$schema,omitempty"`
	Name          string        `json:"name" minLength:"1" maxLength:"200" bson:"name"`
	Description   string        `json:"description" minLength:"1" maxLength:"100" bson:"description"`
	Status        ServerStatus  `json:"status,omitempty" minLength:"1" bson:"status,omitempty"`
	Repository    Repository    `json:"repository,omitempty" bson:"repository"`
	VersionDetail VersionDetail `json:"version_detail" bson:"version_detail"`
	Packages      []Package     `json:"packages,omitempty" bson:"packages,omitempty"`
	Remotes       []Remote      `json:"remotes,omitempty" bson:"remotes,omitempty"`
}

// RegistryMetadata represents registry-generated metadata
type RegistryMetadata struct {
	ID          string    `json:"id" bson:"_id"`
	PublishedAt time.Time `json:"published_at" bson:"published_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	IsLatest    bool      `json:"is_latest" bson:"is_latest"`
	ReleaseDate string    `json:"release_date" bson:"release_date"`
}

// ServerRecord represents the complete storage model that separates server.json from registry metadata
type ServerRecord struct {
	ServerJSON          ServerDetail           `json:"server" bson:"server"`                             // Pure MCP server.json
	RegistryMetadata    RegistryMetadata       `json:"registry_metadata" bson:"registry_metadata"`       // Registry-generated data
	PublisherExtensions map[string]interface{} `json:"publisher_extensions" bson:"publisher_extensions"` // x-publisher extensions
}

// ServerResponse represents the API response format with wrapper and extensions
type ServerResponse struct {
	Server                          ServerDetail `json:"server"`
	XIOModelContextProtocolRegistry interface{}  `json:"x-io.modelcontextprotocol.registry,omitempty"`
	XPublisher                      interface{}  `json:"x-publisher,omitempty"`
}

// ServerListResponse represents the paginated server list response
type ServerListResponse struct {
	Servers  []ServerResponse `json:"servers"`
	Metadata *Metadata        `json:"metadata,omitempty"`
}

// PublishRequest represents the API request format for publishing servers
type PublishRequest struct {
	Server     ServerDetail `json:"server"`
	XPublisher interface{}  `json:"x-publisher,omitempty"`
}

// Metadata represents pagination metadata
type Metadata struct {
	NextCursor string `json:"next_cursor,omitempty"`
	Count      int    `json:"count,omitempty"`
	Total      int    `json:"total,omitempty"`
}

// Helper functions

// ValidatePublisherExtensions validates that publisher extensions are within size limits
func ValidatePublisherExtensions(req PublishRequest) error {
	const maxExtensionSize = 4 * 1024 // 4KB limit

	// Check size limit for x-publisher extension
	if req.XPublisher != nil {
		extensionsJSON, err := json.Marshal(req.XPublisher)
		if err != nil {
			return fmt.Errorf("failed to marshal x-publisher extension: %w", err)
		}
		if len(extensionsJSON) > maxExtensionSize {
			return fmt.Errorf("x-publisher extension exceeds 4KB limit (%d bytes)", len(extensionsJSON))
		}
	}

	return nil
}

// ValidatePublishRequestExtensions validates that only allowed extension fields are present
func ValidatePublishRequestExtensions(requestData []byte) error {
	// Parse the raw JSON to check for unknown fields
	var rawRequest map[string]interface{}
	if err := json.Unmarshal(requestData, &rawRequest); err != nil {
		return fmt.Errorf("failed to parse request JSON: %w", err)
	}

	// Define allowed top-level fields
	allowedFields := map[string]bool{
		"server":      true,
		"x-publisher": true,
	}

	// Check for any disallowed fields
	var invalidFields []string
	for field := range rawRequest {
		if !allowedFields[field] {
			invalidFields = append(invalidFields, field)
		}
	}

	if len(invalidFields) > 0 {
		return fmt.Errorf("invalid extension fields: %v. Only 'server' and 'x-publisher' fields are allowed", invalidFields)
	}

	return nil
}

// ExtractPublisherExtensions extracts publisher extensions from a PublishRequest
func ExtractPublisherExtensions(req PublishRequest) map[string]interface{} {
	publisherExtensions := make(map[string]interface{})
	if req.XPublisher != nil {
		// Cast to map and copy fields directly, avoiding double nesting
		if publisherMap, ok := req.XPublisher.(map[string]interface{}); ok {
			for k, v := range publisherMap {
				publisherExtensions[k] = v
			}
		}
	}
	return publisherExtensions
}

// CreateRegistryExtensions generates the x-io.modelcontextprotocol.registry extension from registry metadata
func (rm *RegistryMetadata) CreateRegistryExtensions() map[string]interface{} {
	return map[string]interface{}{
		"x-io.modelcontextprotocol.registry": map[string]interface{}{
			"id":           rm.ID,
			"published_at": rm.PublishedAt,
			"updated_at":   rm.UpdatedAt,
			"is_latest":    rm.IsLatest,
			"release_date": rm.ReleaseDate,
		},
	}
}

// ParseServerName extracts the server name from a ServerDetail for validation purposes
func ParseServerName(serverDetail ServerDetail) (string, error) {
	name := serverDetail.Name
	if name == "" {
		return "", fmt.Errorf("server name is required and must be a string")
	}

	// Validate format: dns-namespace/name
	if !strings.Contains(name, "/") {
		return "", fmt.Errorf("server name must be in format 'dns-namespace/name' (e.g., 'com.example.api/server')")
	}

	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("server name must be in format 'dns-namespace/name' with non-empty namespace and name parts")
	}

	return name, nil
}

// ValidateRemoteNamespaceMatch validates that remote URLs match the reverse-DNS namespace
func ValidateRemoteNamespaceMatch(serverDetail ServerDetail) error {
	namespace := serverDetail.Name

	for _, remote := range serverDetail.Remotes {
		if err := validateRemoteURLMatchesNamespace(remote.URL, namespace); err != nil {
			return fmt.Errorf("remote URL %s does not match namespace %s: %w", remote.URL, namespace, err)
		}
	}

	return nil
}

// validateRemoteURLMatchesNamespace checks if a remote URL's hostname matches the publisher domain from the namespace
func validateRemoteURLMatchesNamespace(remoteURL, namespace string) error {
	// Parse the URL to extract the hostname
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	// Skip validation for localhost and local development URLs
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") || hostname == "127.0.0.1" {
		return nil
	}

	// Extract publisher domain from reverse-DNS namespace
	publisherDomain := extractPublisherDomainFromNamespace(namespace)
	if publisherDomain == "" {
		return fmt.Errorf("invalid namespace format: cannot extract domain from %s", namespace)
	}

	// Check if the remote URL hostname matches the publisher domain or is a subdomain
	if !isValidHostForDomain(hostname, publisherDomain) {
		return fmt.Errorf("remote URL host %s does not match publisher domain %s", hostname, publisherDomain)
	}

	return nil
}

// extractPublisherDomainFromNamespace converts reverse-DNS namespace to normal domain format
// e.g., "com.example" -> "example.com"
func extractPublisherDomainFromNamespace(namespace string) string {
	// Extract the namespace part before the first slash
	namespacePart := namespace
	if slashIdx := strings.Index(namespace, "/"); slashIdx != -1 {
		namespacePart = namespace[:slashIdx]
	}

	// Split into parts and reverse them to get normal domain format
	parts := strings.Split(namespacePart, ".")
	if len(parts) < 2 {
		return ""
	}

	// Reverse the parts to convert from reverse-DNS to normal domain
	slices.Reverse(parts)

	return strings.Join(parts, ".")
}

// isValidHostForDomain checks if a hostname is the domain or a subdomain of the publisher domain
func isValidHostForDomain(hostname, publisherDomain string) bool {
	// Exact match
	if hostname == publisherDomain {
		return true
	}

	// Subdomain match - hostname should end with "." + publisherDomain
	if strings.HasSuffix(hostname, "."+publisherDomain) {
		return true
	}

	return false
}

// ToServerResponse converts a ServerRecord to API response format
func (sr *ServerRecord) ToServerResponse() ServerResponse {
	response := ServerResponse{
		Server: sr.ServerJSON,
	}

	// Add registry metadata extension
	response.XIOModelContextProtocolRegistry = sr.RegistryMetadata.CreateRegistryExtensions()["x-io.modelcontextprotocol.registry"]

	// Add publisher extensions directly
	if len(sr.PublisherExtensions) > 0 {
		response.XPublisher = sr.PublisherExtensions
	}

	return response
}
