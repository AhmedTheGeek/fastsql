# FastSQL — Project Plan

> A TUI database client with native SSH tunneling and multi-provider AI SQL generation.
> Fork of [lazysql](https://github.com/jorgerojas26/lazysql) (MIT License).
> Repository: [github.com/AhmedTheGeek/fastsql](https://github.com/AhmedTheGeek/fastsql)

---

## Overview

**Goal:** Build a terminal-native database client that combines the simplicity of lazysql with:
1. Native SSH connection support (like TablePlus)
2. AI-powered SQL generation with multi-provider support

**Target Users:**
- Backend developers who live in the terminal
- DevOps/SREs managing production databases
- Anyone who wants AI-assisted SQL without leaving the CLI

---

## Features

### Phase 1: Core Fork & SSH Support (Weeks 1-2)

#### 1.1 Fork & Refactor
- [ ] Fork lazysql, rename project (e.g., `smartsql`, `aisql`, `sqlpilot`)
- [ ] Review codebase structure, identify extension points
- [ ] Set up CI/CD (GitHub Actions: build, test, release binaries)
- [ ] Add contribution guidelines, update README

#### 1.2 Native SSH Tunneling
- [ ] Add SSH config fields to connection:
  ```toml
  [[database]]
  Name = "Production"
  Provider = "postgres"
  
  [database.ssh]
  enabled = true
  host = "bastion.example.com"
  port = 22
  user = "deploy"
  auth = "key"                    # "key" | "password" | "agent"
  key_path = "~/.ssh/id_ed25519"
  passphrase = ""                 # or prompt
  
  [database.ssh.tunnel]
  local_port = 15432              # auto-assign if 0
  remote_host = "db.internal"
  remote_port = 5432
  ```
- [ ] Implement SSH tunnel manager using `golang.org/x/crypto/ssh`
- [ ] Support auth methods: key file, password, SSH agent
- [ ] Auto-reconnect on tunnel drop
- [ ] Connection status indicator in UI (🔒 tunneled)

#### 1.3 Connection UX Improvements
- [ ] Connection test button (verify before save)
- [ ] Import connections from TablePlus / DBeaver / pgAdmin (stretch)
- [ ] Secure credential storage (keyring integration or encrypted file)

---

### Phase 2: AI Integration — Core (Weeks 3-4)

#### 2.1 Schema Extraction
- [ ] Build schema introspection for each DB:
  | Database   | Source                          |
  |------------|---------------------------------|
  | PostgreSQL | `information_schema`, `pg_catalog` |
  | MySQL      | `information_schema`            |
  | SQLite     | `sqlite_master`, `PRAGMA`       |
  | SQL Server | `INFORMATION_SCHEMA`, `sys.*`   |
  
- [ ] Extract:
  - Tables & views (name, type, row count estimate)
  - Columns (name, type, nullable, default, primary key)
  - Indexes (name, columns, unique)
  - Foreign keys (relationships)
  - Constraints
  
- [ ] Cache schema (refresh on demand or on DDL)
- [ ] Optional: column stats (cardinality, min/max for query hints)

#### 2.2 AI Provider Abstraction
```go
// provider.go
type ProviderConfig struct {
    Type     string            // "openai", "anthropic", "ollama", etc.
    Endpoint string            // API endpoint
    APIKey   string            // API key (if needed)
    Model    string            // Model name
    Options  map[string]any    // Provider-specific options
}

type AIProvider interface {
    ID() string
    DisplayName() string
    
    // Core generation
    GenerateSQL(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
    StreamSQL(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    
    // Capabilities
    SupportsStreaming() bool
    MaxContextTokens() int
    
    // Lifecycle
    ValidateConfig() error
    Close() error
}

type GenerateRequest struct {
    Schema      Schema          // Database schema context
    Prompt      string          // User's natural language query
    Dialect     string          // "postgres", "mysql", "sqlite"
    History     []Message       // Conversation history (optional)
    MaxTokens   int
    Temperature float64
}

type GenerateResponse struct {
    SQL         string          // Generated SQL
    Explanation string          // Optional explanation
    Confidence  float64         // Optional confidence score
    TokensUsed  int
}
```

#### 2.3 Built-in Providers
| Provider   | Endpoint                          | Auth        | Notes                     |
|------------|-----------------------------------|-------------|---------------------------|
| OpenAI     | `api.openai.com`                  | API Key     | GPT-4o, GPT-4, etc.       |
| Anthropic  | `api.anthropic.com`               | API Key     | Claude 3.5/4              |
| Ollama     | `localhost:11434`                 | None        | Local models              |
| Kimi       | `api.moonshot.cn`                 | API Key     | Moonshot AI               |
| DeepSeek   | `api.deepseek.com`                | API Key     | DeepSeek Coder            |
| Google     | `generativelanguage.googleapis.com` | API Key   | Gemini                    |
| Groq       | `api.groq.com`                    | API Key     | Fast inference            |
| OpenRouter | `openrouter.ai`                   | API Key     | Multi-model proxy         |
| Azure OpenAI | Custom                         | API Key     | Enterprise                |
| Custom     | Any OpenAI-compatible             | Configurable| LM Studio, vLLM, etc.     |

---

### Phase 3: AI Integration — UX (Weeks 5-6)

#### 3.1 AI Panel UI
```
┌─ AI Assistant ─────────────────────────────────────────┐
│ Provider: [Claude ▾]                     Model: sonnet │
├────────────────────────────────────────────────────────┤
│ > Show me the top 10 customers by total order value   │
│   in the last 30 days, with their email addresses     │
│                                                        │
│ ─────────────────────────────────────────────────────  │
│                                                        │
│ SELECT c.email, c.name, SUM(o.total) as total_value   │
│ FROM customers c                                       │
│ JOIN orders o ON o.customer_id = c.id                 │
│ WHERE o.created_at > NOW() - INTERVAL '30 days'       │
│ GROUP BY c.id, c.email, c.name                        │
│ ORDER BY total_value DESC                             │
│ LIMIT 10;                                             │
│                                                        │
│ [Run] [Edit] [Copy] [Explain] [Optimize]              │
└────────────────────────────────────────────────────────┘
```

#### 3.2 Keybindings
| Key           | Action                              |
|---------------|-------------------------------------|
| `Ctrl+G`      | Open AI prompt panel                |
| `Ctrl+Shift+G`| Switch AI provider                  |
| `Ctrl+E`      | Explain selected/generated query    |
| `Ctrl+O`      | Optimize selected query             |
| `Enter`       | Submit prompt                       |
| `Ctrl+Enter`  | Run generated SQL immediately       |
| `Tab`         | Accept into SQL editor              |
| `Esc`         | Close AI panel                      |

#### 3.3 Context Injection
System prompt template:
```
You are a SQL expert. Generate valid {dialect} SQL based on the user's request.

DATABASE SCHEMA:
{schema_dump}

STATISTICS:
- Total tables: {table_count}
- Largest tables: {top_tables_by_rows}

RULES:
- Output ONLY valid SQL, no markdown fences
- Use table/column names exactly as shown
- Prefer explicit JOINs over implicit
- Add comments for complex logic
- If ambiguous, ask for clarification

USER REQUEST:
{prompt}
```

#### 3.4 Conversation History
- [ ] Maintain session history for follow-up queries
- [ ] "Refine" mode: "make it also filter by status = 'active'"
- [ ] Clear history option

---

### Phase 4: Polish & Release (Weeks 7-8)

#### 4.1 Configuration
```toml
# ~/.config/smartsql/config.toml

[ai]
enabled = true
default_provider = "claude"
auto_inject_schema = true
max_schema_tables = 50          # Limit for large DBs
include_row_counts = true
include_sample_values = false   # Privacy consideration

[ai.providers.openai]
api_key = "${OPENAI_API_KEY}"
model = "gpt-4o"
temperature = 0.2
max_tokens = 2000

[ai.providers.claude]
api_key = "${ANTHROPIC_API_KEY}"
model = "claude-sonnet-4-20250514"

[ai.providers.ollama]
endpoint = "http://localhost:11434"
model = "deepseek-coder:33b"

[ai.providers.kimi]
api_key = "${MOONSHOT_API_KEY}"
model = "moonshot-v1-128k"

[ai.providers.custom]
type = "openai-compatible"
endpoint = "http://localhost:1234/v1"
model = "local-model"
```

#### 4.2 Security
- [ ] API keys: support env vars, keyring, encrypted config
- [ ] Schema filtering: exclude sensitive tables/columns from AI context
- [ ] Audit log: optional logging of AI queries
- [ ] Read-only mode warning before running AI-generated mutations

#### 4.3 Documentation
- [ ] README with screenshots/GIFs
- [ ] Provider setup guides (Ollama, OpenAI, etc.)
- [ ] Keybinding reference
- [ ] Configuration reference
- [ ] Privacy/security considerations

#### 4.4 Distribution
- [ ] Homebrew formula
- [ ] AUR package
- [ ] Scoop manifest (Windows)
- [ ] Docker image
- [ ] Pre-built binaries (Linux, macOS, Windows)
- [ ] `go install` support

---

## Technical Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        TUI Layer                            │
│  (tview: connection list, sql editor, results, ai panel)   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Core Services                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ Connection  │  │   Schema    │  │    AI Service       │ │
│  │  Manager    │  │  Extractor  │  │  (provider router)  │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   SSH Tunnel    │  │    Database     │  │   AI Providers  │
│    Manager      │  │    Drivers      │  │                 │
│ (x/crypto/ssh)  │  │ (pgx, mysql,    │  │ OpenAI, Claude, │
│                 │  │  sqlite, etc.)  │  │ Ollama, Kimi... │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

---

## File Structure

```
smartsql/
├── cmd/
│   └── smartsql/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── app.go              # Main application
│   ├── config/
│   │   ├── config.go           # Config parsing
│   │   └── schema.go           # Config schema
│   ├── connection/
│   │   ├── manager.go          # Connection lifecycle
│   │   ├── ssh.go              # SSH tunnel management
│   │   └── drivers/            # DB-specific drivers
│   ├── schema/
│   │   ├── extractor.go        # Schema extraction interface
│   │   ├── postgres.go
│   │   ├── mysql.go
│   │   └── sqlite.go
│   ├── ai/
│   │   ├── service.go          # AI service orchestration
│   │   ├── provider.go         # Provider interface
│   │   ├── context.go          # Schema → prompt builder
│   │   └── providers/
│   │       ├── openai.go
│   │       ├── anthropic.go
│   │       ├── ollama.go
│   │       ├── kimi.go
│   │       ├── deepseek.go
│   │       └── generic.go      # OpenAI-compatible
│   └── ui/
│       ├── app.go              # TUI main
│       ├── connections.go      # Connection list view
│       ├── editor.go           # SQL editor
│       ├── results.go          # Query results
│       ├── ai_panel.go         # AI assistant panel
│       └── keybindings.go
├── pkg/
│   └── sqlparse/               # SQL parsing utilities
├── config.example.toml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Milestones

| Milestone        | Target    | Deliverable                                    |
|------------------|-----------|------------------------------------------------|
| M1: Fork Ready   | Week 2    | Renamed fork, CI/CD, SSH tunneling working     |
| M2: AI MVP       | Week 4    | Single provider (Claude), basic SQL generation |
| M3: Multi-Provider | Week 6  | All providers, provider switching UI           |
| M4: Release      | Week 8    | v0.1.0, Homebrew, docs, binaries               |

---

## Open Questions

1. **Project Name?** — `smartsql`, `sqlpilot`, `aisql`, `termsql`, `sqlai`?
2. **Monetization?** — Open source + hosted/pro version? Or pure OSS?
3. **Contribute upstream?** — Offer SSH/AI features as PRs to lazysql?
4. **Supported DBs at launch?** — Start with Postgres + MySQL + SQLite?

---

## Resources

- [lazysql source](https://github.com/jorgerojas26/lazysql)
- [tview docs](https://github.com/rivo/tview)
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh)
- [Anthropic API](https://docs.anthropic.com/claude/reference)
- [OpenAI API](https://platform.openai.com/docs/api-reference)
- [Ollama API](https://github.com/ollama/ollama/blob/main/docs/api.md)

---

*Created: 2026-02-22*
*Status: Draft*
