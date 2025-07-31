package verification

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPVerifier handles HTTP-01 well-known URL verification
type HTTPVerifier struct {
	client *http.Client
}

// HTTPVerifierOption configures HTTPVerifier
type HTTPVerifierOption func(*HTTPVerifier)

// WithInsecureSkipVerify configures the HTTPVerifier to skip TLS certificate verification
// This should only be used for testing purposes
func WithInsecureSkipVerify() HTTPVerifierOption {
	return func(hv *HTTPVerifier) {
		if transport, ok := hv.client.Transport.(*http.Transport); ok {
			transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}
}

// NewHTTPVerifier creates a new HTTP verifier with secure defaults
func NewHTTPVerifier(opts ...HTTPVerifierOption) *HTTPVerifier {
	// Create HTTP client with secure defaults and reasonable timeouts
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12, // Require TLS 1.2 or higher
			},
			ResponseHeaderTimeout: 5 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
	}

	verifier := &HTTPVerifier{
		client: client,
	}

	// Apply options
	for _, opt := range opts {
		opt(verifier)
	}

	return verifier
}

// VerifyDomainHTTP verifies domain ownership by fetching a token from the well-known URL
// The URL format is: https://<domain>/.well-known/mcp-challenge/<token>
// The response should contain the token as plain text
func (h *HTTPVerifier) VerifyDomainHTTP(ctx context.Context, domain, expectedToken string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	if expectedToken == "" {
		return fmt.Errorf("expected token cannot be empty")
	}

	// Construct the well-known URL
	wellKnownURL := fmt.Sprintf("https://%s/.well-known/mcp-challenge/%s", domain, expectedToken)

	// Validate the URL format
	parsedURL, err := url.Parse(wellKnownURL)
	if err != nil {
		return fmt.Errorf("invalid URL format for domain %s: %w", domain, err)
	}

	// Ensure we're using HTTPS only
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTPS is supported for domain verification")
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set User-Agent to identify as MCP Registry
	req.Header.Set("User-Agent", "MCP-Registry/1.0 (+https://github.com/modelcontextprotocol/registry)")

	// Make the HTTP request
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch verification URL %s: %w", wellKnownURL, err)
	}
	defer resp.Body.Close()

	// Check for successful status code (only 200 OK is accepted)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP verification failed for domain %s: received status %d, expected 200", domain, resp.StatusCode)
	}

	// Read the response body (limit to reasonable size to prevent DoS)
	const maxResponseSize = 1024 // 1KB should be more than enough for a token
	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("failed to read response body from %s: %w", wellKnownURL, err)
	}

	// Convert response to string and trim whitespace
	responseToken := strings.TrimSpace(string(body))

	// Verify the token matches exactly
	if responseToken != expectedToken {
		return fmt.Errorf("HTTP verification failed for domain %s: token mismatch (expected %s, got %s)", domain, expectedToken, responseToken)
	}

	return nil
}

// VerifyDomainHTTPWithRetry verifies domain ownership with retry logic for transient failures
func (h *HTTPVerifier) VerifyDomainHTTPWithRetry(ctx context.Context, domain, expectedToken string, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := h.VerifyDomainHTTP(ctx, domain, expectedToken)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry for certain types of errors (validation, auth, etc.)
		if isNonRetryableError(err) {
			return err
		}

		// If this wasn't the last attempt, wait before retrying
		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s...
			backoffDuration := time.Duration(1<<attempt) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDuration):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("HTTP verification failed after %d attempts: %w", maxRetries+1, lastErr)
}

// isNonRetryableError determines if an error should not be retried
func isNonRetryableError(err error) bool {
	errorStr := err.Error()

	// Don't retry for validation errors
	if strings.Contains(errorStr, "domain cannot be empty") ||
		strings.Contains(errorStr, "expected token cannot be empty") ||
		strings.Contains(errorStr, "invalid URL format") ||
		strings.Contains(errorStr, "only HTTPS is supported") ||
		strings.Contains(errorStr, "token mismatch") {
		return true
	}

	// Don't retry for 4xx client errors (except 408, 429)
	if strings.Contains(errorStr, "received status 4") {
		// But allow retries for timeouts and rate limiting
		if strings.Contains(errorStr, "received status 408") || // Request Timeout
			strings.Contains(errorStr, "received status 429") { // Too Many Requests
			return false
		}
		return true
	}

	return false
}
