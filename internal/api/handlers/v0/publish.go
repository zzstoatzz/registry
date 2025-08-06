// Package v0 contains API handlers for version 0 of the API
package v0

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/modelcontextprotocol/registry/internal/namespace"
	"github.com/modelcontextprotocol/registry/internal/service"
	"github.com/modelcontextprotocol/registry/internal/verification"
	"golang.org/x/net/html"
)

// PublishHandler handles requests to publish new server details to the registry
func PublishHandler(registry service.RegistryService, authService auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow POST method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse and validate request
		serverDetail, err := parseAndValidateRequest(r)
		if err != nil {
			handleRequestError(w, err)
			return
		}

		// Extract token from authorization header
		token, err := extractAuthToken(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// Determine auth method and perform domain verification if needed
		authMethod, err := determineAuthMethodAndVerifyDomain(r.Context(), serverDetail.Name, registry)
		if err != nil {
			handleDomainVerificationError(w, err)
			return
		}

		// Authenticate user
		if err := authenticateUser(r.Context(), authService, authMethod, token, serverDetail.Name); err != nil {
			handleAuthError(w, err)
			return
		}

		// Publish server
		if err := registry.Publish(serverDetail); err != nil {
			handlePublishError(w, err)
			return
		}

		// Send success response
		sendSuccessResponse(w, serverDetail.ID)
	}
}

// parseAndValidateRequest parses the request body and validates required fields
func parseAndValidateRequest(r *http.Request) (*model.ServerDetail, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading request body")
	}
	defer r.Body.Close()

	// Parse request body - try PublishRequest first, then ServerDetail for backward compatibility
	var publishReq model.PublishRequest
	err = json.Unmarshal(body, &publishReq)
	if err != nil {
		return nil, fmt.Errorf("invalid request payload: %w", err)
	}

	var serverDetail model.ServerDetail
	err = json.Unmarshal(body, &serverDetail)
	if err != nil {
		return nil, fmt.Errorf("invalid server detail payload: %w", err)
	}

	// Validate required fields
	if serverDetail.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Validate namespace format if it follows domain-scoped convention
	if err := namespace.ValidateNamespace(serverDetail.Name); err != nil {
		if !errors.Is(err, namespace.ErrInvalidNamespace) {
			return nil, fmt.Errorf("invalid namespace: %w", err)
		}
	}

	if serverDetail.VersionDetail.Version == "" {
		return nil, fmt.Errorf("version is required")
	}

	return &serverDetail, nil
}

// extractAuthToken extracts and parses the authorization token from the request
func extractAuthToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is required")
	}

	// Handle bearer token format (e.g., "Bearer xyz123")
	token := authHeader
	if len(authHeader) > 7 && strings.ToUpper(authHeader[:7]) == "BEARER " {
		token = authHeader[7:]
	}

	return token, nil
}

// determineAuthMethodAndVerifyDomain determines the auth method and performs domain verification if needed
func determineAuthMethodAndVerifyDomain(ctx context.Context, serverName string, registry service.RegistryService) (model.AuthMethod, error) {
	// Check if the namespace is domain-scoped and extract domain for auth
	if parsed, err := namespace.ParseNamespace(serverName); err == nil {
		// For domain-scoped namespaces, perform real-time domain verification
		if err := performDomainVerification(ctx, parsed.Domain, registry.GetDatabase()); err != nil {
			return model.AuthMethodNone, err
		}

		// Determine auth method based on domain
		// For now, all domain-scoped namespaces require GitHub auth
		return model.AuthMethodGitHub, nil
	}

	// Legacy namespace format - use existing logic
	switch {
	case strings.HasPrefix(serverName, "io.github"):
		return model.AuthMethodGitHub, nil
	default:
		return model.AuthMethodNone, nil
	}
}

// authenticateUser performs user authentication
func authenticateUser(ctx context.Context, authService auth.Service, authMethod model.AuthMethod, token, serverName string) error {
	serverName = html.EscapeString(serverName)

	auth := model.Authentication{
		Method:  authMethod,
		Token:   token,
		RepoRef: serverName,
	}

	valid, err := authService.ValidateAuth(ctx, auth)
	if err != nil {
		return err
	}

	if !valid {
		return fmt.Errorf("invalid authentication credentials")
	}

	return nil
}

// Helper functions for error handling
func handleRequestError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func handleDomainVerificationError(w http.ResponseWriter, err error) {
	var domainErr *DomainVerificationError
	if errors.As(err, &domainErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		if encodeErr := json.NewEncoder(w).Encode(domainErr.ToAPIResponse()); encodeErr != nil {
			http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
		}
		return
	}
	http.Error(w, "Domain verification failed: "+err.Error(), http.StatusForbidden)
}

func handleAuthError(w http.ResponseWriter, err error) {
	if errors.Is(err, auth.ErrAuthRequired) {
		http.Error(w, "Authentication is required for publishing", http.StatusUnauthorized)
		return
	}
	http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
}

func handlePublishError(w http.ResponseWriter, err error) {
	if errors.Is(err, database.ErrInvalidVersion) || errors.Is(err, database.ErrAlreadyExists) {
		http.Error(w, "Failed to publish server details: "+err.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, "Failed to publish server details: "+err.Error(), http.StatusInternalServerError)
}

func sendSuccessResponse(w http.ResponseWriter, serverID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Server publication successful",
		"id":      serverID,
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// DomainVerificationError represents domain verification failures with structured guidance
type DomainVerificationError struct {
	Domain    string `json:"domain"`
	Message   string `json:"message"`
	Method    string `json:"method"` // "dns" or "http"
	Token     string `json:"token,omitempty"`
	DNSGuide  string `json:"dns_guide,omitempty"`
	HTTPGuide string `json:"http_guide,omitempty"`
}

func (e *DomainVerificationError) Error() string {
	return e.Message
}

// ToAPIResponse converts the error to a structured API response as specified in issue #22240
func (e *DomainVerificationError) ToAPIResponse() map[string]any {
	return map[string]any{
		"error":      "domain_verification_failed",
		"message":    e.Message,
		"domain":     e.Domain,
		"method":     e.Method,
		"token":      e.Token,
		"dns_guide":  e.DNSGuide,
		"http_guide": e.HTTPGuide,
	}
}

// performDomainVerification implements the real-time domain verification specified in issue #22240
// "Every publish immediately queries DNS and/or fetches the well-known file"
func performDomainVerification(ctx context.Context, domain string, db database.Database) error {
	// Skip domain verification in testing mode
	if os.Getenv("DISABLE_DOMAIN_VERIFICATION") == "true" {
		return nil
	}

	// Skip verification for certain test domains
	testDomains := []string{".github.io", ".test", ".example", ".invalid", ".local"}
	for _, testDomain := range testDomains {
		if strings.HasSuffix(domain, testDomain) || strings.Contains(domain, testDomain) {
			return nil
		}
	}

	// Create a timeout context for verification operations
	verifyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Retrieve stored verification tokens from database (production approach)
	// This matches the background job pattern in internal/verification/background_job.go
	domainVerification, err := db.GetDomainVerification(ctx, domain)
	if err != nil {
		return &DomainVerificationError{
			Domain:  domain,
			Message: "Failed to retrieve domain verification from database: " + err.Error(),
			Method:  "system",
		}
	}

	if domainVerification == nil {
		return &DomainVerificationError{
			Domain:  domain,
			Message: "No domain verification record found. Please register your domain first.",
			Method:  "system",
		}
	}

	// Use stored tokens from database (same approach as background job)
	dnsToken := domainVerification.DNSToken
	httpToken := domainVerification.HTTPToken

	if dnsToken == "" && httpToken == "" {
		return &DomainVerificationError{
			Domain:  domain,
			Message: "No verification tokens found for domain. Please set up domain verification.",
			Method:  "system",
		}
	}

	var dnsResult *verification.DNSVerificationResult
	var httpResult *verification.HTTPVerificationResult
	var dnsErr, httpErr error

	// Try DNS verification if DNS token exists
	if dnsToken != "" {
		dnsResult, dnsErr = verification.VerifyDNSRecordWithConfig(verifyCtx, domain, dnsToken, verification.DefaultDNSConfig())
	}

	// Try HTTP verification if HTTP token exists
	if httpToken != "" {
		httpResult, httpErr = verification.VerifyHTTPChallengeWithConfig(verifyCtx, domain, httpToken, verification.DefaultHTTPConfig())
	}

	// Implement the dual-method policy: allow publish if at least one method passes
	// This follows the design document's "either path marks the domain verified" approach
	dnsSuccess := dnsToken != "" && dnsErr == nil && dnsResult != nil && dnsResult.Success
	httpSuccess := httpToken != "" && httpErr == nil && httpResult != nil && httpResult.Success

	if dnsSuccess || httpSuccess {
		// At least one verification method succeeded
		return nil
	}

	// Both methods failed - return structured error with guidance
	return &DomainVerificationError{
		Domain:    domain,
		Message:   "Domain verification failed using stored tokens. Please ensure your domain verification is properly configured.",
		Method:    "both",
		Token:     "", // Don't expose tokens in error response
		DNSGuide:  "Check your DNS TXT record configuration",
		HTTPGuide: "Check your HTTP challenge endpoint configuration",
	}
}
