package verification

import (
	"context"
	"fmt"
	"time"
)

// VerificationMethod represents the type of verification method used
type VerificationMethod string

const (
	// MethodDNS represents DNS TXT record verification
	MethodDNS VerificationMethod = "dns"
	// MethodHTTP represents HTTP-01 well-known URL verification
	MethodHTTP VerificationMethod = "http"
)

// VerificationResult represents the result of a domain verification attempt
type VerificationResult struct {
	Domain    string             `json:"domain"`
	Token     string             `json:"token"`
	Method    VerificationMethod `json:"method"`
	Success   bool               `json:"success"`
	Error     string             `json:"error,omitempty"`
	Timestamp time.Time          `json:"timestamp"`
}

// DomainVerifier provides unified domain verification using multiple methods
type DomainVerifier struct {
	httpVerifier *HTTPVerifier
	// Future: dnsVerifier *DNSVerifier when DNS verification is implemented
}

// NewDomainVerifier creates a new domain verifier with both HTTP and DNS capabilities
func NewDomainVerifier(opts ...HTTPVerifierOption) *DomainVerifier {
	return &DomainVerifier{
		httpVerifier: NewHTTPVerifier(opts...),
	}
}

// VerifyDomain attempts to verify domain ownership using the specified method
func (dv *DomainVerifier) VerifyDomain(ctx context.Context, domain, token string, method VerificationMethod) *VerificationResult {
	result := &VerificationResult{
		Domain:    domain,
		Token:     token,
		Method:    method,
		Timestamp: time.Now(),
	}

	var err error
	switch method {
	case MethodHTTP:
		err = dv.httpVerifier.VerifyDomainHTTP(ctx, domain, token)
	case MethodDNS:
		// TODO: Implement DNS verification
		err = fmt.Errorf("DNS verification not yet implemented")
	default:
		err = fmt.Errorf("unsupported verification method: %s", method)
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	return result
}

// VerifyDomainDual attempts to verify domain ownership using both DNS and HTTP methods
// Returns success if either method succeeds (as per the design specification)
func (dv *DomainVerifier) VerifyDomainDual(ctx context.Context, domain, token string) (*VerificationResult, *VerificationResult) {
	// Create contexts for parallel verification
	httpCtx, httpCancel := context.WithTimeout(ctx, 15*time.Second)
	defer httpCancel()

	dnsCtx, dnsCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dnsCancel()

	// Channel to collect results
	httpResult := make(chan *VerificationResult, 1)
	dnsResult := make(chan *VerificationResult, 1)

	// Run HTTP verification
	go func() {
		httpResult <- dv.VerifyDomain(httpCtx, domain, token, MethodHTTP)
	}()

	// Run DNS verification
	go func() {
		dnsResult <- dv.VerifyDomain(dnsCtx, domain, token, MethodDNS)
	}()

	// Wait for both results
	httpRes := <-httpResult
	dnsRes := <-dnsResult

	return httpRes, dnsRes
}

// VerifyDomainWithRetry verifies domain ownership with retry logic for transient failures
func (dv *DomainVerifier) VerifyDomainWithRetry(ctx context.Context, domain, token string, method VerificationMethod, maxRetries int) *VerificationResult {
	var lastResult *VerificationResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result := dv.VerifyDomain(ctx, domain, token, method)

		if result.Success {
			return result
		}

		lastResult = result

		// Don't retry for certain types of errors (validation, auth, etc.)
		if method == MethodHTTP && isNonRetryableError(fmt.Errorf(result.Error)) {
			return result
		}

		// If this wasn't the last attempt, wait before retrying
		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s...
			backoffDuration := time.Duration(1<<attempt) * time.Second
			select {
			case <-ctx.Done():
				result.Error = ctx.Err().Error()
				return result
			case <-time.After(backoffDuration):
				// Continue to next attempt
			}
		}
	}

	// Update the final result to indicate retry exhaustion
	if lastResult != nil {
		lastResult.Error = fmt.Sprintf("verification failed after %d attempts: %s", maxRetries+1, lastResult.Error)
	}

	return lastResult
}

// IsVerificationSuccessful checks if at least one verification method succeeded
// This implements the "either method passing = success" policy from the design
func IsVerificationSuccessful(httpResult, dnsResult *VerificationResult) bool {
	return (httpResult != nil && httpResult.Success) || (dnsResult != nil && dnsResult.Success)
}
