package v0

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/modelcontextprotocol/registry/internal/verification"
)

// DomainVerificationRequest represents a request to verify domain ownership
type DomainVerificationRequest struct {
	Domain string                          `json:"domain"`
	Method verification.VerificationMethod `json:"method,omitempty"` // Optional, defaults to HTTP
}

// DomainVerificationResponse represents the response from domain verification
type DomainVerificationResponse struct {
	Domain    string                          `json:"domain"`
	Token     string                          `json:"token"`
	Method    verification.VerificationMethod `json:"method"`
	Success   bool                            `json:"success"`
	Error     string                          `json:"error,omitempty"`
	Timestamp string                          `json:"timestamp"`

	// Instructions for the user
	Instructions *VerificationInstructions `json:"instructions,omitempty"`
}

// VerificationInstructions provides user-friendly instructions for domain verification
type VerificationInstructions struct {
	HTTPChallenge *HTTPChallengeInstructions `json:"http_challenge,omitempty"`
	DNSRecord     *DNSRecordInstructions     `json:"dns_record,omitempty"`
}

// HTTPChallengeInstructions provides instructions for HTTP-01 verification
type HTTPChallengeInstructions struct {
	URL     string `json:"url"`
	Content string `json:"content"`
	Message string `json:"message"`
}

// DNSRecordInstructions provides instructions for DNS TXT record verification
type DNSRecordInstructions struct {
	Record  string `json:"record"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// DomainVerificationHandler handles domain verification requests
func DomainVerificationHandler(authService auth.Service, domainVerifier *verification.DomainVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleDomainVerificationRequest(w, r, authService, domainVerifier)
		case http.MethodGet:
			handleDomainVerificationCheck(w, r, authService, domainVerifier)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDomainVerificationRequest initiates domain verification and returns a token
func handleDomainVerificationRequest(w http.ResponseWriter, r *http.Request, authService auth.Service, domainVerifier *verification.DomainVerifier) {
	// Check authentication
	if !isAuthenticated(r, authService) {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse request
	var req DomainVerificationRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate domain
	if req.Domain == "" {
		http.Error(w, "Domain is required", http.StatusBadRequest)
		return
	}

	// Default to HTTP method if not specified
	if req.Method == "" {
		req.Method = verification.MethodHTTP
	}

	// Validate method
	if req.Method != verification.MethodHTTP && req.Method != verification.MethodDNS {
		http.Error(w, "Invalid verification method. Supported methods: http, dns", http.StatusBadRequest)
		return
	}

	// Generate verification token
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		http.Error(w, "Failed to generate verification token", http.StatusInternalServerError)
		return
	}

	// Create response with instructions
	response := DomainVerificationResponse{
		Domain:       req.Domain,
		Token:        token,
		Method:       req.Method,
		Success:      false, // Not verified yet, just token generated
		Timestamp:    "pending",
		Instructions: &VerificationInstructions{},
	}

	// Add method-specific instructions
	switch req.Method {
	case verification.MethodHTTP:
		response.Instructions.HTTPChallenge = &HTTPChallengeInstructions{
			URL:     fmt.Sprintf("https://%s/.well-known/mcp-challenge/%s", req.Domain, token),
			Content: token,
			Message: fmt.Sprintf("Host a file at the URL above containing the token '%s' as plain text", token),
		}
	case verification.MethodDNS:
		response.Instructions.DNSRecord = &DNSRecordInstructions{
			Record:  fmt.Sprintf("mcp-verify=%s", token),
			Value:   token,
			Message: fmt.Sprintf("Add a TXT record 'mcp-verify=%s' to your domain's DNS", token),
		}
	}

	// TODO: Store the token and domain association in a database for later verification
	// For now, we just return the instructions

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleDomainVerificationCheck verifies the domain ownership using the provided token
func handleDomainVerificationCheck(w http.ResponseWriter, r *http.Request, authService auth.Service, domainVerifier *verification.DomainVerifier) {
	// Check authentication
	if !isAuthenticated(r, authService) {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get domain and token from query parameters
	domain := r.URL.Query().Get("domain")
	token := r.URL.Query().Get("token")
	method := r.URL.Query().Get("method")

	if domain == "" {
		http.Error(w, "Domain parameter is required", http.StatusBadRequest)
		return
	}

	if token == "" {
		http.Error(w, "Token parameter is required", http.StatusBadRequest)
		return
	}

	// Default to HTTP method if not specified
	if method == "" {
		method = string(verification.MethodHTTP)
	}

	verificationMethod := verification.VerificationMethod(method)
	if verificationMethod != verification.MethodHTTP && verificationMethod != verification.MethodDNS {
		http.Error(w, "Invalid verification method. Supported methods: http, dns", http.StatusBadRequest)
		return
	}

	// Perform verification
	result := domainVerifier.VerifyDomain(r.Context(), domain, token, verificationMethod)

	// Convert result to response format
	response := DomainVerificationResponse{
		Domain:    result.Domain,
		Token:     result.Token,
		Method:    result.Method,
		Success:   result.Success,
		Timestamp: result.Timestamp.Format("2006-01-02T15:04:05Z"),
	}

	if !result.Success {
		response.Error = result.Error
	}

	// Set appropriate status code
	statusCode := http.StatusOK
	if !result.Success {
		statusCode = http.StatusBadRequest
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// isAuthenticated checks if the request has valid authentication
func isAuthenticated(r *http.Request, authService auth.Service) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// Handle bearer token format
	token := authHeader
	if len(authHeader) > 7 && strings.ToUpper(authHeader[:7]) == "BEARER " {
		token = authHeader[7:]
	}

	// For now, we'll use GitHub auth as the primary method
	// In the future, this could be extended to support multiple auth methods
	auth := model.Authentication{
		Method: model.AuthMethodGitHub,
		Token:  token,
	}

	valid, err := authService.ValidateAuth(r.Context(), auth)
	return err == nil && valid
}
