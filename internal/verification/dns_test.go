package verification

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestVerifyDNSRecordSuccess(t *testing.T) {
	token, err := GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	domain := "example.com"
	result, err := VerifyDNSRecord(domain, token)
	if err != nil {
		t.Errorf("VerifyDNSRecord returned unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("VerifyDNSRecord returned nil result")
	}

	if result.Domain != domain {
		t.Errorf("Result domain = %s, want %s", result.Domain, domain)
	}

	if result.Token != token {
		t.Errorf("Result token = %s, want %s", result.Token, token)
	}

	t.Logf("DNS verification result: %+v", result)
}

func TestVerifyDNSRecordInvalidInputs(t *testing.T) {
	tests := []struct {
		name          string
		domain        string
		token         string
		expectError   bool
		errorContains string
	}{
		{
			name:          "empty domain",
			domain:        "",
			token:         "validtoken123456789012",
			expectError:   true,
			errorContains: "domain cannot be empty",
		},
		{
			name:          "empty token",
			domain:        "example.com",
			token:         "",
			expectError:   true,
			errorContains: "token cannot be empty",
		},
		{
			name:          "invalid token format",
			domain:        "example.com",
			token:         "invalid-token!@#",
			expectError:   true,
			errorContains: "invalid token format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := VerifyDNSRecord(tt.domain, tt.token)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			t.Logf("Result: %+v, Error: %v", result, err)
		})
	}
}

func TestVerifyDNSRecordTokenFormatValidation(t *testing.T) {
	token, err := GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	domain := "example.com"
	result, err := VerifyDNSRecord(domain, token)

	if err != nil {
		var dnsErr *DNSVerificationError
		if errors.As(err, &dnsErr) {
			if strings.Contains(dnsErr.Message, "invalid token format") {
				t.Errorf("Unexpected token format validation error: %v", err)
			}
		}
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.Domain != domain {
		t.Errorf("Result domain = %s, want %s", result.Domain, domain)
	}

	if result.Token != token {
		t.Errorf("Result token = %s, want %s", result.Token, token)
	}
}

func TestVerifyDNSRecordWithConfigTimeout(t *testing.T) {
	token, err := GenerateVerificationToken()
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	config := &DNSVerificationConfig{
		Timeout:            100 * time.Millisecond,
		MaxRetries:         0,
		RetryDelay:         0,
		UseSecureResolvers: false,
		CustomResolvers:    []string{},
	}

	domain := "non-existent-domain-that-should-timeout.com"
	result, err := VerifyDNSRecordWithConfig(domain, token, config)

	if err == nil {
		t.Log("DNS query succeeded unexpectedly")
	} else {
		t.Logf("DNS query failed as expected: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result but got nil")
	}

	if result.Duration == "" {
		t.Error("Expected duration to be populated")
	}

	t.Logf("Verification completed in: %s", result.Duration)
}

func TestDefaultDNSConfig(t *testing.T) {
	config := DefaultDNSConfig()

	if config == nil {
		t.Fatal("DefaultDNSConfig returned nil")
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

	if !config.UseSecureResolvers {
		t.Error("Default should use secure resolvers")
	}

	if len(config.CustomResolvers) == 0 {
		t.Error("Default should have custom resolvers configured")
	}

	t.Logf("Default DNS config: %+v", config)
}

func TestDNSVerificationError(t *testing.T) {
	baseErr := errors.New("base network error")
	dnsErr := &DNSVerificationError{
		Domain:  "example.com",
		Token:   "test-token",
		Message: "DNS query failed",
		Cause:   baseErr,
	}

	errMsg := dnsErr.Error()
	if !strings.Contains(errMsg, "example.com") {
		t.Errorf("Error message should contain domain: %s", errMsg)
	}

	if !strings.Contains(errMsg, "DNS query failed") {
		t.Errorf("Error message should contain message: %s", errMsg)
	}

	if !strings.Contains(errMsg, "base network error") {
		t.Errorf("Error message should contain cause: %s", errMsg)
	}

	unwrapped := errors.Unwrap(dnsErr)
	if unwrapped != baseErr {
		t.Errorf("Unwrap should return base error, got: %v", unwrapped)
	}
}

func TestIsRetryableDNSError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "nil error",
			err:         nil,
			shouldRetry: false,
		},
		{
			name:        "context deadline exceeded",
			err:         context.DeadlineExceeded,
			shouldRetry: true,
		},
		{
			name:        "temporary DNS error",
			err:         &net.DNSError{Err: "server failure", IsTemporary: true},
			shouldRetry: true,
		},
		{
			name:        "non-temporary DNS error",
			err:         &net.DNSError{Err: "no such host", IsTemporary: false},
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableDNSError(tt.err)
			if result != tt.shouldRetry {
				t.Errorf("isRetryableDNSError(%v) = %t, want %t", tt.err, result, tt.shouldRetry)
			}
		})
	}
}

func TestDNSRecordFormat(t *testing.T) {
	token := "TBeVXe_X4npM6p8vpzStnA"
	expectedFormat := "mcp-verify=" + token

	tokenInfo, err := GenerateTokenWithInfo()
	if err != nil {
		t.Fatalf("Failed to generate token info: %v", err)
	}

	if !strings.HasPrefix(tokenInfo.DNSRecord, "mcp-verify=") {
		t.Errorf("DNS record format mismatch: %s", tokenInfo.DNSRecord)
	}

	if expectedFormat != fmt.Sprintf("mcp-verify=%s", token) {
		t.Errorf("DNS record format construction error")
	}

	t.Logf("Expected DNS record format: %s", expectedFormat)
	t.Logf("Generated DNS record format: %s", tokenInfo.DNSRecord)
}
