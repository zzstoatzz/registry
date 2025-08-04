package verification

import (
	"strings"
)

// ValidationError represents a validation error that can be used by both DNS and HTTP verification
type ValidationError struct {
	Domain  string
	Token   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateVerificationInputs performs common validation for both DNS and HTTP verification
// Returns the normalized domain and any validation error
func ValidateVerificationInputs(domain, token string) (string, error) {
	// Input validation
	if domain == "" {
		return "", &ValidationError{
			Domain:  domain,
			Token:   token,
			Message: "domain cannot be empty",
		}
	}

	if token == "" {
		return "", &ValidationError{
			Domain:  domain,
			Token:   token,
			Message: "token cannot be empty",
		}
	}

	// Validate token format
	if !ValidateTokenFormat(token) {
		return "", &ValidationError{
			Domain:  domain,
			Token:   token,
			Message: "invalid token format",
		}
	}

	// Normalize domain (remove trailing dots, convert to lowercase)
	normalizedDomain := strings.ToLower(strings.TrimSuffix(domain, "."))

	return normalizedDomain, nil
}
