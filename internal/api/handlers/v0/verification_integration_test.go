package v0_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v0 "github.com/modelcontextprotocol/registry/internal/api/handlers/v0"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/modelcontextprotocol/registry/internal/verification"
)

const (
	testDomain        = "example.com"
	testPath          = "/v0/verify-domain"
	testAuth          = "Bearer test-token"
	testContentType   = "application/json"
	headerContentType = "Content-Type"
	headerAuth        = "Authorization"
)

// mockAuthService for testing
type mockAuthService struct{}

func (m *mockAuthService) StartAuthFlow(ctx context.Context, method model.AuthMethod, repoRef string) (map[string]string, string, error) {
	return map[string]string{"url": "mock-auth-url"}, "status-token", nil
}

func (m *mockAuthService) CheckAuthStatus(ctx context.Context, statusToken string) (string, error) {
	return "auth-token", nil
}

func (m *mockAuthService) ValidateAuth(ctx context.Context, auth model.Authentication) (bool, error) {
	return true, nil // Always return valid for tests
}

func TestHTTPVerificationTokenGeneration(t *testing.T) {
	authService := &mockAuthService{}
	domainVerifier := verification.NewDomainVerifier(verification.WithInsecureSkipVerify())
	handler := v0.DomainVerificationHandler(authService, domainVerifier)

	requestBody := v0.DomainVerificationRequest{
		Domain: testDomain,
		Method: verification.MethodHTTP,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, testPath, bytes.NewReader(body))
	req.Header.Set(headerContentType, testContentType)
	req.Header.Set(headerAuth, testAuth)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response v0.DomainVerificationResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Domain != testDomain {
		t.Errorf("Expected domain '%s', got '%s'", testDomain, response.Domain)
	}

	if response.Method != verification.MethodHTTP {
		t.Errorf("Expected method '%s', got '%s'", verification.MethodHTTP, response.Method)
	}

	if response.Token == "" {
		t.Error("Expected non-empty token")
	}

	if response.Instructions == nil || response.Instructions.HTTPChallenge == nil {
		t.Error("Expected HTTP challenge instructions")
	}
}

func TestDNSVerificationTokenGeneration(t *testing.T) {
	authService := &mockAuthService{}
	domainVerifier := verification.NewDomainVerifier(verification.WithInsecureSkipVerify())
	handler := v0.DomainVerificationHandler(authService, domainVerifier)

	requestBody := v0.DomainVerificationRequest{
		Domain: testDomain,
		Method: verification.MethodDNS,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, testPath, bytes.NewReader(body))
	req.Header.Set(headerContentType, testContentType)
	req.Header.Set(headerAuth, testAuth)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response v0.DomainVerificationResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Instructions == nil || response.Instructions.DNSRecord == nil {
		t.Error("Expected DNS record instructions")
	}
}

func TestVerificationWithMissingDomain(t *testing.T) {
	authService := &mockAuthService{}
	domainVerifier := verification.NewDomainVerifier(verification.WithInsecureSkipVerify())
	handler := v0.DomainVerificationHandler(authService, domainVerifier)

	requestBody := v0.DomainVerificationRequest{
		Method: verification.MethodHTTP,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, testPath, bytes.NewReader(body))
	req.Header.Set(headerContentType, testContentType)
	req.Header.Set(headerAuth, testAuth)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Domain is required") {
		t.Errorf("Expected error message about domain being required, got: %s", w.Body.String())
	}
}

func TestHTTPVerificationWithMockServer(t *testing.T) {
	authService := &mockAuthService{}
	domainVerifier := verification.NewDomainVerifier(verification.WithInsecureSkipVerify())
	handler := v0.DomainVerificationHandler(authService, domainVerifier)

	// Generate a token for testing
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create a mock server that serves the token
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/mcp-challenge/"+token, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, token)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	domain := strings.TrimPrefix(server.URL, "https://")

	// Test the verification check endpoint
	url := fmt.Sprintf("%s?domain=%s&token=%s&method=http", testPath, domain, token)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set(headerAuth, testAuth)

	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d. Response: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response v0.DomainVerificationResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected verification success, got failure: %s", response.Error)
	}
}
