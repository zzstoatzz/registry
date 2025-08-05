package config

import (
	"github.com/caarlos0/env/v10"
)

type DatabaseType string

const (
	DatabaseTypeMongoDB DatabaseType = "mongodb"
	DatabaseTypeMemory  DatabaseType = "memory"
)

// Config holds the application configuration
type Config struct {
	ServerAddress              string       `env:"SERVER_ADDRESS" envDefault:":8080"`
	DatabaseType               DatabaseType `env:"DATABASE_TYPE" envDefault:"mongodb"`
	DatabaseURL                string       `env:"DATABASE_URL" envDefault:"mongodb://localhost:27017"`
	DatabaseName               string       `env:"DATABASE_NAME" envDefault:"mcp-registry"`
	CollectionName             string       `env:"COLLECTION_NAME" envDefault:"servers_v2"`
	VerificationCollectionName string       `env:"VERIFICATION_COLLECTION_NAME" envDefault:"verification"`
	LogLevel                   string       `env:"LOG_LEVEL" envDefault:"info"`
	SeedFilePath               string       `env:"SEED_FILE_PATH" envDefault:"data/seed.json"`
	SeedImport                 bool         `env:"SEED_IMPORT" envDefault:"true"`
	Version                    string       `env:"VERSION" envDefault:"dev"`
	GithubClientID             string       `env:"GITHUB_CLIENT_ID" envDefault:""`
	GithubClientSecret         string       `env:"GITHUB_CLIENT_SECRET" envDefault:""`

	// Background verification job configuration
	BackgroundJobEnabled                    bool   `env:"BACKGROUND_JOB_ENABLED" envDefault:"true"`
	BackgroundJobCronSchedule               string `env:"BACKGROUND_JOB_CRON_SCHEDULE" envDefault:"0 0 2 * * *"`
	BackgroundJobMaxConcurrentVerifications int    `env:"BACKGROUND_JOB_MAX_CONCURRENT" envDefault:"10"`
	BackgroundJobVerificationTimeoutSeconds int    `env:"BACKGROUND_JOB_VERIFICATION_TIMEOUT_SECONDS" envDefault:"30"`
	BackgroundJobFailureThreshold           int    `env:"BACKGROUND_JOB_FAILURE_THRESHOLD" envDefault:"3"`
	BackgroundJobNotificationCooldownHours  int    `env:"BACKGROUND_JOB_NOTIFICATION_COOLDOWN_HOURS" envDefault:"24"`
	BackgroundJobCleanupIntervalDays        int    `env:"BACKGROUND_JOB_CLEANUP_INTERVAL_DAYS" envDefault:"7"`
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
