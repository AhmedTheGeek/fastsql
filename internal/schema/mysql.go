package schema

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func init() {
	RegisterExtractor("mysql", func() Extractor { return &MySQLExtractor{} })
	RegisterExtractor("mariadb", func() Extractor { return &MySQLExtractor{} })
}

// MySQLExtractor extracts schema from MySQL/MariaDB databases.
type MySQLExtractor struct{}

// Dialect returns the SQL dialect.
func (e *MySQLExtractor) Dialect() string {
	return "mysql"
}

// Extract retrieves the complete schema from a MySQL database.
func (e *MySQLExtractor) Extract(ctx context.Context, db *sql.DB) (*Schema, error) {
	schema := &Schema{
		Dialect:     "mysql",
		ExtractedAt: time.Now(),
	}

	// Get database name
	row := db.QueryRowContext(ctx, "SELECT DATABASE()")
	if err := row.Scan(&schema.Database); err != nil {
		return nil, &ErrExtraction{Dialect: "mysql", Phase: "database name", Err: err}
	}

	// Get tables and views
	tables, err := e.extractTables(ctx, db, schema.Database)
	if err != nil {
		return nil, err
	}
	schema.Tables = tables

	// Get columns for each table
	for i := range schema.Tables {
		columns, err := e.extractColumns(ctx, db, schema.Database, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].Columns = columns
	}

	// Get primary keys
	for i := range schema.Tables {
		pk, err := e.extractPrimaryKey(ctx, db, schema.Database, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].PrimaryKey = pk
	}

	// Get foreign keys
	for i := range schema.Tables {
		fks, err := e.extractForeignKeys(ctx, db, schema.Database, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].ForeignKeys = fks
	}

	// Get indexes
	for i := range schema.Tables {
		indexes, err := e.extractIndexes(ctx, db, schema.Database, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].Indexes = indexes
	}

	return schema, nil
}

func (e *MySQLExtractor) extractTables(ctx context.Context, db *sql.DB, database string) ([]Table, error) {
	query := `
		SELECT 
			TABLE_NAME,
			LOWER(TABLE_TYPE) AS table_type,
			COALESCE(TABLE_ROWS, 0) AS row_count,
			COALESCE(TABLE_COMMENT, '') AS comment
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME`

	rows, err := db.QueryContext(ctx, query, database)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "mysql", Phase: "tables", Err: err}
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		var tableType string
		if err := rows.Scan(&t.Name, &tableType, &t.RowCount, &t.Comment); err != nil {
			return nil, &ErrExtraction{Dialect: "mysql", Phase: "tables scan", Err: err}
		}
		// Normalize table type
		if strings.Contains(tableType, "view") {
			t.Type = "view"
		} else {
			t.Type = "table"
		}
		t.Schema = database
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

func (e *MySQLExtractor) extractColumns(ctx context.Context, db *sql.DB, database, tableName string) ([]Column, error) {
	query := `
		SELECT 
			COLUMN_NAME,
			COLUMN_TYPE,
			CASE IS_NULLABLE WHEN 'YES' THEN 1 ELSE 0 END AS is_nullable,
			COALESCE(COLUMN_DEFAULT, '') AS column_default,
			COALESCE(COLUMN_COMMENT, '') AS comment,
			ORDINAL_POSITION
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ?
			AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`

	rows, err := db.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "mysql", Phase: "columns", Err: err}
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var c Column
		var nullable int
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &c.Comment, &c.Position); err != nil {
			return nil, &ErrExtraction{Dialect: "mysql", Phase: "columns scan", Err: err}
		}
		c.Nullable = nullable == 1
		columns = append(columns, c)
	}

	return columns, rows.Err()
}

func (e *MySQLExtractor) extractPrimaryKey(ctx context.Context, db *sql.DB, database, tableName string) ([]string, error) {
	query := `
		SELECT COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ?
			AND TABLE_NAME = ?
			AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION`

	rows, err := db.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "mysql", Phase: "primary key", Err: err}
	}
	defer rows.Close()

	var pk []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, &ErrExtraction{Dialect: "mysql", Phase: "primary key scan", Err: err}
		}
		pk = append(pk, col)
	}

	return pk, rows.Err()
}

func (e *MySQLExtractor) extractForeignKeys(ctx context.Context, db *sql.DB, database, tableName string) ([]ForeignKey, error) {
	query := `
		SELECT 
			kcu.CONSTRAINT_NAME,
			GROUP_CONCAT(kcu.COLUMN_NAME ORDER BY kcu.ORDINAL_POSITION) AS columns,
			kcu.REFERENCED_TABLE_NAME,
			kcu.REFERENCED_TABLE_SCHEMA,
			GROUP_CONCAT(kcu.REFERENCED_COLUMN_NAME ORDER BY kcu.ORDINAL_POSITION) AS ref_columns,
			rc.DELETE_RULE,
			rc.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
			ON rc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
			AND rc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		WHERE kcu.TABLE_SCHEMA = ?
			AND kcu.TABLE_NAME = ?
			AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		GROUP BY kcu.CONSTRAINT_NAME, kcu.REFERENCED_TABLE_NAME, 
			kcu.REFERENCED_TABLE_SCHEMA, rc.DELETE_RULE, rc.UPDATE_RULE`

	rows, err := db.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "mysql", Phase: "foreign keys", Err: err}
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		var columns, refColumns string
		if err := rows.Scan(&fk.Name, &columns, &fk.RefTable, &fk.RefSchema, &refColumns, &fk.OnDelete, &fk.OnUpdate); err != nil {
			return nil, &ErrExtraction{Dialect: "mysql", Phase: "foreign keys scan", Err: err}
		}
		fk.Columns = strings.Split(columns, ",")
		fk.RefColumns = strings.Split(refColumns, ",")
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func (e *MySQLExtractor) extractIndexes(ctx context.Context, db *sql.DB, database, tableName string) ([]Index, error) {
	query := `
		SELECT 
			INDEX_NAME,
			GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS columns,
			CASE NON_UNIQUE WHEN 0 THEN 1 ELSE 0 END AS is_unique,
			CASE INDEX_NAME WHEN 'PRIMARY' THEN 1 ELSE 0 END AS is_primary,
			INDEX_TYPE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ?
			AND TABLE_NAME = ?
		GROUP BY INDEX_NAME, NON_UNIQUE, INDEX_TYPE`

	rows, err := db.QueryContext(ctx, query, database, tableName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "mysql", Phase: "indexes", Err: err}
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var idx Index
		var columns string
		var unique, primary int
		if err := rows.Scan(&idx.Name, &columns, &unique, &primary, &idx.Type); err != nil {
			return nil, &ErrExtraction{Dialect: "mysql", Phase: "indexes scan", Err: err}
		}
		idx.Columns = strings.Split(columns, ",")
		idx.Unique = unique == 1
		idx.Primary = primary == 1
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}
