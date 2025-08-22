# File Hashes Implementation Guide

## Overview

File hashes provide integrity verification for MCP server packages. The CLI tool generates SHA-256 hashes at publish time, and the registry validates these hashes to ensure package integrity.

## Implementation Strategy: CLI-Generated Hashes

### Flow

```
1. Developer runs: mcp-publisher publish
2. CLI tool fetches package files from URLs
3. CLI tool computes SHA-256 hashes
4. CLI tool includes hashes in publish request
5. Registry validates hashes match the files
6. Registry stores server.json with validated hashes
```

### Responsibilities

#### CLI Tool (Publisher)
- **Generates** hashes for package files
- **Includes** hashes in publish payload
- **Provides** option to skip hash generation (--no-hash flag)

#### Registry
- **Validates** provided hashes against actual files
- **Stores** validated hashes in server.json
- **Accepts** submissions without hashes (optional field)
- **Rejects** submissions with invalid hashes

#### Consumers
- **Verify** downloaded files match provided hashes (optional)
- **Decide** trust policy when hashes are absent

## Hash Format

### Structure
```json
{
  "file_hashes": {
    "<identifier>": "sha256:<hex-hash>"
  }
}
```

### Identifiers by Package Type

#### NPM Packages
```json
{
  "file_hashes": {
    "npm:@modelcontextprotocol/server-postgres@0.6.2": "sha256:abc123..."
  }
}
```

#### Python Packages
```json
{
  "file_hashes": {
    "pypi:mcp-server-postgres==0.6.2": "sha256:def456..."
  }
}
```

#### GitHub Releases
```json
{
  "file_hashes": {
    "github:owner/repo/v1.0.0/server.tar.gz": "sha256:789xyz..."
  }
}
```

#### Direct URLs
```json
{
  "file_hashes": {
    "https://example.com/packages/server-v1.0.0.tar.gz": "sha256:abc123..."
  }
}
```

## CLI Tool Implementation

### Hash Generation Process

1. **Identify Package Files**
   - Parse package_location from server.json
   - Determine download URLs based on package type
   - Handle multiple files if needed (e.g., wheels for different platforms)

2. **Download Files**
   - Use temporary directory for downloads
   - Stream large files to avoid memory issues
   - Implement retry logic for network failures

3. **Compute Hashes**
   - Use SHA-256 algorithm
   - Process files in chunks for memory efficiency
   - Generate consistent identifiers

4. **Include in Publish**
   - Add file_hashes to server.json before submission
   - Validate JSON structure

### CLI Commands

```bash
# Standard publish with hash generation
mcp-publisher publish server.json

# Skip hash generation (for testing or special cases)
mcp-publisher publish server.json --no-hash

# Verify existing hashes without publishing
mcp-publisher verify server.json

# Generate hashes and output to stdout (dry run)
mcp-publisher hash-gen server.json
```

## Registry Validation

### Validation Process

```python
def validate_file_hashes(server_json):
    # Skip if no hashes provided (optional field)
    if 'file_hashes' not in server_json:
        return True
    
    for identifier, expected_hash in server_json['file_hashes'].items():
        # Download file from identifier
        file_content = download_file(identifier)
        
        # Compute actual hash
        actual_hash = compute_sha256(file_content)
        
        # Compare hashes
        if f"sha256:{actual_hash}" != expected_hash:
            raise ValidationError(f"Hash mismatch for {identifier}")
    
    return True
```

### Error Responses

```json
{
  "error": "Hash validation failed",
  "details": {
    "npm:@example/server@1.0.0": {
      "expected": "sha256:abc123...",
      "actual": "sha256:def456...",
      "status": "mismatch"
    }
  }
}
```

## Migration Path

### Phase 1: Deploy Optional Field (Week 1)
- Update registry schema to include optional file_hashes
- Deploy registry without validation
- Document field for early adopters

### Phase 2: Enable Validation (Week 2)
- Activate hash validation in registry
- Continue accepting entries without hashes
- Monitor validation failures

### Phase 3: CLI Tool Support (Week 3-4)
- Release publisher tool with hash generation
- Documentation and examples
- Community feedback incorporation

### Phase 4: Adoption Push (Month 2+)
- Encourage hash inclusion
- Consider making required for verified badges
- Never make fully mandatory (backward compatibility)

## Security Considerations

1. **Algorithm Choice**
   - SHA-256 is current standard
   - Design allows future algorithm updates
   - Include algorithm in hash string (sha256:...)

2. **Network Security**
   - Always download over HTTPS
   - Validate SSL certificates
   - Implement download size limits

3. **Trust Boundaries**
   - Hashes verify integrity, not authenticity
   - Registry validation prevents tampered submissions
   - Consumers should verify independently

## Example Implementation

### Publisher Tool (Go)

```go
func generateFileHashes(serverJSON *ServerJSON) (map[string]string, error) {
    hashes := make(map[string]string)
    
    switch serverJSON.PackageLocation.Type {
    case "npm":
        url := getNPMPackageURL(serverJSON.PackageLocation.PackageName)
        identifier := fmt.Sprintf("npm:%s", serverJSON.PackageLocation.PackageName)
        hash, err := downloadAndHash(url)
        if err != nil {
            return nil, err
        }
        hashes[identifier] = fmt.Sprintf("sha256:%s", hash)
        
    case "pypi":
        // Similar for Python packages
        
    case "github":
        // Similar for GitHub releases
    }
    
    return hashes, nil
}

func downloadAndHash(url string) (string, error) {
    resp, err := http.Get(url)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    hasher := sha256.New()
    _, err = io.Copy(hasher, resp.Body)
    if err != nil {
        return "", err
    }
    
    return hex.EncodeToString(hasher.Sum(nil)), nil
}
```

### Registry Validation (Go)

```go
func (s *RegistryService) validateFileHashes(entry *RegistryEntry) error {
    if entry.FileHashes == nil {
        return nil // Optional field
    }
    
    for identifier, expectedHash := range entry.FileHashes {
        actualHash, err := s.computeHashForIdentifier(identifier)
        if err != nil {
            return fmt.Errorf("failed to validate %s: %w", identifier, err)
        }
        
        if actualHash != expectedHash {
            return fmt.Errorf("hash mismatch for %s", identifier)
        }
    }
    
    return nil
}
```

## Testing Strategy

1. **Unit Tests**
   - Hash computation correctness
   - Identifier generation
   - Error handling

2. **Integration Tests**
   - End-to-end publish with hashes
   - Validation failure scenarios
   - Network failure handling

3. **Manual Testing**
   - Various package types
   - Large files
   - Concurrent validations

## FAQ

**Q: What if package files are updated after publishing?**
A: The hash represents the file at publish time. Updates require new version publication.

**Q: Can I update just the hashes?**
A: No, hashes are part of the version. New hashes require new version.

**Q: What about private packages?**
A: The registry must be able to access files for validation. Private packages need accessible URLs during validation.

**Q: Are hashes required?**
A: No, file_hashes is optional to maintain backward compatibility.

**Q: What about multiple files per package?**
A: Each file gets its own hash entry in the file_hashes object.