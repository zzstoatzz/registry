package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/registry/tools/publisher/auth"
)

// Package type constants
const (
	packageTypeDocker = "docker"
)

// Server structure types for JSON generation
type Repository struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

type VersionDetail struct {
	Version string `json:"version"`
}

type EnvironmentVariable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type RuntimeArgument struct {
	Description string `json:"description"`
	IsRequired  bool   `json:"is_required"`
	Format      string `json:"format"`
	Value       string `json:"value"`
	Default     string `json:"default"`
	Type        string `json:"type"`
	ValueHint   string `json:"value_hint"`
}

type PackageLocation struct {
	// URL to the package (e.g., https://www.npmjs.com/package/@example/server/v/1.5.0)
	URL  string `json:"url"`
	// Type of the package (e.g., "javascript", "python", "mcpb")
	Type string `json:"type"`
}

type Package struct {
	Location             PackageLocation       `json:"location"`
	Version              string                `json:"version,omitempty"`
	RuntimeHint          string                `json:"runtime_hint,omitempty"`
	RuntimeArguments     []RuntimeArgument     `json:"runtime_arguments,omitempty"`
	PackageArguments     []RuntimeArgument     `json:"package_arguments,omitempty"`
	EnvironmentVariables []EnvironmentVariable `json:"environment_variables,omitempty"`
}

type ServerJSON struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Status        string            `json:"status,omitempty"`
	Repository    Repository        `json:"repository"`
	VersionDetail VersionDetail     `json:"version_detail"`
	Packages      []Package         `json:"packages"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	var err error
	switch os.Args[1] {
	case "publish":
		err = publishCommand()
	case "create":
		err = createCommand()
	default:
		printUsage()
	}
	if err != nil {
		log.Fatal(err)
	}
}

func printUsage() {
	fmt.Fprint(os.Stdout, "MCP Registry Publisher Tool\n")
	fmt.Fprint(os.Stdout, "\n")
	fmt.Fprint(os.Stdout, "Usage:\n")
	fmt.Fprint(os.Stdout, "  mcp-publisher publish [flags]    Publish a server.json file to the registry\n")
	fmt.Fprint(os.Stdout, "  mcp-publisher create [flags]     Create a new server.json file\n")
	fmt.Fprint(os.Stdout, "\n")
	fmt.Fprint(os.Stdout, "Use 'mcp-publisher <command> --help' for more information about a command.\n")
}

func publishCommand() error {
	publishFlags := flag.NewFlagSet("publish", flag.ExitOnError)

	var registryURL string
	var mcpFilePath string
	var forceLogin bool
	var authMethod string
	var dnsDomain string
	var dnsPrivateKey string
	var httpDomain string
	var httpPrivateKey string

	// Command-line flags for configuration
	publishFlags.StringVar(&registryURL, "registry-url", "", "URL of the registry (required)")
	publishFlags.StringVar(&mcpFilePath, "mcp-file", "", "path to the MCP file (required)")
	publishFlags.BoolVar(&forceLogin, "login", false, "force a new login even if a token exists")
	publishFlags.StringVar(&authMethod, "auth-method", "github-at", "authentication method (default: github-at)")
	publishFlags.StringVar(&dnsDomain, "dns-domain", "", "domain name for DNS authentication (required for dns auth method)")
	publishFlags.StringVar(&dnsPrivateKey, "dns-private-key", "", "64-character hex seed for DNS authentication (required for dns auth method)")
	publishFlags.StringVar(&httpDomain, "http-domain", "", "domain name for HTTP authentication (required for http auth method)")
	publishFlags.StringVar(&httpPrivateKey, "http-private-key", "", "64-character hex seed for HTTP authentication (required for http auth method)")

	// Set custom usage function
	publishFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: mcp-publisher publish [flags]\n")
		fmt.Fprint(os.Stdout, "\n")
		fmt.Fprint(os.Stdout, "Publish a server.json file to the registry\n")
		fmt.Fprint(os.Stdout, "\n")
		fmt.Fprint(os.Stdout, "Flags:\n")
		fmt.Fprint(os.Stdout, "  --registry-url string       URL of the registry (required)\n")
		fmt.Fprint(os.Stdout, "  --mcp-file string           path to the MCP file (required)\n")
		fmt.Fprint(os.Stdout, "  --login                     force a new login even if a token exists\n")
		fmt.Fprint(os.Stdout, "  --auth-method string        authentication method (default: github-at)\n")
		fmt.Fprint(os.Stdout, "  --dns-domain string         domain name for DNS authentication\n")
		fmt.Fprint(os.Stdout, "  --dns-private-key string    64-character hex seed for DNS authentication\n")
		fmt.Fprint(os.Stdout, "  --http-domain string        domain name for HTTP authentication\n")
		fmt.Fprint(os.Stdout, "  --http-private-key string   64-character hex seed for HTTP authentication\n")
	}

	if err := publishFlags.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Error parsing flags: %v", err)
	}

	if registryURL == "" || mcpFilePath == "" {
		publishFlags.Usage()
		return errors.New("registry-url and mcp-file are required")
	}

	// Read MCP file
	mcpData, err := os.ReadFile(mcpFilePath)
	if err != nil {
		return fmt.Errorf("error reading MCP file: %w", err)
	}

	var authProvider auth.Provider // Determine the authentication method
	switch authMethod {
	case "github-at":
		log.Println("Using GitHub Access Token for authentication")
		authProvider = auth.NewGitHubATProvider(forceLogin, registryURL)
	case "github-oidc":
		log.Println("Using GitHub Actions OIDC for authentication")
		authProvider = auth.NewGitHubOIDCProvider(registryURL)
	case "dns":
		log.Println("Using DNS-based authentication")
		authProvider = auth.NewDNSProvider(registryURL, dnsDomain, dnsPrivateKey)
	case "http":
		log.Println("Using HTTP-based authentication")
		authProvider = auth.NewHTTPProvider(registryURL, httpDomain, httpPrivateKey)
	case "none":
		log.Println("Using anonymous authentication")
		authProvider = auth.NewNoneProvider(registryURL)
	default:
		return fmt.Errorf("unsupported authentication method: %s", authMethod)
	}

	// Check if login is needed and perform authentication
	ctx := context.Background()
	if authProvider.NeedsLogin() {
		err := authProvider.Login(ctx)
		if err != nil {
			return fmt.Errorf("failed to authenticate with %s: %w", authProvider.Name(), err)
		}
	}

	// Get the token
	token, err := authProvider.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("error getting token from %s: %w", authProvider.Name(), err)
	}

	// Publish to registry
	err = publishToRegistry(registryURL, mcpData, token)
	if err != nil {
		return fmt.Errorf("failed to publish to registry: %w", err)
	}

	log.Println("Successfully published to registry!")
	return nil
}

func createCommand() error {
	createFlags := flag.NewFlagSet("create", flag.ExitOnError)

	// Basic server information flags
	var name string
	var description string
	var version string
	var repoURL string
	var repoSource string
	var output string
	var status string

	// Package information flags
	var registryName string
	var packageName string
	var packageVersion string
	var runtimeHint string
	var execute string

	// Repeatable flags
	var envVars []string
	var packageArgs []string

	createFlags.StringVar(&name, "name", "", "Server name (e.g., io.github.owner/repo-name) (required)")
	createFlags.StringVar(&name, "n", "", "Server name (shorthand)")
	createFlags.StringVar(&description, "description", "", "Server description (required)")
	createFlags.StringVar(&description, "d", "", "Server description (shorthand)")
	createFlags.StringVar(&version, "version", "1.0.0", "Server version")
	createFlags.StringVar(&version, "v", "1.0.0", "Server version (shorthand)")
	createFlags.StringVar(&repoURL, "repo-url", "", "Repository URL (required)")
	createFlags.StringVar(&repoSource, "repo-source", "github", "Repository source")
	createFlags.StringVar(&output, "output", "server.json", "Output file path")
	createFlags.StringVar(&output, "o", "server.json", "Output file path (shorthand)")
	createFlags.StringVar(&status, "status", "active", "Server status (active or deprecated)")

	createFlags.StringVar(&registryName, "registry", "npm", "Package registry name")
	createFlags.StringVar(&packageName, "package-name", "", "Package name (defaults to server name)")
	createFlags.StringVar(&packageVersion, "package-version", "", "Package version (defaults to server version)")
	createFlags.StringVar(&runtimeHint, "runtime-hint", "", "Runtime hint (e.g., docker)")
	createFlags.StringVar(&execute, "execute", "", "Command to execute the server")
	createFlags.StringVar(&execute, "e", "", "Command to execute the server (shorthand)")

	// Custom flag for environment variables
	createFlags.Func("env-var",
		"Environment variable in format NAME:DESCRIPTION (can be repeated)",
		func(value string) error {
			envVars = append(envVars, value)
			return nil
		})

	// Custom flag for package arguments
	createFlags.Func("package-arg",
		"Package argument in format VALUE:DESCRIPTION (can be repeated)",
		func(value string) error {
			packageArgs = append(packageArgs, value)
			return nil
		})

	// Set custom usage function
	createFlags.Usage = func() {
		fmt.Fprint(os.Stdout, "Usage: mcp-publisher create [flags]\n")
		fmt.Fprint(os.Stdout, "\n")
		fmt.Fprint(os.Stdout, "Create a new server.json file\n")
		fmt.Fprint(os.Stdout, "\n")
		fmt.Fprint(os.Stdout, "Flags:\n")
		fmt.Fprint(os.Stdout, "  --name/-n string         Server name (e.g., io.github.owner/repo-name) (required)\n")
		fmt.Fprint(os.Stdout, "  --description/-d string  Server description (required)\n")
		fmt.Fprint(os.Stdout, "  --repo-url string        Repository URL (required)\n")
		fmt.Fprint(os.Stdout, "  --version/-v string      Server version (default: 1.0.0)\n")
		fmt.Fprint(os.Stdout, "  --status string          Server status (active or deprecated) (default: active)\n")
		fmt.Fprint(os.Stdout, "  --execute/-e string      Command to execute the server\n")
		fmt.Fprint(os.Stdout, "  --output/-o string       Output file path (default: server.json)\n")
		fmt.Fprint(os.Stdout, "  --registry string        Package registry name (default: npm)\n")
		fmt.Fprint(os.Stdout, "  --package-name string    Package name (defaults to server name)\n")
		fmt.Fprint(os.Stdout, "  --package-version string Package version (defaults to server version)\n")
		fmt.Fprint(os.Stdout, "  --runtime-hint string    Runtime hint (e.g., docker)\n")
		fmt.Fprint(os.Stdout, "  --repo-source string     Repository source (default: github)\n")
		fmt.Fprint(os.Stdout, "  --env-var string         Environment variable in format "+
			"NAME:DESCRIPTION (can be repeated)\n")
		fmt.Fprint(os.Stdout, "  --package-arg string     Package argument in format VALUE:DESCRIPTION (can be repeated)\n")
	}

	if err := createFlags.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Error parsing flags: %v", err)
	}

	// Validate required flags
	if name == "" {
		return errors.New("--name/-n is required")
	}
	if description == "" {
		return errors.New("--description/-d is required")
	}
	if repoURL == "" {
		return errors.New("--repo-url is required")
	}

	// Validate status field
	if status != "active" && status != "deprecated" {
		return errors.New("--status must be either 'active' or 'deprecated'")
	}

	// Set defaults
	if packageName == "" {
		packageName = name
	}
	if packageVersion == "" {
		packageVersion = version
	}

	// Set runtime hint based on registry name if not explicitly provided
	if runtimeHint == "" {
		switch registryName {
		case packageTypeDocker:
			runtimeHint = packageTypeDocker
		case "npm":
			runtimeHint = "npx"
		}
	}

	// Create server structure
	server := createServerStructure(name, description, version, repoURL, repoSource,
		registryName, packageName, packageVersion, runtimeHint, execute, envVars, packageArgs, status)

	// Convert to JSON
	jsonData, err := json.MarshalIndent(server, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Write to file
	err = os.WriteFile(output, jsonData, 0600)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	log.Printf("Successfully created %s", output)
	log.Println("You may need to edit the file to:")
	log.Println("  - Add or modify package arguments")
	log.Println("  - Set environment variable requirements")
	log.Println("  - Add remote server configurations")
	log.Println("  - Adjust runtime arguments")
	return nil
}

// publishToRegistry sends the MCP server details to the registry with authentication
func publishToRegistry(registryURL string, mcpData []byte, token string) error {
	// Parse the MCP JSON data
	var mcpDetails map[string]any
	err := json.Unmarshal(mcpData, &mcpDetails)
	if err != nil {
		return fmt.Errorf("error parsing server.json file: %w", err)
	}

	// Create the publish request payload
	var publishReq map[string]any
	if _, hasServerField := mcpDetails["server"]; hasServerField {
		// Already in PublishRequest format with server field (and possibly x-publisher)
		publishReq = mcpDetails
	} else {
		// Legacy ServerDetail format - wrap it in extension wrapper format
		publishReq = map[string]any{
			"server": mcpDetails,
		}
	}

	// Convert the request to JSON
	jsonData, err := json.Marshal(publishReq)
	if err != nil {
		return fmt.Errorf("error serializing request: %w", err)
	}

	// Ensure the URL ends with the publish endpoint
	if !strings.HasSuffix(registryURL, "/") {
		registryURL += "/"
	}
	publishURL := registryURL + "v0/publish"

	// Create and send the request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, publishURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read and check the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("publication failed with status %d: %s", resp.StatusCode, body)
	}

	log.Println(string(body))
	return nil
}

// parseEnvironmentVariables parses environment variable specifications
func parseEnvironmentVariables(envVars []string) []EnvironmentVariable {
	var environmentVariables []EnvironmentVariable
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, ":", 2)
		if len(parts) == 2 {
			environmentVariables = append(environmentVariables, EnvironmentVariable{
				Name:        parts[0],
				Description: parts[1],
			})
		} else {
			// If no description provided, use a default
			environmentVariables = append(environmentVariables, EnvironmentVariable{
				Name:        parts[0],
				Description: fmt.Sprintf("Environment variable for %s", parts[0]),
			})
		}
	}
	return environmentVariables
}

// parsePackageArguments parses package argument specifications
func parsePackageArguments(packageArgs []string) []RuntimeArgument {
	var packageArguments []RuntimeArgument
	for i, pkgArg := range packageArgs {
		parts := strings.SplitN(pkgArg, ":", 2)
		value := parts[0]
		description := fmt.Sprintf("Package argument %d", i+1)
		if len(parts) == 2 {
			description = parts[1]
		}

		packageArguments = append(packageArguments, RuntimeArgument{
			Description: description,
			IsRequired:  true, // Package arguments are typically required
			Format:      "string",
			Value:       value,
			Default:     value,
			Type:        "positional",
			ValueHint:   value,
		})
	}
	return packageArguments
}

// parseRuntimeArguments parses the execute command to create runtime arguments
func parseRuntimeArguments(execute string) []RuntimeArgument {
	var runtimeArguments []RuntimeArgument
	if execute == "" {
		return runtimeArguments
	}
	
	// Split the execute command into parts, handling quoted strings
	parts := smartSplit(execute)
	if len(parts) <= 1 {
		return runtimeArguments
	}
	
	// Skip the first part (command) and add each argument as a runtime argument
	for i, arg := range parts[1:] {
		description := getArgumentDescription(arg, i, parts)
		runtimeArguments = append(runtimeArguments, RuntimeArgument{
			Description: description,
			IsRequired:  false,
			Format:      "string",
			Value:       arg,
			Default:     arg,
			Type:        "positional",
			ValueHint:   arg,
		})
	}
	
	return runtimeArguments
}

// getArgumentDescription provides a description for a runtime argument
func getArgumentDescription(arg string, index int, parts []string) string {
	description := fmt.Sprintf("Runtime argument %d", index+1)
	
	// Try to provide better descriptions based on common patterns
	switch {
	case strings.HasPrefix(arg, "--"):
		description = fmt.Sprintf("Command line flag: %s", arg)
	case strings.HasPrefix(arg, "-") && len(arg) == 2:
		description = fmt.Sprintf("Command line option: %s", arg)
	case strings.Contains(arg, "="):
		description = fmt.Sprintf("Configuration parameter: %s", arg)
	case index > 0 && strings.HasPrefix(parts[index], "-"):
		description = fmt.Sprintf("Value for %s", parts[index])
	}
	
	return description
}

func createServerStructure(
	name, description, version, repoURL, repoSource, registryName,
	packageName, packageVersion, runtimeHint, execute string,
	envVars []string, packageArgs []string, status string,
) ServerJSON {
	// Parse environment variables
	environmentVariables := parseEnvironmentVariables(envVars)

	// Parse package arguments
	packageArguments := parsePackageArguments(packageArgs)

	// Parse execute command to create runtime arguments
	runtimeArguments := parseRuntimeArguments(execute)

	// Create package with URL based on registry type
	var packageURL string
	var packageType string
	
	switch registryName {
	case "npm":
		packageType = "javascript"
		if packageVersion != "" {
			packageURL = fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", packageName, packageVersion)
		} else {
			packageURL = fmt.Sprintf("https://www.npmjs.com/package/%s", packageName)
		}
	case "pypi":
		packageType = "python"
		if packageVersion != "" {
			packageURL = fmt.Sprintf("https://pypi.org/project/%s/%s", packageName, packageVersion)
		} else {
			packageURL = fmt.Sprintf("https://pypi.org/project/%s", packageName)
		}
	case packageTypeDocker:
		packageType = packageTypeDocker
		if packageVersion != "" {
			// Note: This is a Docker tag, not a port (docker://image:tag format)
			packageURL = fmt.Sprintf("docker://%s:%s", packageName, packageVersion) //nolint:nosprintfhostport
		} else {
			packageURL = fmt.Sprintf("docker://%s", packageName)
		}
	default:
		// Default to a generic URL format
		packageType = registryName
		packageURL = fmt.Sprintf("%s://%s/%s", registryName, packageName, packageVersion)
	}
	
	pkg := Package{
		Location: PackageLocation{
			URL:  packageURL,
			Type: packageType,
		},
		Version:              packageVersion,
		RuntimeHint:          runtimeHint,
		RuntimeArguments:     runtimeArguments,
		PackageArguments:     packageArguments,
		EnvironmentVariables: environmentVariables,
	}

	// Create server structure
	return ServerJSON{
		Name:        name,
		Description: description,
		Status:      status,
		Repository: Repository{
			URL:    repoURL,
			Source: repoSource,
		},
		VersionDetail: VersionDetail{
			Version: version,
		},
		Packages: []Package{pkg},
	}
}

// smartSplit splits a command string into parts, handling quoted strings and common shell patterns
func smartSplit(command string) []string {
	var parts []string
	var current strings.Builder
	var inQuotes bool
	var quoteChar rune

	for _, char := range command {
		switch {
		case char == '"' || char == '\'':
			switch {
			case !inQuotes:
				inQuotes = true
				quoteChar = char
			case char == quoteChar:
				inQuotes = false
				quoteChar = 0
			default:
				current.WriteRune(char)
			}
		case char == ' ' && !inQuotes:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
