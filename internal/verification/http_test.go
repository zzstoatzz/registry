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

func TestNewHTTPVerifier(t *testing.T) {
	verifier := verification.NewHTTPVerifier()
	if verifier == nil {
		t.Error("NewHTTPVerifier() returned nil")
	}
}

func TestHTTPVerifierSuccess(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/mcp-challenge/"+token, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, token)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, "https://")

	verifier := verification.NewHTTPVerifier(verification.WithInsecureSkipVerify())
	ctx := context.Background()

	err = verifier.VerifyDomainHTTP(ctx, domain, token)
	if err != nil {
		t.Errorf("VerifyDomainHTTP() error = %v, want nil", err)
	}
}

func TestHTTPVerifierEmptyDomain(t *testing.T) {
	verifier := verification.NewHTTPVerifier()
	ctx := context.Background()

	err := verifier.VerifyDomainHTTP(ctx, "", "sometoken")
	if err == nil {
		t.Error("VerifyDomainHTTP() expected error for empty domain, got nil")
	}

	expectedError := "domain cannot be empty"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Error should contain '%s', got: %v", expectedError, err)
	}
}

func TestHTTPVerifierTokenMismatch(t *testing.T) {
	correctToken, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate correct token: %v", err)
	}

	wrongToken, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate wrong token: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/mcp-challenge/"+correctToken, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, wrongToken) // Serve wrong token
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, "https://")

	verifier := verification.NewHTTPVerifier(verification.WithInsecureSkipVerify())
	ctx := context.Background()

	err = verifier.VerifyDomainHTTP(ctx, domain, correctToken)
	if err == nil {
		t.Error("VerifyDomainHTTP() expected error for token mismatch, got nil")
	}

	expectedError := "token mismatch"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Error should contain '%s', got: %v", expectedError, err)
	}
}
