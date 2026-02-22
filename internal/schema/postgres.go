package schema

import (
	"context"
	"database/sql"
	"time"
)

func init() {
	RegisterExtractor("postgres", func() Extractor { return &PostgresExtractor{} })
	RegisterExtractor("postgresql", func() Extractor { return &PostgresExtractor{} })
}

// PostgresExtractor extracts schema from PostgreSQL databases.
type PostgresExtractor struct{}

// Dialect returns the SQL dialect.
func (e *PostgresExtractor) Dialect() string {
	return "postgres"
}

// Extract retrieves the complete schema from a PostgreSQL database.
func (e *PostgresExtractor) Extract(ctx context.Context, db *sql.DB) (*Schema, error) {
	schema := &Schema{
		Dialect:     "postgres",
		ExtractedAt: time.Now(),
	}

	// Get database name
	row := db.QueryRowContext(ctx, "SELECT current_database()")
	if err := row.Scan(&schema.Database); err != nil {
		return nil, &ErrExtraction{Dialect: "postgres", Phase: "database name", Err: err}
	}

	// Get tables and views
	tables, err := e.extractTables(ctx, db)
	if err != nil {
		return nil, err
	}
	schema.Tables = tables

	// Get columns for each table
	for i := range schema.Tables {
		columns, err := e.extractColumns(ctx, db, schema.Tables[i].Schema, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].Columns = columns
	}

	// Get primary keys
	for i := range schema.Tables {
		pk, err := e.extractPrimaryKey(ctx, db, schema.Tables[i].Schema, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].PrimaryKey = pk
	}

	// Get foreign keys
	for i := range schema.Tables {
		fks, err := e.extractForeignKeys(ctx, db, schema.Tables[i].Schema, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].ForeignKeys = fks
	}

	// Get indexes
	for i := range schema.Tables {
		indexes, err := e.extractIndexes(ctx, db, schema.Tables[i].Schema, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].Indexes = indexes
	}

	return schema, nil
}

func (e *PostgresExtractor) extractTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	query := `
		SELECT 
			n.nspname AS schema_name,
			c.relname AS table_name,
			CASE c.relkind 
				WHEN 'r' THEN 'table'
				WHEN 'v' THEN 'view'
				WHEN 'm' THEN 'view'
				ELSE 'table'
			END AS table_type,
			COALESCE(c.reltuples::bigint, 0) AS row_count,
			COALESCE(d.description, '') AS comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_description d ON d.objoid = c.oid AND d.objsubid = 0
		WHERE c.relkind IN ('r', 'v', 'm')
			AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY n.nspname, c.relname`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "postgres", Phase: "tables", Err: err}
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Schema, &t.Name, &t.Type, &t.RowCount, &t.Comment); err != nil {
			return nil, &ErrExtraction{Dialect: "postgres", Phase: "tables scan", Err: err}
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

func (e *PostgresExtractor) extractColumns(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]Column, error) {
	query := `
		SELECT 
			a.attname AS column_name,
			pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
			NOT a.attnotnull AS is_nullable,
			COALESCE(pg_get_expr(d.adbin, d.adrelid), '') AS column_default,
			COALESCE(col_description(c.oid, a.attnum), '') AS comment,
			a.attnum AS ordinal_position
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid
		LEFT JOIN pg_attrdef d ON d.adrelid = c.oid AND d.adnum = a.attnum
		WHERE c.relname = $1
			AND n.nspname = $2
			AND a.attnum > 0
			AND NOT a.attisdropped
		ORDER BY a.attnum`

	rows, err := db.QueryContext(ctx, query, tableName, schemaName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "postgres", Phase: "columns", Err: err}
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.DefaultValue, &c.Comment, &c.Position); err != nil {
			return nil, &ErrExtraction{Dialect: "postgres", Phase: "columns scan", Err: err}
		}
		columns = append(columns, c)
	}

	return columns, rows.Err()
}

func (e *PostgresExtractor) extractPrimaryKey(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]string, error) {
	query := `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
		WHERE c.relname = $1
			AND n.nspname = $2
			AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum)`

	rows, err := db.QueryContext(ctx, query, tableName, schemaName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "postgres", Phase: "primary key", Err: err}
	}
	defer rows.Close()

	var pk []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, &ErrExtraction{Dialect: "postgres", Phase: "primary key scan", Err: err}
		}
		pk = append(pk, col)
	}

	return pk, rows.Err()
}

func (e *PostgresExtractor) extractForeignKeys(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]ForeignKey, error) {
	query := `
		SELECT
			con.conname AS constraint_name,
			array_agg(a.attname ORDER BY x.n) AS columns,
			ref_class.relname AS ref_table,
			ref_ns.nspname AS ref_schema,
			array_agg(ref_a.attname ORDER BY x.n) AS ref_columns,
			CASE con.confdeltype
				WHEN 'a' THEN 'NO ACTION'
				WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'
				WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
				ELSE ''
			END AS on_delete,
			CASE con.confupdtype
				WHEN 'a' THEN 'NO ACTION'
				WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'
				WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
				ELSE ''
			END AS on_update
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_class ref_class ON ref_class.oid = con.confrelid
		JOIN pg_namespace ref_ns ON ref_ns.oid = ref_class.relnamespace
		CROSS JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS x(col, ref_col, n)
		JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = x.col
		JOIN pg_attribute ref_a ON ref_a.attrelid = con.confrelid AND ref_a.attnum = x.ref_col
		WHERE con.contype = 'f'
			AND c.relname = $1
			AND n.nspname = $2
		GROUP BY con.conname, ref_class.relname, ref_ns.nspname, con.confdeltype, con.confupdtype`

	rows, err := db.QueryContext(ctx, query, tableName, schemaName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "postgres", Phase: "foreign keys", Err: err}
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		var columns, refColumns []string
		if err := rows.Scan(&fk.Name, (*pqStringArray)(&columns), &fk.RefTable, &fk.RefSchema, (*pqStringArray)(&refColumns), &fk.OnDelete, &fk.OnUpdate); err != nil {
			return nil, &ErrExtraction{Dialect: "postgres", Phase: "foreign keys scan", Err: err}
		}
		fk.Columns = columns
		fk.RefColumns = refColumns
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func (e *PostgresExtractor) extractIndexes(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]Index, error) {
	query := `
		SELECT
			i.relname AS index_name,
			array_agg(a.attname ORDER BY x.n) AS columns,
			ix.indisunique AS is_unique,
			ix.indisprimary AS is_primary,
			am.amname AS index_type
		FROM pg_index ix
		JOIN pg_class c ON c.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_am am ON am.oid = i.relam
		CROSS JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS x(attnum, n)
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = x.attnum
		WHERE c.relname = $1
			AND n.nspname = $2
		GROUP BY i.relname, ix.indisunique, ix.indisprimary, am.amname`

	rows, err := db.QueryContext(ctx, query, tableName, schemaName)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "postgres", Phase: "indexes", Err: err}
	}
	defer rows.Close()

	var indexes []Index
	for rows.Next() {
		var idx Index
		var columns []string
		if err := rows.Scan(&idx.Name, (*pqStringArray)(&columns), &idx.Unique, &idx.Primary, &idx.Type); err != nil {
			return nil, &ErrExtraction{Dialect: "postgres", Phase: "indexes scan", Err: err}
		}
		idx.Columns = columns
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

// pqStringArray handles PostgreSQL text[] columns.
type pqStringArray []string

func (a *pqStringArray) Scan(src interface{}) error {
	if src == nil {
		*a = nil
		return nil
	}

	switch v := src.(type) {
	case []byte:
		*a = parsePostgresArray(string(v))
	case string:
		*a = parsePostgresArray(v)
	}
	return nil
}

// parsePostgresArray parses a PostgreSQL array literal like {a,b,c}.
func parsePostgresArray(s string) []string {
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil
	}
	s = s[1 : len(s)-1]
	if s == "" {
		return nil
	}

	var result []string
	var current []byte
	inQuote := false

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				result = append(result, string(current))
				current = nil
			} else {
				current = append(current, s[i])
			}
		default:
			current = append(current, s[i])
		}
	}
	if len(current) > 0 || len(result) > 0 {
		result = append(result, string(current))
	}

	return result
}
