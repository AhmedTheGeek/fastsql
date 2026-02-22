package schema

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"
)

func init() {
	RegisterExtractor("sqlite", func() Extractor { return &SQLiteExtractor{} })
	RegisterExtractor("sqlite3", func() Extractor { return &SQLiteExtractor{} })
}

// SQLiteExtractor extracts schema from SQLite databases.
type SQLiteExtractor struct{}

// Dialect returns the SQL dialect.
func (e *SQLiteExtractor) Dialect() string {
	return "sqlite"
}

// Extract retrieves the complete schema from a SQLite database.
func (e *SQLiteExtractor) Extract(ctx context.Context, db *sql.DB) (*Schema, error) {
	schema := &Schema{
		Dialect:     "sqlite",
		Database:    "main",
		ExtractedAt: time.Now(),
	}

	// Get tables and views
	tables, err := e.extractTables(ctx, db)
	if err != nil {
		return nil, err
	}
	schema.Tables = tables

	// Get columns for each table
	for i := range schema.Tables {
		columns, err := e.extractColumns(ctx, db, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].Columns = columns

		// Get primary key
		pk, err := e.extractPrimaryKey(ctx, db, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].PrimaryKey = pk

		// Get foreign keys
		fks, err := e.extractForeignKeys(ctx, db, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].ForeignKeys = fks

		// Get indexes
		indexes, err := e.extractIndexes(ctx, db, schema.Tables[i].Name)
		if err != nil {
			return nil, err
		}
		schema.Tables[i].Indexes = indexes

		// Get row count
		if schema.Tables[i].Type == "table" {
			count, err := e.extractRowCount(ctx, db, schema.Tables[i].Name)
			if err == nil {
				schema.Tables[i].RowCount = count
			}
		}
	}

	return schema, nil
}

func (e *SQLiteExtractor) extractTables(ctx context.Context, db *sql.DB) ([]Table, error) {
	query := `
		SELECT name, type
		FROM sqlite_master
		WHERE type IN ('table', 'view')
			AND name NOT LIKE 'sqlite_%'
		ORDER BY name`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "sqlite", Phase: "tables", Err: err}
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return nil, &ErrExtraction{Dialect: "sqlite", Phase: "tables scan", Err: err}
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

func (e *SQLiteExtractor) extractColumns(ctx context.Context, db *sql.DB, tableName string) ([]Column, error) {
	// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
	query := "PRAGMA table_info(" + quoteIdentifier(tableName) + ")"

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "sqlite", Phase: "columns", Err: err}
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var c Column
		var cid int
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &dflt, &pk); err != nil {
			return nil, &ErrExtraction{Dialect: "sqlite", Phase: "columns scan", Err: err}
		}
		c.Position = cid + 1
		c.Nullable = notNull == 0
		if dflt.Valid {
			c.DefaultValue = dflt.String
		}
		columns = append(columns, c)
	}

	return columns, rows.Err()
}

func (e *SQLiteExtractor) extractPrimaryKey(ctx context.Context, db *sql.DB, tableName string) ([]string, error) {
	query := "PRAGMA table_info(" + quoteIdentifier(tableName) + ")"

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "sqlite", Phase: "primary key", Err: err}
	}
	defer rows.Close()

	// SQLite's pk column is 1-indexed for composite keys, 0 for non-PK
	type pkCol struct {
		name  string
		order int
	}
	var pkCols []pkCol

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, &ErrExtraction{Dialect: "sqlite", Phase: "primary key scan", Err: err}
		}
		if pk > 0 {
			pkCols = append(pkCols, pkCol{name: name, order: pk})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by pk order
	result := make([]string, len(pkCols))
	for _, col := range pkCols {
		if col.order <= len(result) {
			result[col.order-1] = col.name
		}
	}

	// Remove empty entries (shouldn't happen, but safety)
	var pk []string
	for _, name := range result {
		if name != "" {
			pk = append(pk, name)
		}
	}

	return pk, nil
}

func (e *SQLiteExtractor) extractForeignKeys(ctx context.Context, db *sql.DB, tableName string) ([]ForeignKey, error) {
	query := "PRAGMA foreign_key_list(" + quoteIdentifier(tableName) + ")"

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "sqlite", Phase: "foreign keys", Err: err}
	}
	defer rows.Close()

	// Group by id (foreign key id)
	fkMap := make(map[int]*ForeignKey)

	for rows.Next() {
		var id, seq int
		var refTable, fromCol, toCol, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
			return nil, &ErrExtraction{Dialect: "sqlite", Phase: "foreign keys scan", Err: err}
		}

		if fk, exists := fkMap[id]; exists {
			fk.Columns = append(fk.Columns, fromCol)
			fk.RefColumns = append(fk.RefColumns, toCol)
		} else {
			fkMap[id] = &ForeignKey{
				Name:       "", // SQLite doesn't name FK constraints
				Columns:    []string{fromCol},
				RefTable:   refTable,
				RefColumns: []string{toCol},
				OnUpdate:   onUpdate,
				OnDelete:   onDelete,
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var fks []ForeignKey
	for _, fk := range fkMap {
		fks = append(fks, *fk)
	}

	return fks, nil
}

func (e *SQLiteExtractor) extractIndexes(ctx context.Context, db *sql.DB, tableName string) ([]Index, error) {
	// Get index list
	query := "PRAGMA index_list(" + quoteIdentifier(tableName) + ")"

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "sqlite", Phase: "indexes", Err: err}
	}
	defer rows.Close()

	type indexInfo struct {
		seq     int
		name    string
		unique  int
		origin  string
		partial int
	}

	var indexList []indexInfo
	for rows.Next() {
		var info indexInfo
		if err := rows.Scan(&info.seq, &info.name, &info.unique, &info.origin, &info.partial); err != nil {
			return nil, &ErrExtraction{Dialect: "sqlite", Phase: "indexes scan", Err: err}
		}
		indexList = append(indexList, info)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Get columns for each index
	var indexes []Index
	for _, info := range indexList {
		cols, err := e.extractIndexColumns(ctx, db, info.name)
		if err != nil {
			return nil, err
		}

		idx := Index{
			Name:    info.name,
			Columns: cols,
			Unique:  info.unique == 1,
			Primary: info.origin == "pk",
			Type:    "btree", // SQLite uses B-tree
		}
		indexes = append(indexes, idx)
	}

	return indexes, nil
}

func (e *SQLiteExtractor) extractIndexColumns(ctx context.Context, db *sql.DB, indexName string) ([]string, error) {
	query := "PRAGMA index_info(" + quoteIdentifier(indexName) + ")"

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, &ErrExtraction{Dialect: "sqlite", Phase: "index columns", Err: err}
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, &ErrExtraction{Dialect: "sqlite", Phase: "index columns scan", Err: err}
		}
		if name.Valid {
			columns = append(columns, name.String)
		}
	}

	return columns, rows.Err()
}

func (e *SQLiteExtractor) extractRowCount(ctx context.Context, db *sql.DB, tableName string) (int64, error) {
	// Use a simple count - for large tables this could be slow
	// but SQLite doesn't maintain stats like PostgreSQL
	query := "SELECT COUNT(*) FROM " + quoteIdentifier(tableName)

	var count int64
	err := db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// quoteIdentifier quotes a SQLite identifier.
func quoteIdentifier(s string) string {
	// Double any existing double quotes
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

// SQLite type affinity helper - might be useful for type normalization.
var sqliteTypeAffinityRe = regexp.MustCompile(`(?i)^(INT|INTEGER|TINYINT|SMALLINT|MEDIUMINT|BIGINT|UNSIGNED BIG INT|INT2|INT8)`)

// getTypeAffinity returns the SQLite type affinity for a column type.
func getTypeAffinity(colType string) string {
	upper := strings.ToUpper(colType)

	if sqliteTypeAffinityRe.MatchString(upper) {
		return "INTEGER"
	}
	if strings.Contains(upper, "CHAR") || strings.Contains(upper, "CLOB") || strings.Contains(upper, "TEXT") {
		return "TEXT"
	}
	if strings.Contains(upper, "BLOB") || upper == "" {
		return "BLOB"
	}
	if strings.Contains(upper, "REAL") || strings.Contains(upper, "FLOA") || strings.Contains(upper, "DOUB") {
		return "REAL"
	}
	return "NUMERIC"
}
