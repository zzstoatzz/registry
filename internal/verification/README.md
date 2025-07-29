# Domain Verification Package

This package provides cryptographically secure token generation for domain ownership verification in the MCP Registry. It implements the requirements specified in the Server Name Verification system.

## Overview

The verification package generates 128-bit cryptographically secure random tokens used for proving domain ownership through two verification methods:

1. **DNS TXT Record Verification**: Add `mcp-verify=<token>` to your domain's DNS
2. **HTTP-01 Web Challenge**: Serve the token at `https://domain/.well-known/mcp-challenge/<token>`

## Functions

### GenerateVerificationToken()

Generates a cryptographically secure 128-bit random token encoded in base64url format.

```go
token, err := verification.GenerateVerificationToken()
if err != nil {
    return fmt.Errorf("failed to generate token: %w", err)
}
// token: "TBeVXe_X4npM6p8vpzStnA" (22 characters)
```

**Features:**
- Uses `crypto/rand` for cryptographically secure randomness
- 128 bits (16 bytes) of entropy
- Base64url encoding (URL-safe and DNS-safe)
- No padding characters
- 22-character output length

### ValidateTokenFormat(token string)

Validates that a token string matches the expected format for verification tokens.

```go
isValid := verification.ValidateTokenFormat("TBeVXe_X4npM6p8vpzStnA")
// Returns: true
```

**Validation Rules:**
- Exactly 22 characters long
- Contains only base64url characters: `A-Z`, `a-z`, `0-9`, `-`, `_`
- No padding or special characters

### GenerateTokenWithInfo()

Generates a token with additional metadata about how to use it.

```go
tokenInfo, err := verification.GenerateTokenWithInfo()
if err != nil {
    return fmt.Errorf("failed to generate token info: %w", err)
}

fmt.Printf("Token: %s\n", tokenInfo.Token)
fmt.Printf("DNS Record: %s\n", tokenInfo.DNSRecord)
fmt.Printf("HTTP Path: %s\n", tokenInfo.HTTPPath)
```

**Output:**
```
Token: TBeVXe_X4npM6p8vpzStnA
DNS Record: mcp-verify=TBeVXe_X4npM6p8vpzStnA
HTTP Path: /.well-known/mcp-challenge/TBeVXe_X4npM6p8vpzStnA
```

## Security Considerations

### Cryptographic Security
- Uses `crypto/rand` which provides cryptographically secure random numbers
- 128 bits provides 2^128 possible values (negligible collision probability)
- Suitable for cryptographic applications requiring unpredictable tokens

### Token Properties
- **Single-use**: Tokens should be used only once for verification
- **Time-limited**: Implement appropriate expiration policies
- **Secure transmission**: Always use HTTPS when transmitting tokens
- **Secure storage**: Store tokens securely on both client and server side

### Platform Compatibility
- Works on all platforms supported by Go's `crypto/rand`
- Automatically uses platform-appropriate entropy sources:
  - Linux/Unix: `/dev/urandom`
  - Windows: CryptGenRandom
  - macOS: SecRandomCopyBytes

## Usage Examples

### DNS Verification Setup
```go
package main

import (
    "fmt"
    "github.com/modelcontextprotocol/registry/internal/verification"
)

func setupDNSVerification(domain string) error {
    tokenInfo, err := verification.GenerateTokenWithInfo()
    if err != nil {
        return err
    }
    
    fmt.Printf("Add this TXT record to %s:\n", domain)
    fmt.Printf("Record: %s\n", tokenInfo.DNSRecord)
    fmt.Printf("Value: %s\n", tokenInfo.Token)
    
    return nil
}
```

### HTTP-01 Challenge Setup
```go
func setupHTTPChallenge(domain string) error {
    tokenInfo, err := verification.GenerateTokenWithInfo()
    if err != nil {
        return err
    }
    
    fmt.Printf("Serve the token at: https://%s%s\n", domain, tokenInfo.HTTPPath)
    fmt.Printf("Content: %s\n", tokenInfo.Token)
    
    return nil
}
```

### Token Validation
```go
func validateUserToken(userToken string) bool {
    if !verification.ValidateTokenFormat(userToken) {
        return false
    }
    
    // Additional validation logic here
    // (e.g., check against stored tokens, expiration, etc.)
    
    return true
}
```

## Constants

- `TokenLength`: 16 bytes (128 bits) - the entropy size of generated tokens

## Error Handling

The functions return errors in the following cases:

- `GenerateVerificationToken()`: When the system's entropy source is unavailable
- `GenerateTokenWithInfo()`: When token generation fails

Always check for errors and handle them appropriately:

```go
token, err := verification.GenerateVerificationToken()
if err != nil {
    log.Printf("Failed to generate verification token: %v", err)
    // Handle error appropriately (retry, fallback, etc.)
    return err
}
```

## Performance

Benchmark results on Apple M4 Max:

```
BenchmarkGenerateVerificationToken-16    5726528    196.1 ns/op
BenchmarkValidateTokenFormat-16          98329761   12.31 ns/op
BenchmarkGenerateTokenWithInfo-16        4017357    290.5 ns/op
```

Token generation is fast enough for real-time use in web applications.

## Testing

The package includes comprehensive tests covering:

- Token generation and uniqueness
- Entropy validation (exactly 128 bits)
- Format validation
- URL and DNS safety
- Error handling
- Performance benchmarks

Run tests with:
```bash
go test ./internal/verification -v
go test ./internal/verification -bench=.
```

## Integration

This package is designed to integrate with the MCP Registry's domain verification system as specified in `server-name-verification.md`. It provides the foundational token generation capability required for both DNS and HTTP verification methods.
