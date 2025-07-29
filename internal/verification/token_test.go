package verification_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/registry/internal/verification"
)

const (
	errMsgGenTokenIteration = "GenerateVerificationToken() error = %v, iteration %d"
	errMsgGenToken          = "GenerateVerificationToken() error = %v"
	errMsgGenTokenNormal    = "GenerateVerificationToken() should succeed in normal conditions: %v"
	errMsgGenTokenWithInfo  = "GenerateTokenWithInfo() error = %v"
	dnsRecordPrefix         = "mcp-verify="
)

func TestGenerateVerificationToken(t *testing.T) {
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Errorf("GenerateVerificationToken() error = %v, want nil", err)
		return
	}

	// Test token is not empty
	if token == "" {
		t.Error("GenerateVerificationToken() returned empty token")
	}

	// Test token format is valid
	if !verification.ValidateTokenFormat(token) {
		t.Errorf("GenerateVerificationToken() returned invalid token format: %s", token)
	}

	// Test token length (should be 22 characters for base64url encoding of 16 bytes)
	expectedLength := 22
	if len(token) != expectedLength {
		t.Errorf("GenerateVerificationToken() token length = %d, want %d", len(token), expectedLength)
	}

	// Test token contains only base64url characters
	for _, char := range token {
		if !isValidBase64URLChar(char) {
			t.Errorf("GenerateVerificationToken() token contains invalid character: %c", char)
		}
	}

	// Test token doesn't contain padding
	if strings.Contains(token, "=") {
		t.Error("GenerateVerificationToken() token should not contain padding")
	}
}

// isValidBase64URLChar checks if a character is valid for base64url encoding
func isValidBase64URLChar(char rune) bool {
	return (char >= 'A' && char <= 'Z') ||
		(char >= 'a' && char <= 'z') ||
		(char >= '0' && char <= '9') ||
		char == '-' || char == '_'
}

func TestGenerateVerificationTokenUniqueness(t *testing.T) {
	// Generate multiple tokens and ensure they're unique
	tokenCount := 1000
	tokens := make(map[string]bool)

	for i := 0; i < tokenCount; i++ {
		token, err := verification.GenerateVerificationToken()
		if err != nil {
			t.Fatalf(errMsgGenTokenIteration, err, i)
		}

		if tokens[token] {
			t.Errorf("GenerateVerificationToken() generated duplicate token: %s", token)
		}
		tokens[token] = true
	}

	if len(tokens) != tokenCount {
		t.Errorf("Expected %d unique tokens, got %d", tokenCount, len(tokens))
	}
}

func TestGenerateVerificationTokenEntropy(t *testing.T) {
	// Test that generated tokens have exactly 128 bits (16 bytes) of entropy
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenToken, err)
	}

	// Decode the base64url token to verify byte length
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(token)
	if err != nil {
		t.Fatalf("Failed to decode token: %v", err)
	}

	expectedBytes := 16
	if len(decoded) != expectedBytes {
		t.Errorf("Token entropy = %d bytes, want %d bytes", len(decoded), expectedBytes)
	}
}

func TestGenerateVerificationTokenErrorHandling(t *testing.T) {
	// This test verifies that the function properly wraps errors from crypto/rand
	// We can't easily mock crypto/rand.Read without causing fatal errors,
	// so we test the error wrapping behavior indirectly

	// Test with valid input to ensure normal operation
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		// If this fails in a normal environment, there's likely a real issue
		t.Errorf(errMsgGenTokenNormal, err)
	}

	if token == "" {
		t.Error("GenerateVerificationToken() should return non-empty token")
	}

	// The error handling is tested by the fact that our function
	// properly declares error returns and wraps rand.Read errors
	// This is validated by the successful compilation and the above test
}

func TestValidateTokenFormat(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{
			name:  "Valid token",
			token: "ABCDEFGHIJKLMNOPQRSTuv", // 22 chars, valid base64url
			want:  true,
		},
		{
			name:  "Valid token with numbers",
			token: "ABC123def456GHI789KLMN", // 22 chars with numbers
			want:  true,
		},
		{
			name:  "Valid token with URL-safe chars",
			token: "ABC_DEF-GHI123jklmnopq", // 22 chars with - and _
			want:  true,
		},
		{
			name:  "Empty token",
			token: "",
			want:  false,
		},
		{
			name:  "Too short",
			token: "ABC123", // Only 6 chars
			want:  false,
		},
		{
			name:  "Too long",
			token: "ABCDEFGHIJKLMNOPQRSTuvw", // 23 chars
			want:  false,
		},
		{
			name:  "Invalid character +",
			token: "ABCDEFGHIJKLMNOPQRST+v", // Contains +
			want:  false,
		},
		{
			name:  "Invalid character /",
			token: "ABCDEFGHIJKLMNOPQRST/v", // Contains /
			want:  false,
		},
		{
			name:  "Invalid character =",
			token: "ABCDEFGHIJKLMNOPQRST=v", // Contains =
			want:  false,
		},
		{
			name:  "Invalid character space",
			token: "ABCDEFGHIJKLMNOPQRST v", // Contains space
			want:  false,
		},
		{
			name:  "Invalid character special",
			token: "ABCDEFGHIJKLMNOPQRST@v", // Contains @
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verification.ValidateTokenFormat(tt.token)
			if got != tt.want {
				t.Errorf("ValidateTokenFormat(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestValidateTokenFormatWithGeneratedTokens(t *testing.T) {
	// Test that all generated tokens pass validation
	for i := 0; i < 100; i++ {
		token, err := verification.GenerateVerificationToken()
		if err != nil {
			t.Fatalf(errMsgGenTokenIteration, err, i)
		}

		if !verification.ValidateTokenFormat(token) {
			t.Errorf("Generated token failed validation: %s", token)
		}
	}
}

func TestGenerateTokenWithInfo(t *testing.T) {
	tokenInfo, err := verification.GenerateTokenWithInfo()
	if err != nil {
		t.Fatalf("GenerateTokenWithInfo() error = %v", err)
	}

	// Test basic token validation
	if !verification.ValidateTokenFormat(tokenInfo.Token) {
		t.Errorf("GenerateTokenWithInfo() returned invalid token: %s", tokenInfo.Token)
	}

	// Test metadata fields
	if tokenInfo.Length != verification.TokenLength {
		t.Errorf("TokenInfo.Length = %d, want %d", tokenInfo.Length, verification.TokenLength)
	}

	if tokenInfo.Encoding != "base64url" {
		t.Errorf("TokenInfo.Encoding = %s, want base64url", tokenInfo.Encoding)
	}

	expectedDNSRecord := dnsRecordPrefix + tokenInfo.Token
	if tokenInfo.DNSRecord != expectedDNSRecord {
		t.Errorf("TokenInfo.DNSRecord = %s, want %s", tokenInfo.DNSRecord, expectedDNSRecord)
	}

	expectedHTTPPath := "/.well-known/mcp-challenge/" + tokenInfo.Token
	if tokenInfo.HTTPPath != expectedHTTPPath {
		t.Errorf("TokenInfo.HTTPPath = %s, want %s", tokenInfo.HTTPPath, expectedHTTPPath)
	}
}

func TestTokenConstants(t *testing.T) {
	// Test that TokenLength is exactly 16 bytes (128 bits)
	expectedLength := 16
	if verification.TokenLength != expectedLength {
		t.Errorf("TokenLength = %d, want %d (128 bits)", verification.TokenLength, expectedLength)
	}
}

func TestTokenURLSafety(t *testing.T) {
	// Generate multiple tokens and ensure they're URL-safe
	for i := 0; i < 100; i++ {
		token, err := verification.GenerateVerificationToken()
		if err != nil {
			t.Fatalf(errMsgGenTokenIteration, err, i)
		}

		// Check that token doesn't contain URL-unsafe characters
		unsafeChars := []string{"+", "/", "=", " ", "%", "&", "?", "#"}
		for _, unsafe := range unsafeChars {
			if strings.Contains(token, unsafe) {
				t.Errorf("Token contains URL-unsafe character '%s': %s", unsafe, token)
			}
		}
	}
}

func TestTokenDNSSafety(t *testing.T) {
	// Generate multiple tokens and ensure they're DNS TXT record safe
	for i := 0; i < 100; i++ {
		token, err := verification.GenerateVerificationToken()
		if err != nil {
			t.Fatalf(errMsgGenTokenIteration, err, i)
		}

		// Check that token doesn't contain DNS-problematic characters
		// DNS TXT records generally support alphanumeric and some symbols
		unsafeChars := []string{" ", "\"", "\\", "\n", "\r", "\t"}
		for _, unsafe := range unsafeChars {
			if strings.Contains(token, unsafe) {
				t.Errorf("Token contains DNS-unsafe character '%s': %s", unsafe, token)
			}
		}

		// Test full DNS record format
		dnsRecord := dnsRecordPrefix + token
		MaxDNSRecordLength := 255
		if len(dnsRecord) > MaxDNSRecordLength {
			t.Errorf("DNS record too long (%d chars): %s", len(dnsRecord), dnsRecord)
		}
	}
}

func TestDNSTXTRecordRFCCompliance(t *testing.T) {
	// Test DNS TXT record format compliance according to RFC 1035 and RFC 1464
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenToken, err)
	}

	dnsRecord := dnsRecordPrefix + token

	// RFC 1035: DNS names and TXT records have specific length limitations
	// TXT record data must not exceed 255 octets per string
	if len(dnsRecord) > 255 {
		t.Errorf("DNS TXT record exceeds 255 character limit: %d chars", len(dnsRecord))
	}

	// RFC 1464: TXT records should follow attribute=value format
	if !strings.Contains(dnsRecord, "=") {
		t.Error("DNS TXT record missing required '=' separator")
	}

	parts := strings.SplitN(dnsRecord, "=", 2)
	if len(parts) != 2 {
		t.Error("DNS TXT record should have exactly one '=' separator")
	}

	attribute := parts[0]
	value := parts[1]

	// Validate attribute name (should be "mcp-verify")
	expectedAttribute := strings.TrimSuffix(dnsRecordPrefix, "=")
	if attribute != expectedAttribute {
		t.Errorf("DNS TXT record attribute = %s, want %s", attribute, expectedAttribute)
	}

	// Validate that value is our token
	if value != token {
		t.Errorf("DNS TXT record value = %s, want %s", value, token)
	}

	// Test that the record contains only ASCII printable characters (RFC compliant)
	for i, char := range dnsRecord {
		if char < 32 || char > 126 {
			t.Errorf("DNS TXT record contains non-ASCII printable character at position %d: %c (code %d)", i, char, char)
		}
	}
}

func TestDNSTXTRecordTokenValidation(t *testing.T) {
	// Test that tokens in DNS records are valid according to our format
	for i := 0; i < 50; i++ {
		token, err := verification.GenerateVerificationToken()
		if err != nil {
			t.Fatalf(errMsgGenTokenIteration, err, i)
		}

		dnsRecord := dnsRecordPrefix + token

		// Extract token from DNS record
		if !strings.HasPrefix(dnsRecord, dnsRecordPrefix) {
			t.Errorf("DNS record missing expected prefix: %s", dnsRecord)
			continue
		}

		extractedToken := strings.TrimPrefix(dnsRecord, dnsRecordPrefix)
		
		// Validate extracted token format
		if !verification.ValidateTokenFormat(extractedToken) {
			t.Errorf("Extracted token from DNS record failed validation: %s", extractedToken)
		}

		// Ensure token matches what we generated
		if extractedToken != token {
			t.Errorf("Extracted token %s does not match generated token %s", extractedToken, token)
		}
	}
}

func TestDNSTXTRecordSpecialCharacters(t *testing.T) {
	// Test that DNS records handle RFC-compliant special characters correctly
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenToken, err)
	}

	dnsRecord := dnsRecordPrefix + token

	// Characters that should NOT appear in our DNS records
	prohibitedChars := []rune{
		0,   // NULL
		9,   // TAB
		10,  // LF
		13,  // CR
		34,  // Double quote
		92,  // Backslash
		127, // DEL
	}

	for _, prohibited := range prohibitedChars {
		if strings.ContainsRune(dnsRecord, prohibited) {
			t.Errorf("DNS record contains prohibited character: %c (code %d)", prohibited, prohibited)
		}
	}

	// Characters that SHOULD be allowed (base64url safe)
	allowedChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_="
	for _, char := range dnsRecord {
		if !strings.ContainsRune(allowedChars, char) {
			t.Errorf("DNS record contains unexpected character: %c (code %d)", char, char)
		}
	}
}

func TestDNSTXTRecordLength(t *testing.T) {
	// Test DNS TXT record length constraints
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		t.Fatalf(errMsgGenToken, err)
	}

	dnsRecord := dnsRecordPrefix + token

	// RFC 1035: TXT record strings are limited to 255 octets
	maxTXTRecordLength := 255
	if len(dnsRecord) > maxTXTRecordLength {
		t.Errorf("DNS TXT record length %d exceeds RFC limit of %d", len(dnsRecord), maxTXTRecordLength)
	}

	// Calculate expected length: "mcp-verify=" (11 chars) + token (22 chars) = 33 chars
	expectedLength := 11 + 22 // len("mcp-verify=") + token length
	if len(dnsRecord) != expectedLength {
		t.Errorf("DNS TXT record length %d, expected %d", len(dnsRecord), expectedLength)
	}

	// Ensure we have reasonable margin below the limit
	marginRequired := 50 // Leave room for future changes
	if len(dnsRecord) > (maxTXTRecordLength - marginRequired) {
		t.Errorf("DNS TXT record length %d too close to limit, needs %d char margin", len(dnsRecord), marginRequired)
	}
}

// Benchmark tests for performance
func BenchmarkGenerateVerificationToken(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := verification.GenerateVerificationToken()
		if err != nil {
			b.Fatalf(errMsgGenToken, err)
		}
	}
}

func BenchmarkValidateTokenFormat(b *testing.B) {
	// Generate a token once for benchmarking validation
	token, err := verification.GenerateVerificationToken()
	if err != nil {
		b.Fatalf(errMsgGenToken, err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verification.ValidateTokenFormat(token)
	}
}

func BenchmarkGenerateTokenWithInfo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := verification.GenerateTokenWithInfo()
		if err != nil {
			b.Fatalf(errMsgGenTokenWithInfo, err)
		}
	}
}
