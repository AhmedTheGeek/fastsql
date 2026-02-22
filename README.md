# ⚡ FastSQL

A cross-platform TUI database management tool with **native SSH tunneling** and **AI-powered SQL generation**.

> Fork of [lazysql](https://github.com/jorgerojas26/lazysql) — enhanced with modern features for power users.

![FastSQL Demo](images/lazysql.png)

## ✨ Features

### Core (inherited from lazysql)
- 🖥️ Cross-platform (macOS, Windows, Linux)
- ⌨️ Vim keybindings
- 📑 Multiple connections with tabs
- 📝 Built-in SQL editor
- 🔍 Table browser with filtering

### New in FastSQL
- 🔐 **Native SSH Tunneling** — Connect through bastions without manual tunnels
- 🤖 **AI SQL Generation** — Natural language to SQL with multi-provider support
- 🧠 **Schema-Aware AI** — AI knows your tables, columns, and relationships

## 🤖 Supported AI Providers

| Provider | Status | Notes |
|----------|--------|-------|
| OpenAI | 🔜 Planned | GPT-4o, GPT-4 |
| Anthropic | 🔜 Planned | Claude 3.5/4 |
| Ollama | 🔜 Planned | Local models (llama, codellama, mistral) |
| Kimi | 🔜 Planned | Moonshot AI |
| DeepSeek | 🔜 Planned | DeepSeek Coder |
| Google | 🔜 Planned | Gemini |
| Groq | 🔜 Planned | Fast inference |
| OpenRouter | 🔜 Planned | Multi-model proxy |
| Custom | 🔜 Planned | Any OpenAI-compatible endpoint |

## 🗄️ Supported Databases

- PostgreSQL
- MySQL / MariaDB
- SQLite
- SQL Server

## 📦 Installation

### Homebrew (coming soon)
```bash
brew install fastsql
```

### Go
```bash
go install github.com/AhmedTheGeek/fastsql@latest
```

### Binary Releases
Download from [Releases](https://github.com/AhmedTheGeek/fastsql/releases).

## ⚙️ Configuration

Config file location:
- Linux: `~/.config/fastsql/config.toml`
- macOS: `~/Library/Application Support/fastsql/config.toml`
- Windows: `%APPDATA%\fastsql\config.toml`

### Example Configuration

```toml
[[database]]
Name = "Production DB"
Provider = "postgres"
DBName = "myapp"
URL = "postgres://user:pass@localhost:5432/myapp"

# SSH Tunnel (coming soon)
[database.ssh]
enabled = true
host = "bastion.example.com"
port = 22
user = "deploy"
key_path = "~/.ssh/id_ed25519"
tunnel_local_port = 15432
tunnel_remote_host = "db.internal"
tunnel_remote_port = 5432

# AI Configuration (coming soon)
[ai]
default_provider = "ollama"

[ai.providers.ollama]
endpoint = "http://localhost:11434"
model = "codellama:13b"

[ai.providers.claude]
api_key = "${ANTHROPIC_API_KEY}"
model = "claude-sonnet-4-20250514"
```

## ⌨️ Keybindings

| Key | Action |
|-----|--------|
| `?` | Show help |
| `Ctrl+E` | Open SQL editor |
| `Ctrl+G` | AI generate SQL (coming soon) |
| `Backspace` | Connection manager |
| `Enter` | Execute / Select |
| `j/k` | Navigate up/down |
| `H/L` | Switch panels |
| `q` | Quit |

## 🗺️ Roadmap

- [x] Fork and rebrand
- [ ] Native SSH tunnel support
- [ ] AI provider abstraction layer
- [ ] OpenAI integration
- [ ] Anthropic (Claude) integration
- [ ] Ollama integration (local models)
- [ ] Schema extraction and AI context injection
- [ ] AI prompt panel UI
- [ ] Kimi, DeepSeek, Gemini providers
- [ ] Query explanation and optimization

See [PROJECT_PLAN.md](docs/PROJECT_PLAN.md) for the full roadmap.

## 🤝 Contributing

Contributions are welcome! Please read the contributing guidelines before submitting PRs.

## 📄 License

MIT License — see [LICENSE.txt](LICENSE.txt)

## 🙏 Acknowledgments

- [lazysql](https://github.com/jorgerojas26/lazysql) — The original project this fork is based on
- [lazygit](https://github.com/jesseduffield/lazygit) — Inspiration for the TUI design
- [tview](https://github.com/rivo/tview) — TUI framework

---

**FastSQL** — Query faster. Think less. Ship more. ⚡
