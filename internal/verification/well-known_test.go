package verification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWellKnownVerifier_VerifyWellKnownToken(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse string
		serverStatus   int
		expectedToken  string
		expectError    bool
		protocol       string
		hostport       string
	}{
		{
			name:           "successful verification with clean token",
			serverResponse: "abc123def456",
			serverStatus:   http.StatusOK,
			expectedToken:  "abc123def456",
			expectError:    false,
			protocol:       "https",
			hostport:       "example.com",
		},
		{
			name:           "successful verification with whitespace trimming",
			serverResponse: "  \n\t abc123def456 \n\t  ",
			serverStatus:   http.StatusOK,
			expectedToken:  "abc123def456",
			expectError:    false,
			protocol:       "http",
			hostport:       "localhost:8080",
		},
		{
			name:           "empty token after trimming",
			serverResponse: "   \n\t   ",
			serverStatus:   http.StatusOK,
			expectedToken:  "",
			expectError:    true,
			protocol:       "https",
			hostport:       "example.com",
		},
		{
			name:           "404 not found",
			serverResponse: "Not Found",
			serverStatus:   http.StatusNotFound,
			expectedToken:  "",
			expectError:    true,
			protocol:       "https",
			hostport:       "example.com",
		},
		{
			name:           "500 internal server error",
			serverResponse: "Internal Server Error",
			serverStatus:   http.StatusInternalServerError,
			expectedToken:  "",
			expectError:    true,
			protocol:       "https",
			hostport:       "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/.well-known/mcp-verify", r.URL.Path)
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "MCP-Registry-Verifier/1.0", r.Header.Get("User-Agent"))
				assert.Equal(t, "text/plain", r.Header.Get("Accept"))

				w.WriteHeader(tt.serverStatus)
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()

			// Extract host and port from test server URL
			serverURL := strings.TrimPrefix(server.URL, "http://")

			// Create verifier with fast timeouts for testing
			config := &WellKnownConfig{
				InitialBackoff: 10 * time.Millisecond,
				MaxBackoff:     100 * time.Millisecond,
				MaxRetries:     2,
				RequestTimeout: 5 * time.Second,
			}
			verifier := NewWellKnownVerifier(config)

			ctx := context.Background()
			token, err := verifier.VerifyWellKnownToken(ctx, "http", serverURL)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedToken, token)
			}
		})
	}
}

func TestWellKnownVerifier_ExponentialBackoff(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail the first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server Error"))
		} else {
			// Succeed on the third attempt
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success-token"))
		}
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	config := &WellKnownConfig{
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     200 * time.Millisecond,
		MaxRetries:     3,
		RequestTimeout: 5 * time.Second,
	}
	verifier := NewWellKnownVerifier(config)

	start := time.Now()
	ctx := context.Background()
	token, err := verifier.VerifyWellKnownToken(ctx, "http", serverURL)
	elapsed := time.Since(start)

	// Should succeed after retries
	assert.NoError(t, err)
	assert.Equal(t, "success-token", token)
	assert.Equal(t, 3, attempts)

	// Should have taken some time due to backoff (at least 50ms + 100ms for two retries)
	assert.GreaterOrEqual(t, elapsed, 150*time.Millisecond)
}

func TestWellKnownVerifier_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return server error to trigger retries
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Server Error"))
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	config := &WellKnownConfig{
		InitialBackoff: 1 * time.Second, // Long backoff to test cancellation
		MaxBackoff:     5 * time.Second,
		MaxRetries:     5,
		RequestTimeout: 10 * time.Second,
	}
	verifier := NewWellKnownVerifier(config)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	token, err := verifier.VerifyWellKnownToken(ctx, "http", serverURL)

	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestWellKnownVerifier_NonRetryable4xxError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	serverURL := strings.TrimPrefix(server.URL, "http://")

	config := &WellKnownConfig{
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		MaxRetries:     3,
		RequestTimeout: 5 * time.Second,
	}
	verifier := NewWellKnownVerifier(config)

	ctx := context.Background()
	token, err := verifier.VerifyWellKnownToken(ctx, "http", serverURL)

	// Should fail immediately without retries for 4xx errors
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, 1, attempts)
	assert.Contains(t, err.Error(), "non-retryable HTTP error")
}

func TestDefaultWellKnownConfig(t *testing.T) {
	config := DefaultWellKnownConfig()

	require.NotNil(t, config)
	assert.Equal(t, 100*time.Millisecond, config.InitialBackoff)
	assert.Equal(t, 10*time.Second, config.MaxBackoff)
	assert.Equal(t, 5, config.MaxRetries)
	assert.Equal(t, 30*time.Second, config.RequestTimeout)
}

func TestNewWellKnownVerifier_WithNilConfig(t *testing.T) {
	verifier := NewWellKnownVerifier(nil)

	require.NotNil(t, verifier)
	require.NotNil(t, verifier.config)

	// Should use default config
	expected := DefaultWellKnownConfig()
	assert.Equal(t, expected.InitialBackoff, verifier.config.InitialBackoff)
	assert.Equal(t, expected.MaxBackoff, verifier.config.MaxBackoff)
	assert.Equal(t, expected.MaxRetries, verifier.config.MaxRetries)
	assert.Equal(t, expected.RequestTimeout, verifier.config.RequestTimeout)
}

func TestHTTPError(t *testing.T) {
	err := &HTTPError{
		StatusCode: 404,
		Status:     "404 Not Found",
		URL:        "https://example.com/.well-known/mcp-verify",
	}

	expected := "HTTP 404 (404 Not Found) for URL: https://example.com/.well-known/mcp-verify"
	assert.Equal(t, expected, err.Error())
}
