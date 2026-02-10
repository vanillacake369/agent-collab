<p align="center">
  <img src="https://img.shields.io/github/v/release/vanillacake369/agent-collab?style=flat-square" alt="Release">
  <img src="https://img.shields.io/github/license/vanillacake369/agent-collab?style=flat-square" alt="License">
  <img src="https://img.shields.io/github/actions/workflow/status/vanillacake369/agent-collab/ci.yml?branch=main&style=flat-square" alt="CI">
  <img src="https://img.shields.io/github/go-mod/go-version/vanillacake369/agent-collab?style=flat-square" alt="Go Version">
</p>

<h1 align="center">agent-collab</h1>

<p align="center">
  <b>P2P distributed collaboration for AI agents</b><br>
  Share context and coordinate work across your team without a central server.
</p>

---

## Why agent-collab?

When multiple AI agents work on the same codebase, conflicts happen. `agent-collab` solves this with:

- **No Server Required** — Direct P2P communication via libp2p
- **Semantic Locks** — Prevent conflicts before they happen with intention-based locking
- **Context Sharing** — Keep all agents in sync with CRDT-based synchronization
- **MCP Integration** — Connect Claude Code, Gemini CLI, or any MCP-compatible agent

## Installation

### Homebrew (macOS/Linux)

```bash
brew install vanillacake369/tap/agent-collab
```

### APT (Debian/Ubuntu)

```bash
curl -fsSL https://vanillacake369.github.io/agent-collab/gpg.key | \
  sudo gpg --dearmor -o /usr/share/keyrings/agent-collab.gpg

echo "deb [signed-by=/usr/share/keyrings/agent-collab.gpg] \
  https://vanillacake369.github.io/agent-collab stable main" | \
  sudo tee /etc/apt/sources.list.d/agent-collab.list

sudo apt update && sudo apt install agent-collab
```

### Go Install

```bash
go install github.com/vanillacake369/agent-collab/src@latest
```

### Docker

```bash
docker pull ghcr.io/vanillacake369/agent-collab:latest
```

<details>
<summary><b>Other installation methods</b></summary>

### RPM (Fedora/RHEL)

```bash
curl -fsSL https://github.com/vanillacake369/agent-collab/releases/latest/download/agent-collab_linux_amd64.rpm -o agent-collab.rpm
sudo rpm -i agent-collab.rpm
```

### Binary Download

Download from [Releases](https://github.com/vanillacake369/agent-collab/releases):
- `agent-collab_vX.Y.Z_darwin_arm64.tar.gz` — macOS Apple Silicon
- `agent-collab_vX.Y.Z_darwin_amd64.tar.gz` — macOS Intel
- `agent-collab_vX.Y.Z_linux_amd64.tar.gz` — Linux x86_64
- `agent-collab_vX.Y.Z_linux_arm64.tar.gz` — Linux ARM64
- `agent-collab_vX.Y.Z_windows_amd64.zip` — Windows

### Build from Source

```bash
git clone https://github.com/vanillacake369/agent-collab.git
cd agent-collab
go build -o agent-collab ./src
```

</details>

## Quick Start

### 1. Start the Daemon

```bash
agent-collab daemon start
```

### 2. Create a Cluster

```bash
agent-collab init my-project
# Outputs an invite token for teammates
```

### 3. Connect Claude Code

```bash
claude mcp add agent-collab -- agent-collab mcp serve
```

That's it! Your AI agents can now share context and coordinate locks.

<details>
<summary><b>Join an existing cluster</b></summary>

```bash
# Get the invite token from the cluster creator
agent-collab join <invite-token>
```

</details>

## How It Works

```
┌─────────────────────────────────────────────────────────┐
│                  Developer Machine A                     │
│  ┌─────────────┐    ┌──────────────┐    ┌───────────┐  │
│  │ Claude Code │───▶│ agent-collab │◀──▶│ Vector DB │  │
│  │   (MCP)     │    └──────┬───────┘    └───────────┘  │
│  └─────────────┘           │                            │
└────────────────────────────┼────────────────────────────┘
                             │ P2P (libp2p + GossipSub)
┌────────────────────────────┼────────────────────────────┐
│                  Developer Machine B                     │
│  ┌─────────────┐    ┌──────┴───────┐    ┌───────────┐  │
│  │ Gemini CLI  │───▶│ agent-collab │◀──▶│ Vector DB │  │
│  │   (MCP)     │    └──────────────┘    └───────────┘  │
│  └─────────────┘                                        │
└─────────────────────────────────────────────────────────┘
```

**Semantic Locks** prevent conflicts by tracking *intent*. Before editing `auth/handler.go`, an agent acquires a lock explaining what it plans to do. Other agents see this and work elsewhere.

**Context Sync** uses CRDTs to share knowledge across the cluster. When one agent learns something about the codebase, all agents benefit.

## MCP Tools

Once connected, your AI agent has access to:

| Tool | Description |
|------|-------------|
| `acquire_lock` | Lock a code region before editing |
| `release_lock` | Release a lock when done |
| `list_locks` | See what other agents are working on |
| `share_context` | Share knowledge with other agents |
| `search_similar` | Find related context via semantic search |
| `get_warnings` | Get alerts about conflicts or relevant changes |
| `cluster_status` | View cluster health and connected peers |

## Commands

### Cluster

```bash
agent-collab init <project>     # Create a new cluster
agent-collab join <token>       # Join an existing cluster
agent-collab leave              # Leave the cluster
agent-collab status             # Show cluster status
```

### Locks

```bash
agent-collab lock list          # List active locks
agent-collab lock release <id>  # Release a lock
agent-collab lock history       # Recent lock activity
```

### Daemon

```bash
agent-collab daemon start       # Start background daemon
agent-collab daemon stop        # Stop daemon
agent-collab daemon status      # Check daemon status
```

### Token & Config

```bash
agent-collab token show         # Show invite token
agent-collab token usage        # API token usage stats
agent-collab config show        # Current configuration
agent-collab config set <k> <v> # Set config value
```

<details>
<summary><b>All configuration options</b></summary>

| Key | Default | Description |
|-----|---------|-------------|
| `network.listen_port` | 4001 | P2P listening port |
| `lock.default_ttl` | 30s | Lock time-to-live |
| `lock.heartbeat_interval` | 10s | Lock heartbeat interval |
| `context.sync_interval` | 5s | Context sync frequency |
| `token.daily_limit` | 200000 | Daily API token limit |
| `embedding.provider` | auto | Embedding provider |
| `embedding.model` | provider default | Embedding model |
| `ui.theme` | dark | UI theme |

</details>

## Multi-Provider Support

agent-collab auto-detects available AI providers:

| Provider | Environment Variable | Default Model |
|----------|---------------------|---------------|
| OpenAI | `OPENAI_API_KEY` | text-embedding-3-small |
| Anthropic | `ANTHROPIC_API_KEY` | voyage-2 |
| Google AI | `GOOGLE_API_KEY` | text-embedding-004 |
| Ollama | (auto-detect) | nomic-embed-text |

```bash
# Manual override
agent-collab config set embedding.provider openai
agent-collab config set embedding.model text-embedding-3-large
```

## Data Directory

```
~/.agent-collab/
├── key.json        # Node identity
├── vectors/        # Embeddings
├── metrics/        # Usage stats
├── daemon.sock     # Daemon API socket
├── daemon.pid      # Daemon PID
└── events.sock     # Event stream socket
```

## Contributing

Contributions are welcome! See the [contribution guidelines](CONTRIBUTING.md) for details.

```bash
# Run tests
go test ./src/...

# Build
go build ./src
```

## License

MIT License — see [LICENSE](LICENSE) for details.
