// Package storage provides a storage interface for omniagent.
//
// Storage backends are used by the agent and skills for:
//   - Caching expensive operations (API responses, generated reports)
//   - Persisting conversation state
//   - Storing user preferences
//
// # Available Backends
//
//   - memory: In-memory storage (default, no persistence)
//   - sqlite: SQLite database (recommended for Lightsail)
//   - dynamodb: AWS DynamoDB (for serverless deployments)
package storage

import (
	"context"
	"errors"
	"time"
)

// Common errors.
var (
	ErrNotFound = errors.New("key not found")
	ErrClosed   = errors.New("storage is closed")
)

// Storage is the primary storage interface.
type Storage interface {
	// Get retrieves a value by key.
	// Returns ErrNotFound if the key doesn't exist or has expired.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value with an optional TTL.
	// If ttl is 0, the value never expires.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key.
	// Returns nil if the key doesn't exist.
	Delete(ctx context.Context, key string) error

	// Close releases storage resources.
	Close() error
}

// DocumentStorage extends Storage with document operations.
type DocumentStorage interface {
	Storage

	// Put stores a document in a collection.
	Put(ctx context.Context, collection string, doc Document) error

	// Query retrieves documents matching a filter.
	Query(ctx context.Context, collection string, filter map[string]any) ([]Document, error)

	// DeleteDoc removes a document by ID.
	DeleteDoc(ctx context.Context, collection, id string) error
}

// Document represents a stored document.
type Document struct {
	// ID is the document identifier.
	ID string

	// Data contains the document fields.
	Data map[string]any

	// Metadata contains system metadata.
	Metadata map[string]string

	// CreatedAt is when the document was created.
	CreatedAt time.Time

	// UpdatedAt is when the document was last modified.
	UpdatedAt time.Time
}

// Config configures storage creation.
type Config struct {
	// Type is the storage backend type: "memory", "sqlite", "dynamodb".
	Type string `yaml:"type" json:"type"`

	// Path is the database path (for sqlite).
	Path string `yaml:"path,omitempty" json:"path,omitempty"`

	// TableName is the table name (for dynamodb).
	TableName string `yaml:"table_name,omitempty" json:"table_name,omitempty"`

	// Region is the AWS region (for dynamodb).
	Region string `yaml:"region,omitempty" json:"region,omitempty"`
}
