//nolint:testpackage // Internal package testing allows access to private functions
package namespace

import (
	"errors"
	"testing"
)

// Test constants to avoid duplication
const (
	testGitHubNamespace     = "com.github/my-server"
	testGitHubDomain        = "github.com"
	testMyServer            = "my-server"
	testAPIGitHubDomain     = "api.github.com"
	testApacheCommonsDomain = "commons.apache.org"
	testKubernetesIODomain  = "kubernetes.io"
	testGitHubReverseDomain = "com.github"
	testLocalhostNamespace  = "com.localhost/server"
	invalidNamespaceLabel   = "invalid namespace"
)

func TestParseNamespaceValidCases(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		wantDomain string
		wantServer string
	}{
		{
			name:       "simple github namespace",
			namespace:  testGitHubNamespace,
			wantDomain: testGitHubDomain,
			wantServer: testMyServer,
		},
		{
			name:       "subdomain namespace",
			namespace:  "com.github.api/tool",
			wantDomain: testAPIGitHubDomain,
			wantServer: "tool",
		},
		{
			name:       "apache commons namespace",
			namespace:  "org.apache.commons/utility",
			wantDomain: testApacheCommonsDomain,
			wantServer: "utility",
		},
		{
			name:       "kubernetes io namespace",
			namespace:  "io.kubernetes/plugin",
			wantDomain: testKubernetesIODomain,
			wantServer: "plugin",
		},
		{
			name:       "case normalization",
			namespace:  "COM.GITHUB/MY-SERVER",
			wantDomain: testGitHubDomain,
			wantServer: testMyServer,
		},
		{
			name:       "with whitespace",
			namespace:  "  " + testGitHubNamespace + "  ",
			wantDomain: testGitHubDomain,
			wantServer: testMyServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseNamespace(tt.namespace)
			if err != nil {
				t.Errorf("ParseNamespace() unexpected error: %v", err)
				return
			}

			if result.Domain != tt.wantDomain {
				t.Errorf("ParseNamespace() domain = %v, want %v", result.Domain, tt.wantDomain)
			}

			if result.ServerName != tt.wantServer {
				t.Errorf("ParseNamespace() server name = %v, want %v", result.ServerName, tt.wantServer)
			}

			if result.Original != tt.namespace {
				t.Errorf("ParseNamespace() original = %v, want %v", result.Original, tt.namespace)
			}
		})
	}
}

func TestParseNamespaceInvalidCases(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		expectedErr error
	}{
		{
			name:        "empty namespace",
			namespace:   "",
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "missing server name",
			namespace:   testGitHubReverseDomain,
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "forward domain notation",
			namespace:   testGitHubDomain + "/" + testMyServer,
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "wildcard domain",
			namespace:   "*.github/" + testMyServer,
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "invalid domain format",
			namespace:   ".com.invalid/server",
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "reserved namespace localhost",
			namespace:   testLocalhostNamespace,
			expectedErr: ErrReservedNamespace,
		},
		{
			name:        "reserved namespace example",
			namespace:   "com.example/server",
			expectedErr: ErrReservedNamespace,
		},
		{
			name:        "single domain part",
			namespace:   "github/" + testMyServer,
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "server name with invalid characters",
			namespace:   testGitHubReverseDomain + "/my_server!",
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "server name starting with hyphen",
			namespace:   testGitHubReverseDomain + "/-server",
			expectedErr: ErrInvalidNamespace,
		},
		{
			name:        "server name ending with hyphen",
			namespace:   testGitHubReverseDomain + "/server-",
			expectedErr: ErrInvalidNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseNamespace(tt.namespace)
			if err == nil {
				t.Errorf("ParseNamespace() expected error but got none")
				return
			}
			if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
				t.Errorf("ParseNamespace() expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestParseDomainFromNamespace(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		wantDomain string
		wantErr    bool
	}{
		{
			name:       "github namespace",
			namespace:  testGitHubNamespace,
			wantDomain: testGitHubDomain,
			wantErr:    false,
		},
		{
			name:       "subdomain namespace",
			namespace:  "com.github.api/tool",
			wantDomain: testAPIGitHubDomain,
			wantErr:    false,
		},
		{
			name:      invalidNamespaceLabel,
			namespace: "invalid",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := ParseDomainFromNamespace(tt.namespace)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDomainFromNamespace() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseDomainFromNamespace() unexpected error: %v", err)
				return
			}

			if domain != tt.wantDomain {
				t.Errorf("ParseDomainFromNamespace() = %v, want %v", domain, tt.wantDomain)
			}
		})
	}
}

func TestReverseNotationToDomain(t *testing.T) {
	tests := []struct {
		name          string
		reverseDomain string
		wantDomain    string
		wantErr       bool
	}{
		{
			name:          "github",
			reverseDomain: testGitHubReverseDomain,
			wantDomain:    testGitHubDomain,
			wantErr:       false,
		},
		{
			name:          "github subdomain",
			reverseDomain: "com.github.api",
			wantDomain:    testAPIGitHubDomain,
			wantErr:       false,
		},
		{
			name:          "apache commons",
			reverseDomain: "org.apache.commons",
			wantDomain:    testApacheCommonsDomain,
			wantErr:       false,
		},
		{
			name:          "kubernetes io",
			reverseDomain: "io.kubernetes",
			wantDomain:    testKubernetesIODomain,
			wantErr:       false,
		},
		{
			name:          "case normalization",
			reverseDomain: "COM.GITHUB",
			wantDomain:    testGitHubDomain,
			wantErr:       false,
		},
		{
			name:          "empty string",
			reverseDomain: "",
			wantErr:       true,
		},
		{
			name:          "single part",
			reverseDomain: "github",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := reverseNotationToDomain(tt.reverseDomain)

			if tt.wantErr {
				if err == nil {
					t.Errorf("reverseNotationToDomain() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("reverseNotationToDomain() unexpected error: %v", err)
				return
			}

			if domain != tt.wantDomain {
				t.Errorf("reverseNotationToDomain() = %v, want %v", domain, tt.wantDomain)
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		// Valid domains
		{
			name:    "github.com",
			domain:  testGitHubDomain,
			wantErr: false,
		},
		{
			name:    "api.github.com",
			domain:  testAPIGitHubDomain,
			wantErr: false,
		},
		{
			name:    "kubernetes.io",
			domain:  testKubernetesIODomain,
			wantErr: false,
		},
		{
			name:    "commons.apache.org",
			domain:  testApacheCommonsDomain,
			wantErr: false,
		},
		// Invalid domains
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
		},
		{
			name:    "too short",
			domain:  "a.b",
			wantErr: false, // Actually valid - minimum is met
		},
		{
			name:    "single label",
			domain:  "localhost",
			wantErr: true,
		},
		{
			name:    "starts with dot",
			domain:  ".github.com",
			wantErr: true,
		},
		{
			name:    "ends with dot",
			domain:  "github.com.",
			wantErr: false, // Trailing dots are normalized away
		},
		{
			name:    "contains invalid characters",
			domain:  "github_invalid.com",
			wantErr: true,
		},
		{
			name:    "label starts with hyphen",
			domain:  "-github.com",
			wantErr: true,
		},
		{
			name:    "label ends with hyphen",
			domain:  "github-.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDomain(tt.domain)

			if tt.wantErr && err == nil {
				t.Errorf("validateDomain() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("validateDomain() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateServerName(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
		wantErr    bool
	}{
		// Valid server names
		{
			name:       "simple name",
			serverName: testMyServer,
			wantErr:    false,
		},
		{
			name:       "alphanumeric",
			serverName: "server123",
			wantErr:    false,
		},
		{
			name:       "with hyphens",
			serverName: "my-awesome-server",
			wantErr:    false,
		},
		{
			name:       "single character",
			serverName: "a",
			wantErr:    false,
		},
		// Invalid server names
		{
			name:       "empty",
			serverName: "",
			wantErr:    true,
		},
		{
			name:       "starts with hyphen",
			serverName: "-server",
			wantErr:    true,
		},
		{
			name:       "ends with hyphen",
			serverName: "server-",
			wantErr:    true,
		},
		{
			name:       "contains underscore",
			serverName: "my_server",
			wantErr:    true,
		},
		{
			name:       "contains special characters",
			serverName: "my-server!",
			wantErr:    true,
		},
		{
			name:       "only hyphen",
			serverName: "-",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServerName(tt.serverName)

			if tt.wantErr && err == nil {
				t.Errorf("validateServerName() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("validateServerName() unexpected error: %v", err)
			}
		})
	}
}

func TestIsReservedNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{
			name:      "localhost reserved",
			namespace: "com.localhost",
			want:      true,
		},
		{
			name:      "example reserved",
			namespace: "com.example",
			want:      true,
		},
		{
			name:      "test reserved",
			namespace: "org.test",
			want:      true,
		},
		{
			name:      "case insensitive",
			namespace: "COM.LOCALHOST",
			want:      true,
		},
		{
			name:      "github not reserved",
			namespace: "com.github",
			want:      false,
		},
		{
			name:      "custom domain not reserved",
			namespace: "com.mycompany",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReservedNamespace(tt.namespace)
			if got != tt.want {
				t.Errorf("isReservedNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		wantErr   bool
	}{
		{
			name:      "valid namespace",
			namespace: testGitHubNamespace,
			wantErr:   false,
		},
		{
			name:      invalidNamespaceLabel,
			namespace: "invalid",
			wantErr:   true,
		},
		{
			name:      "reserved namespace",
			namespace: testLocalhostNamespace,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNamespace(tt.namespace)

			if tt.wantErr && err == nil {
				t.Errorf("ValidateNamespace() expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("ValidateNamespace() unexpected error: %v", err)
			}
		})
	}
}

func TestIsValidNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      bool
	}{
		{
			name:      "valid namespace",
			namespace: testGitHubNamespace,
			want:      true,
		},
		{
			name:      invalidNamespaceLabel,
			namespace: "invalid",
			want:      false,
		},
		{
			name:      "reserved namespace",
			namespace: testLocalhostNamespace,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidNamespace(tt.namespace)
			if got != tt.want {
				t.Errorf("IsValidNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsSuspiciousUnicode(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		{
			name:   "normal ascii domain",
			domain: testGitHubDomain,
			want:   false,
		},
		{
			name:   "domain with cyrillic",
			domain: "githubрcom", // Contains Cyrillic 'р' (U+0440)
			want:   true,
		},
		{
			name:   "domain with greek",
			domain: "githubαcom", // Contains Greek 'α' (U+03B1)
			want:   true,
		},
		{
			name:   "normal domain with numbers",
			domain: "github123.com",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsSuspiciousUnicode(tt.domain)
			if got != tt.want {
				t.Errorf("containsSuspiciousUnicode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkParseNamespace(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ParseNamespace(testGitHubNamespace)
	}
}

func BenchmarkValidateDomain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = validateDomain(testGitHubDomain)
	}
}
