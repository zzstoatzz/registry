package verification

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WellKnownConfig holds configuration for well-known verification
type WellKnownConfig struct {
	// Initial retry delay (will be doubled with each retry)
	InitialBackoff time.Duration
	// Maximum retry delay
	MaxBackoff time.Duration
	// Maximum number of retry attempts
	MaxRetries int
	// HTTP client timeout for each request
	RequestTimeout time.Duration
}

// DefaultWellKnownConfig returns a default configuration for well-known verification
func DefaultWellKnownConfig() *WellKnownConfig {
	return &WellKnownConfig{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		MaxRetries:     5,
		RequestTimeout: 30 * time.Second,
	}
}

// WellKnownVerifier handles verification of well-known files
type WellKnownVerifier struct {
	config *WellKnownConfig
	client *http.Client
}

// NewWellKnownVerifier creates a new well-known verifier with the given configuration
func NewWellKnownVerifier(config *WellKnownConfig) *WellKnownVerifier {
	if config == nil {
		config = DefaultWellKnownConfig()
	}

	return &WellKnownVerifier{
		config: config,
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}
}

// VerifyWellKnownToken fetches and verifies a token from the well-known endpoint
// It constructs the URL as <protocol>://<hostport>/.well-known/mcp-verify
// and expects a plaintext response containing the token
func (v *WellKnownVerifier) VerifyWellKnownToken(ctx context.Context, protocol, hostport string) (string, error) {
	url := fmt.Sprintf("%s://%s/.well-known/mcp-verify", protocol, hostport)

	token, err := v.fetchTokenWithBackoff(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch token from %s: %w", url, err)
	}

	// Trim whitespace from the token
	token = strings.TrimSpace(token)

	if token == "" {
		return "", fmt.Errorf("token is empty after trimming whitespace")
	}

	// TODO: Call verification method on the token
	return token, nil
}

// fetchTokenWithBackoff fetches the token from the given URL with exponential backoff
func (v *WellKnownVerifier) fetchTokenWithBackoff(ctx context.Context, url string) (string, error) {
	var lastErr error
	backoff := v.config.InitialBackoff

	for attempt := 0; attempt <= v.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait with exponential backoff (except for the first attempt)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
				// Continue with the retry
			}

			// Double the backoff for next attempt, but cap at MaxBackoff
			backoff *= 2
			if backoff > v.config.MaxBackoff {
				backoff = v.config.MaxBackoff
			}
		}

		token, err := v.fetchToken(ctx, url)
		if err == nil {
			return token, nil
		}

		lastErr = err

		// Check if this is a non-retryable error (4xx client errors)
		if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			return "", fmt.Errorf("non-retryable HTTP error: %w", err)
		}
	}

	return "", fmt.Errorf("exhausted all %d retry attempts, last error: %w", v.config.MaxRetries+1, lastErr)
}

// fetchToken makes a single HTTP request to fetch the token
func (v *WellKnownVerifier) fetchToken(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set appropriate headers
	req.Header.Set("User-Agent", "MCP-Registry-Verifier/1.0")
	req.Header.Set("Accept", "text/plain")

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        url,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// HTTPError represents an HTTP error response
type HTTPError struct {
	StatusCode int
	Status     string
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d (%s) for URL: %s", e.StatusCode, e.Status, e.URL)
}
