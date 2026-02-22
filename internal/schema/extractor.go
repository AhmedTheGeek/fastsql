// Package schema provides database schema extraction functionality.
package schema

import (
	"context"
	"database/sql"
	"time"
)

// Extractor defines the interface for extracting database schema.
type Extractor interface {
	// Extract retrieves the complete schema from a database.
	Extract(ctx context.Context, db *sql.DB) (*Schema, error)
	// Dialect returns the SQL dialect this extractor supports.
	Dialect() string
}

// Schema represents a complete database schema.
type Schema struct {
	// Dialect is the SQL dialect (postgres, mysql, sqlite).
	Dialect string
	// Database is the name of the database.
	Database string
	// Tables contains all tables and views.
	Tables []Table
	// ExtractedAt is when the schema was captured.
	ExtractedAt time.Time
}

// Table represents a database table or view.
type Table struct {
	// Schema is the schema/namespace (e.g., "public" in PostgreSQL).
	Schema string
	// Name is the table name.
	Name string
	// Type is "table" or "view".
	Type string
	// Columns contains column definitions.
	Columns []Column
	// PrimaryKey lists the primary key column names.
	PrimaryKey []string
	// ForeignKeys contains foreign key constraints.
	ForeignKeys []ForeignKey
	// Indexes contains index definitions.
	Indexes []Index
	// RowCount is an approximate row count (may be 0 if not available).
	RowCount int64
	// Comment is the table comment/description.
	Comment string
}

// Column represents a table column.
type Column struct {
	// Name is the column name.
	Name string
	// Type is the data type (e.g., "integer", "varchar(255)").
	Type string
	// Nullable indicates if NULL values are allowed.
	Nullable bool
	// DefaultValue is the default value expression.
	DefaultValue string
	// Comment is the column comment/description.
	Comment string
	// Position is the ordinal position (1-indexed).
	Position int
}

// ForeignKey represents a foreign key constraint.
type ForeignKey struct {
	// Name is the constraint name.
	Name string
	// Columns are the local column names.
	Columns []string
	// RefTable is the referenced table.
	RefTable string
	// RefSchema is the referenced table's schema.
	RefSchema string
	// RefColumns are the referenced column names.
	RefColumns []string
	// OnDelete is the ON DELETE action.
	OnDelete string
	// OnUpdate is the ON UPDATE action.
	OnUpdate string
}

// Index represents a table index.
type Index struct {
	// Name is the index name.
	Name string
	// Columns are the indexed column names (in order).
	Columns []string
	// Unique indicates if this is a unique index.
	Unique bool
	// Primary indicates if this is the primary key index.
	Primary bool
	// Type is the index type (btree, hash, etc.).
	Type string
}

// ExtractorRegistry holds registered schema extractors by dialect.
var extractorRegistry = make(map[string]func() Extractor)

// RegisterExtractor registers a schema extractor for a dialect.
func RegisterExtractor(dialect string, factory func() Extractor) {
	extractorRegistry[dialect] = factory
}

// GetExtractor returns an extractor for the given dialect.
func GetExtractor(dialect string) (Extractor, bool) {
	factory, ok := extractorRegistry[dialect]
	if !ok {
		return nil, false
	}
	return factory(), true
}

// SupportedDialects returns all registered dialect names.
func SupportedDialects() []string {
	dialects := make([]string, 0, len(extractorRegistry))
	for d := range extractorRegistry {
		dialects = append(dialects, d)
	}
	return dialects
}

// ExtractSchema is a convenience function that extracts schema using the appropriate extractor.
func ExtractSchema(ctx context.Context, db *sql.DB, dialect string) (*Schema, error) {
	extractor, ok := GetExtractor(dialect)
	if !ok {
		return nil, &ErrUnsupportedDialect{Dialect: dialect}
	}
	return extractor.Extract(ctx, db)
}

// ErrUnsupportedDialect is returned when no extractor exists for a dialect.
type ErrUnsupportedDialect struct {
	Dialect string
}

func (e *ErrUnsupportedDialect) Error() string {
	return "unsupported dialect: " + e.Dialect
}

// ErrExtraction is returned when schema extraction fails.
type ErrExtraction struct {
	Dialect string
	Phase   string
	Err     error
}

func (e *ErrExtraction) Error() string {
	return e.Dialect + " schema extraction failed (" + e.Phase + "): " + e.Err.Error()
}

func (e *ErrExtraction) Unwrap() error {
	return e.Err
}
