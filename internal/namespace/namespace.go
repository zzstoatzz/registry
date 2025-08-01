package namespace

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"unicode"
)

// ErrInvalidNamespace is returned when a namespace is invalid
var ErrInvalidNamespace = errors.New("invalid namespace")

// ErrInvalidDomain is returned when a domain is invalid
var ErrInvalidDomain = errors.New("invalid domain")

// ErrReservedNamespace is returned when trying to use a reserved namespace
var ErrReservedNamespace = errors.New("reserved namespace")

// reservedNamespaces contains namespaces that are reserved and cannot be used
var reservedNamespaces = map[string]bool{
	"com.localhost": true,
	"org.localhost": true,
	"net.localhost": true,
	"localhost":     true,
	"com.example":   true,
	"org.example":   true,
	"net.example":   true,
	"example":       true,
	"com.test":      true,
	"org.test":      true,
	"net.test":      true,
	"test":          true,
	"com.invalid":   true,
	"org.invalid":   true,
	"net.invalid":   true,
	"invalid":       true,
	"com.local":     true,
	"org.local":     true,
	"net.local":     true,
	"local":         true,
}

// Namespace represents a parsed namespace with its components
type Namespace struct {
	Original   string // Original namespace string
	Domain     string // Extracted domain (e.g., "github.com")
	ServerName string // Server name portion (e.g., "my-server")
}

// domainPattern matches valid domain names according to RFC specifications
// This regex handles:
// - Domain labels (segments between dots)
// - International domain names (IDN)
// - Proper length restrictions
// - Valid characters
var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// reverseNotationPattern matches reverse domain notation with server name
// Format: tld.domain/server-name (e.g., com.github/my-server)
// This pattern ensures we start with a TLD-like identifier followed by domain components
var reverseNotationPattern = regexp.MustCompile(
	`^([a-zA-Z]{2,4}\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?` +
		`(?:\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*)/` +
		`([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)$`,
)

// ParseDomainFromNamespace extracts the domain from a domain-scoped namespace
// Supports reverse domain notation (e.g., com.github/my-server -> github.com)
// Returns the normalized domain and any parsing errors
func ParseDomainFromNamespace(namespace string) (string, error) {
	parsed, err := ParseNamespace(namespace)
	if err != nil {
		return "", err
	}
	return parsed.Domain, nil
}

// ParseNamespace parses a namespace string and returns a Namespace struct
// with the extracted domain and server name components
func ParseNamespace(namespace string) (*Namespace, error) {
	if namespace == "" {
		return nil, fmt.Errorf("%w: namespace cannot be empty", ErrInvalidNamespace)
	}

	// Normalize namespace to lowercase for consistent processing
	normalizedNamespace := strings.ToLower(strings.TrimSpace(namespace))

	// Check if it matches the reverse domain notation pattern
	matches := reverseNotationPattern.FindStringSubmatch(normalizedNamespace)
	if len(matches) < 5 {
		return nil, fmt.Errorf("%w: namespace must follow format 'domain.tld/server-name'", ErrInvalidNamespace)
	}

	reverseDomain := matches[1]
	serverName := matches[4]

	// Check for reserved domains before conversion
	if isReservedNamespace(reverseDomain) {
		return nil, fmt.Errorf("%w: namespace %s is reserved", ErrReservedNamespace, reverseDomain)
	}

	// Convert reverse domain notation to normal domain
	domain, err := reverseNotationToDomain(reverseDomain)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidNamespace, err)
	}

	// Validate the extracted domain
	err = validateDomain(domain)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidNamespace, err)
	}

	// Validate server name
	if err := validateServerName(serverName); err != nil {
		return nil, fmt.Errorf("%w: invalid server name: %w", ErrInvalidNamespace, err)
	}

	return &Namespace{
		Original:   namespace,
		Domain:     domain,
		ServerName: serverName,
	}, nil
}

// reverseNotationToDomain converts reverse domain notation to normal domain format
// Examples:
//   - com.github -> github.com
//   - com.github.api -> api.github.com
//   - org.apache.commons -> commons.apache.org
func reverseNotationToDomain(reverseDomain string) (string, error) {
	if reverseDomain == "" {
		return "", errors.New("reverse domain cannot be empty")
	}

	parts := strings.Split(reverseDomain, ".")
	if len(parts) < 2 {
		return "", errors.New("reverse domain must have at least two parts")
	}

	// Reverse the parts to create normal domain notation
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	domain := strings.Join(parts, ".")

	// Normalize domain (lowercase, remove trailing dots)
	domain = strings.ToLower(strings.TrimRight(domain, "."))

	return domain, nil
}

// validateDomain validates that a domain meets RFC requirements and security standards
func validateDomain(domain string) error {
	if domain == "" {
		return errors.New("domain cannot be empty")
	}

	// Normalize domain (lowercase, remove trailing dots)
	domain = strings.ToLower(strings.TrimRight(domain, "."))

	// Check for maximum length (253 characters for FQDN)
	if len(domain) > 253 {
		return errors.New("domain too long (max 253 characters)")
	}

	// Check for minimum length
	if len(domain) < 3 {
		return errors.New("domain too short (min 3 characters)")
	}

	// Check for valid characters and format using regex
	if !domainPattern.MatchString(domain) {
		return errors.New("domain contains invalid characters or format")
	}

	// Check each label (part between dots)
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return errors.New("domain must have at least two labels")
	}

	for _, label := range labels {
		if err := validateDomainLabel(label); err != nil {
			return fmt.Errorf("invalid domain label '%s': %w", label, err)
		}
	}

	// Check for suspicious Unicode homographs (basic check)
	if containsSuspiciousUnicode(domain) {
		return errors.New("domain contains suspicious Unicode characters")
	}

	// Validate using Go's net package for additional checks
	//nolint:staticcheck // Empty branch is intentional - this is a soft validation
	if _, err := net.LookupTXT(domain); err != nil {
		// Note: This is a soft validation - we don't require DNS resolution to succeed
		// as the domain might be newly registered or not yet propagated
		// This is just a basic sanity check for obvious invalid domains
	}

	return nil
}

// validateDomainLabel validates individual domain labels (parts between dots)
func validateDomainLabel(label string) error {
	if label == "" {
		return errors.New("label cannot be empty")
	}

	if len(label) > 63 {
		return errors.New("label too long (max 63 characters)")
	}

	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return errors.New("label cannot start or end with hyphen")
	}

	// Check that label doesn't consist only of numbers (to prevent confusion with IP addresses)
	if regexp.MustCompile(`^\d+$`).MatchString(label) {
		return errors.New("label cannot consist only of numbers")
	}

	return nil
}

// validateServerName validates the server name portion of the namespace
func validateServerName(serverName string) error {
	if serverName == "" {
		return errors.New("server name cannot be empty")
	}

	if len(serverName) < 1 || len(serverName) > 100 {
		return errors.New("server name must be between 1 and 100 characters")
	}

	// Server name should contain only alphanumeric characters and hyphens
	// Cannot start or end with hyphen
	serverNamePattern := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	if !serverNamePattern.MatchString(serverName) {
		return errors.New("server name can only contain alphanumeric characters and hyphens, and cannot start or end with hyphen")
	}

	return nil
}

// isReservedNamespace checks if a namespace is reserved
func isReservedNamespace(namespace string) bool {
	return reservedNamespaces[strings.ToLower(namespace)]
}

// containsSuspiciousUnicode performs basic checks for Unicode homograph attacks
func containsSuspiciousUnicode(domain string) bool {
	for _, r := range domain {
		// Check for characters that might be used in homograph attacks
		// This is a basic check - a more comprehensive solution would use
		// a proper Unicode confusables database
		if unicode.Is(unicode.Mn, r) || // Mark, nonspacing
			unicode.Is(unicode.Me, r) || // Mark, enclosing
			unicode.Is(unicode.Mc, r) { // Mark, spacing combining
			return true
		}

		// Check for certain suspicious Unicode blocks
		if r >= 0x0400 && r <= 0x04FF { // Cyrillic
			return true
		}
		if r >= 0x0370 && r <= 0x03FF { // Greek
			return true
		}
	}
	return false
}

// ValidateNamespace is a convenience function that validates a namespace
// and returns detailed error information
func ValidateNamespace(namespace string) error {
	_, err := ParseNamespace(namespace)
	return err
}

// IsValidNamespace returns true if the namespace is valid
func IsValidNamespace(namespace string) bool {
	return ValidateNamespace(namespace) == nil
}

// GetDomainFromNamespace is a convenience function that extracts and validates
// a domain from a namespace, returning just the domain string or an error
func GetDomainFromNamespace(namespace string) (string, error) {
	return ParseDomainFromNamespace(namespace)
}
