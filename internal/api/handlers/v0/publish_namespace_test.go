package v0

import (
	"testing"

	"github.com/modelcontextprotocol/registry/internal/namespace"
)

// TestNamespaceValidationIntegration tests that the namespace validation
// functionality is working correctly for various namespace formats
func TestNamespaceValidationIntegration(t *testing.T) {
	tests := []struct {
		name        string
		serverName  string
		shouldPass  bool
		description string
	}{
		{
			name:        "valid domain-scoped namespace",
			serverName:  "com.github/my-server",
			shouldPass:  true,
			description: "Standard domain-scoped namespace should be valid",
		},
		{
			name:        "valid subdomain namespace",
			serverName:  "com.github.api/tool",
			shouldPass:  true,
			description: "Subdomain-scoped namespace should be valid",
		},
		{
			name:        "valid apache commons namespace",
			serverName:  "org.apache.commons/utility",
			shouldPass:  true,
			description: "Apache commons style namespace should be valid",
		},
		{
			name:        "valid kubernetes namespace",
			serverName:  "io.kubernetes/plugin",
			shouldPass:  true,
			description: "Kubernetes.io style namespace should be valid",
		},
		{
			name:        "reserved namespace - localhost",
			serverName:  "com.localhost/server",
			shouldPass:  false,
			description: "Reserved localhost namespace should be rejected",
		},
		{
			name:        "reserved namespace - example",
			serverName:  "com.example/server",
			shouldPass:  false,
			description: "Reserved example namespace should be rejected",
		},
		{
			name:        "legacy format - io.github prefix",
			serverName:  "io.github.username/my-server",
			shouldPass:  true, // This is valid reverse domain notation
			description: "Legacy io.github format should be valid as reverse domain notation",
		},
		{
			name:        "invalid format - forward domain notation",
			serverName:  "github.com/my-server",
			shouldPass:  false,
			description: "Forward domain notation should be rejected",
		},
		{
			name:        "invalid format - simple name",
			serverName:  "my-server",
			shouldPass:  false,
			description: "Simple server name should not match domain-scoped pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := namespace.ValidateNamespace(tt.serverName)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("Expected namespace '%s' to be valid, but got error: %v", tt.serverName, err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected namespace '%s' to be invalid, but validation passed", tt.serverName)
				}
			}
		})
	}
}

// TestDomainExtractionIntegration tests the domain extraction functionality
func TestDomainExtractionIntegration(t *testing.T) {
	// Test valid extractions
	validTests := []struct {
		namespace string
		domain    string
	}{
		{"com.github/my-server", "github.com"},
		{"com.github.api/tool", "api.github.com"},
		{"org.apache.commons/utility", "commons.apache.org"},
		{"io.kubernetes/plugin", "kubernetes.io"},
	}

	for _, test := range validTests {
		t.Run("extract_"+test.domain, func(t *testing.T) {
			domain, err := namespace.ParseDomainFromNamespace(test.namespace)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			} else if domain != test.domain {
				t.Errorf("Expected '%s', got '%s'", test.domain, domain)
			}
		})
	}

	// Test invalid extractions
	invalidTests := []string{"invalid-format", "github.com/server", "simple"}
	for _, test := range invalidTests {
		t.Run("invalid_"+test, func(t *testing.T) {
			_, err := namespace.ParseDomainFromNamespace(test)
			if err == nil {
				t.Errorf("Expected error for '%s'", test)
			}
		})
	}
}
