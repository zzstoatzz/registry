package verification_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/registry/internal/verification"
)

const (
	errMsgGenTokenHTTP         = "Failed to generate test token: %v"
	wellKnownChallengePathHTTP = "/.well-known/mcp-challenge/%s"
	httpsScheme                = "https://"
	errMsgUnexpectedHTTP       = "VerifyHTTPChallenge returned unexpected error: %v"
	errMsgNilResultHTTP        = "VerifyHTTPChallenge returned nil result"
	logMsgResultHTTP           = "HTTP verification result: %+v"
	testDomainHTTP             = "example.com"
	wrongTokenHTTP             = "wrong-token"
	resultStatusCodeHTTP       = "Result status code = %d, want %d"
	resultResponseBodyHTTP     = "Result response body = %s, want %s"
)

func TestVerifyHTTPChallenge(t *testing.T) {
	// Generate a test token
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenTokenHTTP, err)
	}

	// Create test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf(wellKnownChallengePathHTTP, token)
		if r.URL.Path == expectedPath {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(token))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Extract domain from test server URL
	domain := strings.TrimPrefix(server.URL, httpsScheme)

	// Create custom config with test server transport
	config := verification.DefaultHTTPConfig()
	config.CustomTransport = server.Client().Transport

	result, err := verification.VerifyHTTPChallengeWithConfig(context.Background(), domain, token, config)

	if err != nil {
		t.Errorf(errMsgUnexpectedHTTP, err)
	}

	if result == nil {
		t.Fatal(errMsgNilResultHTTP)
	}

	if !result.Success {
		t.Errorf("Expected verification to succeed, got: %s", result.Message)
	}

	if result.Domain != domain {
		t.Errorf("Result domain = %s, want %s", result.Domain, domain)
	}

	if result.Token != token {
		t.Errorf("Result token = %s, want %s", result.Token, token)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf(resultStatusCodeHTTP, result.StatusCode, http.StatusOK)
	}

	if result.ResponseBody != token {
		t.Errorf(resultResponseBodyHTTP, result.ResponseBody, token)
	}

	t.Logf(logMsgResultHTTP, result)
}

func TestVerifyHTTPChallengeTokenNotFound(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Create test server that returns 404 for all requests
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, "https://")

	config := verification.DefaultHTTPConfig()
	config.CustomTransport = server.Client().Transport

	result, err := verification.VerifyHTTPChallengeWithConfig(context.Background(), domain, token, config)

	if err != nil {
		t.Errorf("VerifyHTTPChallenge returned unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("VerifyHTTPChallenge returned nil result")
	}

	if result.Success {
		t.Error("Expected verification to fail when token is not found")
	}

	if !strings.Contains(result.Message, "404") {
		t.Errorf("Expected '404' in message, got: %s", result.Message)
	}

	if result.StatusCode != http.StatusNotFound {
		t.Errorf("Result status code = %d, want %d", result.StatusCode, http.StatusNotFound)
	}

	t.Logf("HTTP verification result: %+v", result)
}

func TestVerifyHTTPChallengeTokenMismatch(t *testing.T) {
	// Generate a test token
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenTokenHTTP, err)
	}

	// Create test server that returns wrong token
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf(wellKnownChallengePathHTTP, token)
		if r.URL.Path == expectedPath {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(wrongTokenHTTP))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Extract domain from test server URL
	domain := strings.TrimPrefix(server.URL, httpsScheme)

	// Create custom config with test server transport
	config := verification.DefaultHTTPConfig()
	config.CustomTransport = server.Client().Transport

	result, err := verification.VerifyHTTPChallengeWithConfig(context.Background(), domain, token, config)

	if err != nil {
		t.Errorf(errMsgUnexpectedHTTP, err)
	}

	if result == nil {
		t.Fatal(errMsgNilResultHTTP)
	}

	if result.Success {
		t.Error("Expected verification to fail due to token mismatch")
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf(resultStatusCodeHTTP, result.StatusCode, http.StatusOK)
	}

	if result.ResponseBody != wrongTokenHTTP {
		t.Errorf(resultResponseBodyHTTP, result.ResponseBody, wrongTokenHTTP)
	}

	t.Logf(logMsgResultHTTP, result)
}

func TestVerifyHTTPChallengeInvalidInputs(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenTokenHTTP, err)
	}

	testCases := []struct {
		name   string
		domain string
		token  string
	}{
		{"empty domain", "", token},
		{"empty token", testDomainHTTP, ""},
		{"invalid token", testDomainHTTP, "invalid-token"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := verification.VerifyHTTPChallenge(tc.domain, tc.token)

			if err == nil {
				t.Error("Expected error for invalid input")
			}

			var httpErr *verification.HTTPVerificationError
			if !errors.As(err, &httpErr) {
				t.Errorf("Expected HTTPVerificationError, got: %T", err)
			}

			// Result should be nil for validation errors
			if result != nil {
				t.Error("Expected nil result for validation error")
			}
		})
	}
}

func TestVerifyHTTPChallengeWithTimeout(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenTokenHTTP, err)
	}

	// Create test server that responds slowly
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Longer than the config timeout
		expectedPath := fmt.Sprintf(wellKnownChallengePathHTTP, token)
		if r.URL.Path == expectedPath {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(token))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, httpsScheme)

	config := &verification.HTTPVerificationConfig{
		Timeout:         100 * time.Millisecond,
		MaxRetries:      0,
		RetryDelay:      0,
		FollowRedirects: true,
		UserAgent:       "test",
		AllowHTTP:       false,
		MaxResponseSize: 1024,
		CustomTransport: server.Client().Transport,
	}

	result, err := verification.VerifyHTTPChallengeWithConfig(context.Background(), domain, token, config)

	if err == nil {
		t.Error("Expected timeout error but got none")
	} else {
		t.Logf("HTTP request failed as expected: %v", err)
		// Verify it's a context timeout or network error
		if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("Expected timeout-related error, got: %v", err)
		}
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.Duration == "" {
		t.Error("Expected duration to be populated")
	}

	t.Logf("Verification completed in: %s", result.Duration)
}

func TestDefaultHTTPConfig(t *testing.T) {
	config := verification.DefaultHTTPConfig()

	if config == nil {
		t.Fatal("DefaultHTTPConfig returned nil")
	}

	if config.Timeout <= 0 {
		t.Error("Default timeout should be positive")
	}

	if config.MaxRetries < 0 {
		t.Error("Default max retries should be non-negative")
	}

	if config.RetryDelay <= 0 {
		t.Error("Default retry delay should be positive")
	}

	if !config.FollowRedirects {
		t.Error("Default should follow redirects")
	}

	if config.UserAgent == "" {
		t.Error("Default should have user agent")
	}

	if config.AllowHTTP {
		t.Error("Default should not allow HTTP (HTTPS only)")
	}

	if config.MaxResponseSize <= 0 {
		t.Error("Default max response size should be positive")
	}

	t.Logf("Default HTTP config: %+v", config)
}

func TestHTTPVerificationError(t *testing.T) {
	// Test error without cause
	err1 := &verification.HTTPVerificationError{
		Domain:  testDomainHTTP,
		Token:   "test-token",
		URL:     "https://example.com/.well-known/mcp-challenge/test-token",
		Message: "test error",
	}

	expectedMsg1 := "HTTP verification failed for domain example.com: test error"
	if err1.Error() != expectedMsg1 {
		t.Errorf("Error() = %q, want %q", err1.Error(), expectedMsg1)
	}

	// Test error with cause
	cause := errors.New("network error")
	err2 := &verification.HTTPVerificationError{
		Domain:  testDomainHTTP,
		Token:   "test-token",
		URL:     "https://example.com/.well-known/mcp-challenge/test-token",
		Message: "request failed",
		Cause:   cause,
	}

	expectedMsg2 := "HTTP verification failed for domain example.com: request failed (cause: network error)"
	if err2.Error() != expectedMsg2 {
		t.Errorf("Error() = %q, want %q", err2.Error(), expectedMsg2)
	}

	// Test Unwrap
	if !errors.Is(err2, cause) {
		t.Errorf("Expected error to wrap cause, but errors.Is returned false")
	}
}

func TestIsRetryableHTTPError(t *testing.T) {
	testCases := []struct {
		name  string
		err   error
		retry bool
	}{
		{"nil error", nil, false},
		{"context timeout", context.DeadlineExceeded, true},
		{"validation error", &verification.HTTPVerificationError{Message: "validation failed"}, false},
		{"network error", &mockNetError{timeout: true, temporary: false}, true},
		{"non-retryable temporary network error (Temporary() deprecated)", &mockNetError{timeout: false, temporary: true}, false}, // Not retryable: Temporary() is deprecated
		{"permanent network error", &mockNetError{timeout: false, temporary: false}, false},
		{"unknown error", errors.New("unknown"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := verification.IsRetryableHTTPError(tc.err)
			if result != tc.retry {
				t.Errorf("IsRetryableHTTPError(%v) = %t, want %t", tc.err, result, tc.retry)
			}
		})
	}
}

// mockNetError implements net.Error for testing
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string {
	return "mock network error"
}

func (e *mockNetError) Timeout() bool {
	return e.timeout
}

func (e *mockNetError) Temporary() bool {
	return e.temporary
}
