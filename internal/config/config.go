package config

import "os"

// Config holds the application configuration
type Config struct {
	ServerAddress  string
	DatabaseURL    string
	DatabaseName   string
	CollectionName string
	LogLevel       string
	Environment    string // Added environment field
	Version        string // Application version
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	// Get environment from OS environment variable or default to "production"
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}

	// Get version from environment variable or default to "dev"
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "dev"
	}

	// Get database URL from environment variable or default to localhost
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "localhost:5432"
	}

	// Get database name from environment variable or default to "mcp_registry"
	dbName := os.Getenv("DATABASE_NAME")
	if dbName == "" {
		dbName = "mcp-registry"
	}
	// Get collection name from environment variable or default to "entries"
	collectionName := os.Getenv("COLLECTION_NAME")
	if collectionName == "" {
		collectionName = "servers_v2"
	}

	return &Config{
		ServerAddress:  ":8080",
		DatabaseURL:    dbURL,
		DatabaseName:   dbName,
		CollectionName: collectionName,
		LogLevel:       "info",
		Environment:    env,
		Version:        version,
	}
}
