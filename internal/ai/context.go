package ai

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AhmedTheGeek/fastsql/internal/schema"
)

// ContextBuilder builds prompts with schema context.
type ContextBuilder struct {
	maxTables     int
	includeStats  bool
	includeFKs    bool
	includeIndexes bool
}

// NewContextBuilder creates a new context builder.
func NewContextBuilder() *ContextBuilder {
	return &ContextBuilder{
		maxTables:     50,
		includeStats:  true,
		includeFKs:    true,
		includeIndexes: false, // Keep prompts smaller by default
	}
}

// WithMaxTables sets the maximum number of tables to include.
func (b *ContextBuilder) WithMaxTables(n int) *ContextBuilder {
	b.maxTables = n
	return b
}

// WithStats enables/disables row count stats.
func (b *ContextBuilder) WithStats(include bool) *ContextBuilder {
	b.includeStats = include
	return b
}

// WithForeignKeys enables/disables foreign key info.
func (b *ContextBuilder) WithForeignKeys(include bool) *ContextBuilder {
	b.includeFKs = include
	return b
}

// WithIndexes enables/disables index info.
func (b *ContextBuilder) WithIndexes(include bool) *ContextBuilder {
	b.includeIndexes = include
	return b
}

// BuildPrompt builds a complete prompt with schema context.
func (b *ContextBuilder) BuildPrompt(sch *schema.Schema, userPrompt string) string {
	var sb strings.Builder
	
	sb.WriteString("You are an expert SQL developer. Generate valid ")
	sb.WriteString(sch.Dialect)
	sb.WriteString(" SQL based on the user's request.\n\n")
	
	// Add schema context
	sb.WriteString("DATABASE SCHEMA:\n")
	sb.WriteString(b.FormatSchema(sch))
	sb.WriteString("\n")
	
	// Add rules
	sb.WriteString("RULES:\n")
	sb.WriteString("- Output ONLY valid SQL, no markdown code fences\n")
	sb.WriteString("- Use table and column names exactly as shown in the schema\n")
	sb.WriteString("- Use explicit JOINs rather than implicit joins\n")
	sb.WriteString("- Add comments for complex logic\n")
	sb.WriteString("- If the request is ambiguous, make reasonable assumptions\n\n")
	
	// Add user prompt
	sb.WriteString("USER REQUEST:\n")
	sb.WriteString(userPrompt)
	
	return sb.String()
}

// FormatSchema formats a schema for inclusion in a prompt.
func (b *ContextBuilder) FormatSchema(sch *schema.Schema) string {
	if sch == nil || len(sch.Tables) == 0 {
		return "(no tables)"
	}
	
	var sb strings.Builder
	
	// Sort tables by name for consistent output
	tables := make([]schema.Table, len(sch.Tables))
	copy(tables, sch.Tables)
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Name < tables[j].Name
	})
	
	// Limit tables if necessary
	if b.maxTables > 0 && len(tables) > b.maxTables {
		// Prioritize tables with more foreign keys (likely more important)
		sort.Slice(tables, func(i, j int) bool {
			return len(tables[i].ForeignKeys) > len(tables[j].ForeignKeys)
		})
		tables = tables[:b.maxTables]
		// Re-sort by name
		sort.Slice(tables, func(i, j int) bool {
			return tables[i].Name < tables[j].Name
		})
		sb.WriteString(fmt.Sprintf("(showing %d of %d tables)\n\n", b.maxTables, len(sch.Tables)))
	}
	
	for i, table := range tables {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(b.formatTable(&table))
	}
	
	return sb.String()
}

// formatTable formats a single table for the prompt.
func (b *ContextBuilder) formatTable(t *schema.Table) string {
	var sb strings.Builder
	
	// Table header
	tableName := t.Name
	if t.Schema != "" && t.Schema != "public" && t.Schema != "dbo" {
		tableName = t.Schema + "." + t.Name
	}
	
	if t.Type == "view" {
		sb.WriteString(fmt.Sprintf("VIEW %s", tableName))
	} else {
		sb.WriteString(fmt.Sprintf("TABLE %s", tableName))
	}
	
	if b.includeStats && t.RowCount > 0 {
		sb.WriteString(fmt.Sprintf(" (~%d rows)", t.RowCount))
	}
	sb.WriteString(":\n")
	
	// Columns
	for _, col := range t.Columns {
		sb.WriteString("  - ")
		sb.WriteString(col.Name)
		sb.WriteString(" ")
		sb.WriteString(col.Type)
		
		// Mark primary key
		for _, pk := range t.PrimaryKey {
			if pk == col.Name {
				sb.WriteString(" [PK]")
				break
			}
		}
		
		if !col.Nullable {
			sb.WriteString(" NOT NULL")
		}
		
		if col.DefaultValue != "" {
			sb.WriteString(fmt.Sprintf(" DEFAULT %s", col.DefaultValue))
		}
		
		sb.WriteString("\n")
	}
	
	// Foreign keys
	if b.includeFKs && len(t.ForeignKeys) > 0 {
		sb.WriteString("  FOREIGN KEYS:\n")
		for _, fk := range t.ForeignKeys {
			refTable := fk.RefTable
			if fk.RefSchema != "" && fk.RefSchema != "public" && fk.RefSchema != "dbo" {
				refTable = fk.RefSchema + "." + fk.RefTable
			}
			sb.WriteString(fmt.Sprintf("    - %s -> %s(%s)\n",
				strings.Join(fk.Columns, ", "),
				refTable,
				strings.Join(fk.RefColumns, ", ")))
		}
	}
	
	// Indexes (optional, can make prompts too long)
	if b.includeIndexes && len(t.Indexes) > 0 {
		sb.WriteString("  INDEXES:\n")
		for _, idx := range t.Indexes {
			if idx.Primary {
				continue // Already shown as [PK]
			}
			unique := ""
			if idx.Unique {
				unique = " UNIQUE"
			}
			sb.WriteString(fmt.Sprintf("    - %s%s (%s)\n",
				idx.Name, unique, strings.Join(idx.Columns, ", ")))
		}
	}
	
	return sb.String()
}

// FormatSchemaCompact formats schema in a more compact form.
func (b *ContextBuilder) FormatSchemaCompact(sch *schema.Schema) string {
	if sch == nil || len(sch.Tables) == 0 {
		return "(no tables)"
	}
	
	var sb strings.Builder
	
	for i, table := range sch.Tables {
		if i > 0 {
			sb.WriteString("\n")
		}
		
		tableName := table.Name
		if table.Schema != "" && table.Schema != "public" {
			tableName = table.Schema + "." + table.Name
		}
		
		sb.WriteString(tableName)
		sb.WriteString("(")
		
		for j, col := range table.Columns {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(col.Name)
			
			// Mark primary keys
			for _, pk := range table.PrimaryKey {
				if pk == col.Name {
					sb.WriteString("*")
					break
				}
			}
		}
		
		sb.WriteString(")")
	}
	
	return sb.String()
}

// EstimateTokens provides a rough estimate of tokens for a schema.
// This is approximate - actual tokenization varies by model.
func EstimateTokens(text string) int {
	// Rough estimate: ~4 characters per token for English text
	return len(text) / 4
}
