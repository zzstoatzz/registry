package verification_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/registry/internal/verification"
)

func TestNewDomainVerifier(t *testing.T) {
	verifier := verification.NewDomainVerifier()
	if verifier == nil {
		t.Error("NewDomainVerifier() returned nil")
	}
}

func TestDomainVerifierHTTPSuccess(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/mcp-challenge/"+token, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, token)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, "https://")

	verifier := verification.NewDomainVerifier(verification.WithInsecureSkipVerify())
	ctx := context.Background()

	result := verifier.VerifyDomain(ctx, domain, token, verification.MethodHTTP)

	if result == nil {
		t.Fatal("VerifyDomain() returned nil result")
	}

	if !result.Success {
		t.Errorf("VerifyDomain() expected success, got failure: %s", result.Error)
	}

	if result.Domain != domain {
		t.Errorf("VerifyDomain() domain = %s, want %s", result.Domain, domain)
	}

	if result.Token != token {
		t.Errorf("VerifyDomain() token = %s, want %s", result.Token, token)
	}

	if result.Method != verification.MethodHTTP {
		t.Errorf("VerifyDomain() method = %s, want %s", result.Method, verification.MethodHTTP)
	}
}

func TestDomainVerifierHTTPFailure(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	wrongToken, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate wrong token: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/mcp-challenge/"+token, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, wrongToken) // Serve wrong token
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, "https://")

	verifier := verification.NewDomainVerifier(verification.WithInsecureSkipVerify())
	ctx := context.Background()

	result := verifier.VerifyDomain(ctx, domain, token, verification.MethodHTTP)

	if result == nil {
		t.Fatal("VerifyDomain() returned nil result")
	}

	if result.Success {
		t.Error("VerifyDomain() expected failure, got success")
	}

	if result.Error == "" {
		t.Error("VerifyDomain() expected error message, got empty string")
	}

	expectedError := "token mismatch"
	if !strings.Contains(result.Error, expectedError) {
		t.Errorf("Error should contain '%s', got: %s", expectedError, result.Error)
	}
}

func TestDomainVerifierDNSNotImplemented(t *testing.T) {
	verifier := verification.NewDomainVerifier()
	ctx := context.Background()

	result := verifier.VerifyDomain(ctx, "example.com", "testtoken", verification.MethodDNS)

	if result == nil {
		t.Fatal("VerifyDomain() returned nil result")
	}

	if result.Success {
		t.Error("VerifyDomain() expected failure for unimplemented DNS method, got success")
	}

	expectedError := "DNS verification not yet implemented"
	if !strings.Contains(result.Error, expectedError) {
		t.Errorf("Error should contain '%s', got: %s", expectedError, result.Error)
	}

	if result.Method != verification.MethodDNS {
		t.Errorf("VerifyDomain() method = %s, want %s", result.Method, verification.MethodDNS)
	}
}

func TestDomainVerifierUnsupportedMethod(t *testing.T) {
	verifier := verification.NewDomainVerifier()
	ctx := context.Background()

	unsupportedMethod := verification.VerificationMethod("unsupported")
	result := verifier.VerifyDomain(ctx, "example.com", "testtoken", unsupportedMethod)

	if result == nil {
		t.Fatal("VerifyDomain() returned nil result")
	}

	if result.Success {
		t.Error("VerifyDomain() expected failure for unsupported method, got success")
	}

	expectedError := "unsupported verification method"
	if !strings.Contains(result.Error, expectedError) {
		t.Errorf("Error should contain '%s', got: %s", expectedError, result.Error)
	}
}

func TestDomainVerifierWithRetrySuccess(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/mcp-challenge/"+token, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Fail first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on third attempt
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, token)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, "https://")

	verifier := verification.NewDomainVerifier(verification.WithInsecureSkipVerify())
	ctx := context.Background()

	result := verifier.VerifyDomainWithRetry(ctx, domain, token, verification.MethodHTTP, 3)

	if result == nil {
		t.Fatal("VerifyDomainWithRetry() returned nil result")
	}

	if !result.Success {
		t.Errorf("VerifyDomainWithRetry() expected success after retries, got failure: %s", result.Error)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestIsVerificationSuccessful(t *testing.T) {
	testCases := []struct {
		name       string
		httpResult *verification.VerificationResult
		dnsResult  *verification.VerificationResult
		expected   bool
	}{
		{
			name:       "Both successful",
			httpResult: &verification.VerificationResult{Success: true},
			dnsResult:  &verification.VerificationResult{Success: true},
			expected:   true,
		},
		{
			name:       "HTTP successful, DNS failed",
			httpResult: &verification.VerificationResult{Success: true},
			dnsResult:  &verification.VerificationResult{Success: false},
			expected:   true,
		},
		{
			name:       "HTTP failed, DNS successful",
			httpResult: &verification.VerificationResult{Success: false},
			dnsResult:  &verification.VerificationResult{Success: true},
			expected:   true,
		},
		{
			name:       "Both failed",
			httpResult: &verification.VerificationResult{Success: false},
			dnsResult:  &verification.VerificationResult{Success: false},
			expected:   false,
		},
		{
			name:       "HTTP nil, DNS successful",
			httpResult: nil,
			dnsResult:  &verification.VerificationResult{Success: true},
			expected:   true,
		},
		{
			name:       "HTTP successful, DNS nil",
			httpResult: &verification.VerificationResult{Success: true},
			dnsResult:  nil,
			expected:   true,
		},
		{
			name:       "Both nil",
			httpResult: nil,
			dnsResult:  nil,
			expected:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := verification.IsVerificationSuccessful(tc.httpResult, tc.dnsResult)
			if result != tc.expected {
				t.Errorf("IsVerificationSuccessful() = %v, want %v", result, tc.expected)
			}
		})
	}
}
