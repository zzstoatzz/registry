package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/registry/internal/api"
	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/model"
	"github.com/modelcontextprotocol/registry/internal/service"
	"github.com/modelcontextprotocol/registry/internal/telemetry"
)

func main() {
	// Parse command line flags
	showVersion := flag.Bool("version", false, "Display version information")
	flag.Parse()

	// Show version information if requested
	if *showVersion {
		log.Printf("MCP Registry v%s\n", Version)
		log.Printf("Git commit: %s\n", GitCommit)
		log.Printf("Build time: %s\n", BuildTime)
		return
	}

	log.Printf("Starting MCP Registry Application v%s (commit: %s)", Version, GitCommit)

	var (
		registryService service.RegistryService
		db              database.Database
		err             error
	)

	// Initialize configuration
	cfg := config.NewConfig()

	// Initialize services based on environment
	switch cfg.DatabaseType {
	case config.DatabaseTypeMemory:
		db = database.NewMemoryDB(map[string]*model.ServerDetail{})
		registryService = service.NewRegistryServiceWithDB(db)
	case config.DatabaseTypePostgreSQL:
		// Use PostgreSQL for real registry service
		// Create a context with timeout for PostgreSQL connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Connect to PostgreSQL
		db, err = database.NewPostgreSQL(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Printf("Failed to connect to PostgreSQL: %v", err)
			return
		}

		// Create registry service with PostgreSQL
		registryService = service.NewRegistryServiceWithDB(db)
		log.Printf("PostgreSQL database URL: %s", cfg.DatabaseURL)

		// Store the PostgreSQL instance for later cleanup
		defer func() {
			if err := db.Close(); err != nil {
				log.Printf("Error closing PostgreSQL connection: %v", err)
			} else {
				log.Println("PostgreSQL connection closed successfully")
			}
		}()
	default:
		log.Printf("Invalid database type: %s; supported types: %s, %s", cfg.DatabaseType, config.DatabaseTypeMemory, config.DatabaseTypePostgreSQL)
		return
	}

	// Import seed data if seed source is provided
	if cfg.SeedFrom != "" {
		log.Printf("Importing data from %s...", cfg.SeedFrom)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := db.ImportSeed(ctx, cfg.SeedFrom); err != nil {
			log.Printf("Failed to import seed data: %v", err)
		} else {
			log.Println("Data import completed successfully")
		}
	}

	shutdownTelemetry, metrics, err := telemetry.InitMetrics(cfg.Version)
	if err != nil {
		log.Printf("Failed to initialize metrics: %v", err)
		return
	}

	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			log.Printf("Failed to shutdown telemetry: %v", err)
		}
	}()

	// Initialize HTTP server
	server := api.NewServer(cfg, registryService, metrics)

	// Start server in a goroutine so it doesn't block signal handling
	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Failed to start server: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Create context with timeout for shutdown
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()

	// Gracefully shutdown the server
	if err := server.Shutdown(sctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
