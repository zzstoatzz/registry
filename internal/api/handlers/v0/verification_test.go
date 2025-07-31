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
	testDomain       = "example.com"
	testPath         = "/v0/verify-domain"
	testAuth         = "Bearer test-token"
	testContentType  = "application/json"
	headerContentType = "Content-Type"
	headerAuth       = "Authorization"
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

	t.Run("POST - Generate Verification Token", func(t *testing.T) {
		requestBody := v0.DomainVerificationRequest{
			Domain: "example.com",
			Method: verification.MethodHTTP,
		}

		body, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v0/verify-domain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

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

		if response.Domain != "example.com" {
			t.Errorf("Expected domain 'example.com', got '%s'", response.Domain)
		}

		if response.Method != verification.MethodHTTP {
			t.Errorf("Expected method 'http', got '%s'", response.Method)
		}

		if response.Token == "" {
			t.Error("Expected non-empty token")
		}

		if response.Instructions == nil || response.Instructions.HTTPChallenge == nil {
			t.Error("Expected HTTP challenge instructions")
		} else {
			expectedURL := fmt.Sprintf("https://example.com/.well-known/mcp-challenge/%s", response.Token)
			if response.Instructions.HTTPChallenge.URL != expectedURL {
				t.Errorf("Expected URL '%s', got '%s'", expectedURL, response.Instructions.HTTPChallenge.URL)
			}

			if response.Instructions.HTTPChallenge.Content != response.Token {
				t.Errorf("Expected content to be token '%s', got '%s'", response.Token, response.Instructions.HTTPChallenge.Content)
			}
		}
	})

	t.Run("POST - DNS Verification Instructions", func(t *testing.T) {
		requestBody := v0.DomainVerificationRequest{
			Domain: "example.com",
			Method: verification.MethodDNS,
		}

		body, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v0/verify-domain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

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
		} else {
			expectedRecord := fmt.Sprintf("mcp-verify=%s", response.Token)
			if response.Instructions.DNSRecord.Record != expectedRecord {
				t.Errorf("Expected record '%s', got '%s'", expectedRecord, response.Instructions.DNSRecord.Record)
			}

			if response.Instructions.DNSRecord.Value != response.Token {
				t.Errorf("Expected DNS value to be token '%s', got '%s'", response.Token, response.Instructions.DNSRecord.Value)
			}
		}
	})

	t.Run("POST - Missing Domain", func(t *testing.T) {
		requestBody := v0.DomainVerificationRequest{
			Method: verification.MethodHTTP,
		}

		body, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v0/verify-domain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}

		if !strings.Contains(w.Body.String(), "Domain is required") {
			t.Errorf("Expected error message about domain being required, got: %s", w.Body.String())
		}
	})

	t.Run("POST - Invalid Method", func(t *testing.T) {
		requestBody := v0.DomainVerificationRequest{
			Domain: "example.com",
			Method: "invalid",
		}

		body, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v0/verify-domain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}

		if !strings.Contains(w.Body.String(), "Invalid verification method") {
			t.Errorf("Expected error message about invalid method, got: %s", w.Body.String())
		}
	})

	t.Run("GET - HTTP Verification with Mock Server", func(t *testing.T) {
		// Generate a token for testing
		token, err := verification.GenerateVerificationToken()
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		// Create a mock server that serves the token
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/mcp-challenge/"+token, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, token)
		})

		server := httptest.NewTLSServer(mux)
		defer server.Close()

		domain := strings.TrimPrefix(server.URL, "https://")

		// Test the verification check endpoint
		url := fmt.Sprintf("/v0/verify-domain?domain=%s&token=%s&method=http", domain, token)
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer test-token")

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

		if response.Domain != domain {
			t.Errorf("Expected domain '%s', got '%s'", domain, response.Domain)
		}

		if response.Token != token {
			t.Errorf("Expected token '%s', got '%s'", token, response.Token)
		}
	})

	t.Run("GET - Missing Parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v0/verify-domain", nil)
		req.Header.Set("Authorization", "Bearer test-token")

		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}

		if !strings.Contains(w.Body.String(), "Domain parameter is required") {
			t.Errorf("Expected error about missing domain parameter, got: %s", w.Body.String())
		}
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		requestBody := v0.DomainVerificationRequest{
			Domain: "example.com",
			Method: verification.MethodHTTP,
		}

		body, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v0/verify-domain", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// No Authorization header

		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})
}
