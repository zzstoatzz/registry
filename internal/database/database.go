package database

import (
	"context"
	"errors"
	"time"

	"github.com/modelcontextprotocol/registry/internal/model"
)

// Common database errors
var (
	ErrNotFound       = errors.New("record not found")
	ErrAlreadyExists  = errors.New("record already exists")
	ErrInvalidInput   = errors.New("invalid input")
	ErrDatabase       = errors.New("database error")
	ErrInvalidVersion = errors.New("invalid version: cannot publish older version after newer version")
)

// Database defines the interface for database operations on MCPRegistry entries
type Database interface {
	// List retrieves all MCPRegistry entries with optional filtering
	List(ctx context.Context, filter map[string]any, cursor string, limit int) ([]*model.Server, string, error)
	// GetByID retrieves a single ServerDetail by it's ID
	GetByID(ctx context.Context, id string) (*model.ServerDetail, error)
	// Publish adds a new ServerDetail to the database
	Publish(ctx context.Context, serverDetail *model.ServerDetail) error
	// StoreVerificationToken stores a verification token for a server
	StoreVerificationToken(ctx context.Context, serverID string, token *model.VerificationToken) error
	// GetVerificationToken retrieves a verification token by server ID
	GetVerificationToken(ctx context.Context, serverID string) (*model.VerificationToken, error)
	// ImportSeed imports initial data from a seed file
	ImportSeed(ctx context.Context, seedFilePath string) error
	// Close closes the database connection
	Close() error

	// Domain verification methods
	// GetVerifiedDomains retrieves all domains that are currently verified
	GetVerifiedDomains(ctx context.Context) ([]string, error)
	// GetDomainVerification retrieves domain verification details
	GetDomainVerification(ctx context.Context, domain string) (*model.DomainVerification, error)
	// UpdateDomainVerification updates or creates domain verification record
	UpdateDomainVerification(ctx context.Context, domainVerification *model.DomainVerification) error
	// CleanupOldVerifications removes old verification records before the given time
	CleanupOldVerifications(ctx context.Context, before time.Time) (int, error)
}

// ConnectionType represents the type of database connection
type ConnectionType string

const (
	// ConnectionTypeMemory represents an in-memory database connection
	ConnectionTypeMemory ConnectionType = "memory"
	// ConnectionTypeMongoDB represents a MongoDB database connection
	ConnectionTypeMongoDB ConnectionType = "mongodb"
)

// ConnectionInfo provides information about the database connection
type ConnectionInfo struct {
	// Type indicates the type of database connection
	Type ConnectionType
	// IsConnected indicates whether the database is currently connected
	IsConnected bool
	// Raw provides access to the underlying connection object, which will vary by implementation
	// For MongoDB, this will be *mongo.Client
	// For MemoryDB, this will be map[string]*model.MCPRegistry
	Raw any
}
