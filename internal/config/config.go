package config

import (
	"time"

	env "github.com/caarlos0/env/v11"
	"github.com/modelcontextprotocol/registry/internal/verification"
)

type DatabaseType string

const (
	DatabaseTypeMongoDB DatabaseType = "mongodb"
	DatabaseTypeMemory  DatabaseType = "memory"
)

// Config holds the application configuration
type Config struct {
	ServerAddress                string       `env:"SERVER_ADDRESS" envDefault:":8080"`
	DatabaseType                 DatabaseType `env:"DATABASE_TYPE" envDefault:"mongodb"`
	DatabaseURL                  string       `env:"DATABASE_URL" envDefault:"mongodb://localhost:27017"`
	DatabaseName                 string       `env:"DATABASE_NAME" envDefault:"mcp-registry"`
	CollectionName               string       `env:"COLLECTION_NAME" envDefault:"servers_v2"`
	LogLevel                     string       `env:"LOG_LEVEL" envDefault:"info"`
	SeedFilePath                 string       `env:"SEED_FILE_PATH" envDefault:"data/seed.json"`
	SeedImport                   bool         `env:"SEED_IMPORT" envDefault:"true"`
	Version                      string       `env:"VERSION" envDefault:"dev"`
	GithubClientID               string       `env:"GITHUB_CLIENT_ID" envDefault:""`
	GithubClientSecret           string       `env:"GITHUB_CLIENT_SECRET" envDefault:""`
	VerificationEnabled          bool         `env:"VERIFICATION_ENABLED" envDefault:"false"`
	VerificationInitialBackoffMs int          `env:"VERIFICATION_INITIAL_BACKOFF_MS" envDefault:"1000"`
	VerificationMaxBackoffMs     int          `env:"VERIFICATION_MAX_BACKOFF_MS" envDefault:"30000"`
	VerificationMaxRetries       int          `env:"VERIFICATION_MAX_RETRIES" envDefault:"5"`
	VerificationRequestTimeoutMs int          `env:"VERIFICATION_REQUEST_TIMEOUT_MS" envDefault:"30000"`
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	var cfg Config
	err := env.ParseWithOptions(&cfg, env.Options{
		Prefix: "MCP_REGISTRY_",
	})
	if err != nil {
		panic(err)
	}
	return &cfg
}

// ToWellKnownConfig converts the application config to a WellKnownConfig
func (c *Config) ToWellKnownConfig() *verification.WellKnownConfig {
	return &verification.WellKnownConfig{
		InitialBackoff: time.Duration(c.VerificationInitialBackoffMs) * time.Millisecond,
		MaxBackoff:     time.Duration(c.VerificationMaxBackoffMs) * time.Millisecond,
		MaxRetries:     c.VerificationMaxRetries,
		RequestTimeout: time.Duration(c.VerificationRequestTimeoutMs) * time.Millisecond,
	}
}
