package verification

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// HTTPVerificationError represents errors that can occur during HTTP verification
type HTTPVerificationError struct {
	Domain  string
	Token   string
	URL     string
	Message string
	Cause   error
}

func (e *HTTPVerificationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("HTTP verification failed for domain %s: %s (cause: %v)",
			e.Domain, e.Message, e.Cause)
	}
	return fmt.Sprintf("HTTP verification failed for domain %s: %s", e.Domain, e.Message)
}

func (e *HTTPVerificationError) Unwrap() error {
	return e.Cause
}

// HTTPVerificationResult represents the result of an HTTP verification attempt
type HTTPVerificationResult struct {
	Success      bool   `json:"success"`
	Domain       string `json:"domain"`
	Token        string `json:"token"`
	URL          string `json:"url"`
	Message      string `json:"message"`
	StatusCode   int    `json:"status_code,omitempty"`
	ResponseBody string `json:"response_body,omitempty"`
	Duration     string `json:"duration"`
}

// HTTPVerificationConfig holds configuration for HTTP verification
type HTTPVerificationConfig struct {
	// Timeout for HTTP requests (default: 10 seconds)
	Timeout time.Duration

	// MaxRetries for transient failures (default: 3)
	MaxRetries int

	// RetryDelay base delay between retries (default: 1 second)
	RetryDelay time.Duration

	// FollowRedirects whether to follow HTTP redirects (default: true)
	FollowRedirects bool

	// UserAgent to use for HTTP requests (default: "MCP-Registry-Verifier/1.0")
	UserAgent string

	// AllowHTTP whether to allow non-HTTPS URLs (default: false for security)
	AllowHTTP bool

	// MaxResponseSize maximum size of response body to read (default: 1KB)
	MaxResponseSize int64

	// CustomTransport allows injecting a custom HTTP transport (primarily for testing)
	CustomTransport http.RoundTripper
}

// DefaultHTTPConfig returns the default configuration for HTTP verification
func DefaultHTTPConfig() *HTTPVerificationConfig {
	return &HTTPVerificationConfig{
		Timeout:         10 * time.Second,
		MaxRetries:      3,
		RetryDelay:      1 * time.Second,
		FollowRedirects: true,
		UserAgent:       "MCP-Registry-Verifier/1.0",
		AllowHTTP:       false,
		MaxResponseSize: 1024, // 1KB should be enough for a token
	}
}

// VerifyHTTPChallenge verifies domain ownership by checking for a specific token
// at the well-known HTTP-01 challenge URL: https://domain/.well-known/mcp-challenge/token
//
// This function implements the HTTP-01 web challenge verification method described
// in the Server Name Verification system. It fetches the well-known URL and verifies
// that the response body exactly matches the expected token.
//
// Security considerations:
// - Only allows HTTPS by default to prevent man-in-the-middle attacks
// - Uses a short timeout to prevent hanging on slow responses
// - Limits response body size to prevent memory exhaustion attacks
// - Implements retry logic with exponential backoff for transient failures
// - Validates token format before making the HTTP request
//
// Parameters:
// - domain: The domain name to verify (e.g., "example.com")
// - expectedToken: The 128-bit token that should be served at the challenge URL
//
// Returns:
// - HTTPVerificationResult with verification status and details
// - An error if the verification process fails critically
//
// The default configuration uses HTTPS-only. To allow HTTP (not recommended for production),
// use VerifyHTTPChallengeWithConfig with AllowHTTP set to true.
//
// Example usage:
//
//	result, err := VerifyHTTPChallenge("example.com", "TBeVXe_X4npM6p8vpzStnA")
//	if err != nil {
//	    log.Printf("HTTP verification error: %v", err)
//	    return err
//	}
//	if result.Success {
//	    log.Printf("Domain %s verified successfully via HTTP", result.Domain)
//	} else {
//	    log.Printf("Domain %s verification failed: %s", result.Domain, result.Message)
//	}
func VerifyHTTPChallenge(domain, expectedToken string) (*HTTPVerificationResult, error) {
	return VerifyHTTPChallengeWithConfig(domain, expectedToken, DefaultHTTPConfig())
}

// VerifyHTTPChallengeWithConfig performs HTTP verification with custom configuration
func VerifyHTTPChallengeWithConfig(domain, expectedToken string, config *HTTPVerificationConfig) (*HTTPVerificationResult, error) {
	startTime := time.Now()

	// Input validation
	if domain == "" {
		return nil, &HTTPVerificationError{
			Domain:  domain,
			Token:   expectedToken,
			Message: "domain cannot be empty",
		}
	}

	if expectedToken == "" {
		return nil, &HTTPVerificationError{
			Domain:  domain,
			Token:   expectedToken,
			Message: "token cannot be empty",
		}
	}

	// Validate token format
	if !ValidateTokenFormat(expectedToken) {
		return nil, &HTTPVerificationError{
			Domain:  domain,
			Token:   expectedToken,
			Message: "invalid token format",
		}
	}

	// Normalize domain (remove trailing dots, convert to lowercase)
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))

	log.Printf("Starting HTTP verification for domain: %s with token: %s", domain, expectedToken)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Perform verification with retries
	result, err := performHTTPVerificationWithRetries(ctx, domain, expectedToken, config)

	// Calculate duration
	duration := time.Since(startTime)
	if result != nil {
		result.Duration = duration.String()
	}

	log.Printf("HTTP verification completed for domain %s in %v: success=%t",
		domain, duration, result != nil && result.Success)

	return result, err
}

// performHTTPVerificationWithRetries implements the retry logic for HTTP verification
func performHTTPVerificationWithRetries(
	ctx context.Context,
	domain, expectedToken string,
	config *HTTPVerificationConfig,
) (*HTTPVerificationResult, error) {
	var lastErr error
	var lastResult *HTTPVerificationResult

	retryDelay := config.RetryDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("HTTP verification retry %d/%d for domain %s after %v delay",
				attempt+1, config.MaxRetries, domain, retryDelay)

			// Wait before retry with context cancellation support
			timer := time.NewTimer(retryDelay)
			select {
			case <-timer.C:
				// Timer fired normally, continue with retry
			case <-ctx.Done():
				// Context canceled, stop timer to prevent leak
				timer.Stop()
				return nil, &HTTPVerificationError{
					Domain:  domain,
					Token:   expectedToken,
					Message: "verification canceled",
					Cause:   ctx.Err(),
				}
			}

			// Exponential backoff
			retryDelay *= 2
		}

		result, err := performHTTPVerification(ctx, domain, expectedToken, config)
		if err == nil {
			return result, nil
		}

		lastErr = err
		lastResult = result

		// Check if error is retryable
		if !IsRetryableHTTPError(err) {
			log.Printf("Non-retryable HTTP error for domain %s: %v", domain, err)
			break
		}

		log.Printf("Retryable HTTP error for domain %s (attempt %d/%d): %v",
			domain, attempt+1, config.MaxRetries, err)
	}

	// All retries exhausted
	return lastResult, lastErr
}

// performHTTPVerification performs a single HTTP verification attempt
func performHTTPVerification(ctx context.Context, domain, expectedToken string, config *HTTPVerificationConfig) (*HTTPVerificationResult, error) {
	// Construct the challenge URL
	scheme := "https"
	if config.AllowHTTP {
		scheme = "http"
	}
	challengeURL := fmt.Sprintf("%s://%s/.well-known/mcp-challenge/%s", scheme, domain, expectedToken)

	// Create HTTP client
	client := createHTTPClient(config)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", challengeURL, nil)
	if err != nil {
		httpErr := &HTTPVerificationError{
			Domain:  domain,
			Token:   expectedToken,
			URL:     challengeURL,
			Message: "failed to create HTTP request",
			Cause:   err,
		}

		result := &HTTPVerificationResult{
			Success: false,
			Domain:  domain,
			Token:   expectedToken,
			URL:     challengeURL,
			Message: httpErr.Message,
		}

		return result, httpErr
	}

	// Set User-Agent
	req.Header.Set("User-Agent", config.UserAgent)

	log.Printf("Making HTTP request to: %s", challengeURL)

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		httpErr := &HTTPVerificationError{
			Domain:  domain,
			Token:   expectedToken,
			URL:     challengeURL,
			Message: "failed to make HTTP request",
			Cause:   err,
		}

		result := &HTTPVerificationResult{
			Success: false,
			Domain:  domain,
			Token:   expectedToken,
			URL:     challengeURL,
			Message: httpErr.Message,
		}

		return result, httpErr
	}
	defer resp.Body.Close()

	log.Printf("HTTP response status: %d for URL: %s", resp.StatusCode, challengeURL)

	// Check status code
	if resp.StatusCode != http.StatusOK {
		result := &HTTPVerificationResult{
			Success:    false,
			Domain:     domain,
			Token:      expectedToken,
			URL:        challengeURL,
			Message:    fmt.Sprintf("HTTP request failed with status %d", resp.StatusCode),
			StatusCode: resp.StatusCode,
		}

		log.Printf("HTTP verification failed for domain %s: unexpected status code %d", domain, resp.StatusCode)
		return result, nil
	}

	// Read response body with size limit
	limitedReader := io.LimitReader(resp.Body, config.MaxResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		httpErr := &HTTPVerificationError{
			Domain:  domain,
			Token:   expectedToken,
			URL:     challengeURL,
			Message: "failed to read response body",
			Cause:   err,
		}

		result := &HTTPVerificationResult{
			Success:    false,
			Domain:     domain,
			Token:      expectedToken,
			URL:        challengeURL,
			Message:    httpErr.Message,
			StatusCode: resp.StatusCode,
		}

		return result, httpErr
	}

	responseBody := strings.TrimSpace(string(body))
	log.Printf("HTTP response body: '%s' (expected: '%s')", responseBody, expectedToken)

	// Check if response body matches expected token
	if responseBody == expectedToken {
		result := &HTTPVerificationResult{
			Success:      true,
			Domain:       domain,
			Token:        expectedToken,
			URL:          challengeURL,
			Message:      "domain verification successful",
			StatusCode:   resp.StatusCode,
			ResponseBody: responseBody,
		}

		log.Printf("HTTP verification successful for domain %s", domain)
		return result, nil
	}

	// Token mismatch
	result := &HTTPVerificationResult{
		Success:      false,
		Domain:       domain,
		Token:        expectedToken,
		URL:          challengeURL,
		Message:      fmt.Sprintf("token mismatch: expected '%s', got '%s'", expectedToken, responseBody),
		StatusCode:   resp.StatusCode,
		ResponseBody: responseBody,
	}

	log.Printf("HTTP verification failed for domain %s: token mismatch", domain)
	return result, nil
}

// createHTTPClient creates an HTTP client with the specified configuration
func createHTTPClient(config *HTTPVerificationConfig) *http.Client {
	// Use custom transport if provided (for testing)
	if config.CustomTransport != nil {
		return &http.Client{
			Transport: config.CustomTransport,
			Timeout:   config.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if !config.FollowRedirects {
					return http.ErrUseLastResponse
				}
				// Limit redirects to prevent infinite loops
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}
	}

	// Create custom transport with security settings
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
		TLSClientConfig: &tls.Config{
			// Require valid certificates (no self-signed)
			InsecureSkipVerify: false,
		},
		DisableKeepAlives: true, // Don't reuse connections for verification requests
	}

	return &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !config.FollowRedirects {
				return http.ErrUseLastResponse
			}
			// Limit redirects to prevent infinite loops
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

// IsRetryableHTTPError determines if an HTTP error should be retried
func IsRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network timeouts and temporary failures
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check for context timeout
	if err == context.DeadlineExceeded {
		return true
	}

	// Check for DNS errors (might be temporary)
	if dnsErr, ok := err.(*net.DNSError); ok {
		return dnsErr.Temporary()
	}

	// Don't retry on validation errors or permanent failures
	if _, ok := err.(*HTTPVerificationError); ok {
		return false
	}

	// Default to not retryable for unknown errors
	return false
}
