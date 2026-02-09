# agent-collab

A P2P-based distributed collaboration system for AI agents. Enable your local AI agents to share context and coordinate work across a team without a central server.

## Features

- **P2P Architecture**: No central server required - agents communicate directly via libp2p
- **Multi-Model Support**: Works with OpenAI, Anthropic, Google, Ollama, and custom providers
- **Semantic Locking**: Prevent conflicts before they happen with intention-based locks on code regions
- **Context Synchronization**: Share and sync context across all connected agents
- **MCP Server**: Connect external agents (Claude Code, Gemini CLI, etc.) via Model Context Protocol
- **Token Tracking**: Monitor API token usage with detailed breakdowns
- **Vector Embeddings**: Store and query semantic embeddings locally

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/agent-collab.git
cd agent-collab

# Build
go build -o agent-collab ./cmd/agent-collab

# Or install directly
go install ./cmd/agent-collab
```

## Quick Start

### Initialize a New Cluster

```bash
# Create a new cluster for your project
agent-collab init my-project

# This outputs an invite token to share with teammates
```

### Join an Existing Cluster

```bash
# Join using the invite token from the cluster creator
agent-collab join <invite-token>
```

### Check Cluster Status

```bash
# View cluster status, connected peers, and active locks
agent-collab status
```

## Commands

### Cluster Management

| Command | Description |
|---------|-------------|
| `agent-collab init <project>` | Initialize a new cluster |
| `agent-collab join <token>` | Join an existing cluster |
| `agent-collab leave [--force] [--clean]` | Leave the current cluster |
| `agent-collab status` | Show cluster status |

### Lock Management

| Command | Description |
|---------|-------------|
| `agent-collab lock list` | List all active locks |
| `agent-collab lock release <lock-id>` | Release a specific lock |
| `agent-collab lock history` | Show recent lock history |

### Agent Management

| Command | Description |
|---------|-------------|
| `agent-collab agents list` | List connected AI agents |
| `agent-collab agents info <agent-id>` | Show agent details |
| `agent-collab agents providers` | List supported AI providers |

### Peer Management

| Command | Description |
|---------|-------------|
| `agent-collab peers list` | List connected peers |
| `agent-collab peers info <peer-id>` | Show peer details |

### Daemon Management

| Command | Description |
|---------|-------------|
| `agent-collab daemon start` | Start the background daemon |
| `agent-collab daemon stop` | Stop the background daemon |
| `agent-collab daemon status` | Show daemon status |
| `agent-collab daemon start -f` | Run daemon in foreground |

### MCP Server

| Command | Description |
|---------|-------------|
| `agent-collab mcp serve` | Start MCP server (connects to daemon if running) |
| `agent-collab mcp serve --standalone` | Start MCP server in standalone mode |
| `agent-collab mcp info` | Show MCP server info |

### Token Management

| Command | Description |
|---------|-------------|
| `agent-collab token show` | Display current invite token |
| `agent-collab token refresh` | Generate a new invite token |
| `agent-collab token usage [--period day\|week\|month] [--json]` | Show token usage statistics |

### Configuration

| Command | Description |
|---------|-------------|
| `agent-collab config show` | Display current configuration |
| `agent-collab config set <key> <value>` | Set a configuration value |
| `agent-collab config reset --force` | Reset to default configuration |

## Configuration Options

| Key | Default | Description |
|-----|---------|-------------|
| `network.listen_port` | 4001 | Port for P2P connections |
| `lock.default_ttl` | 30s | Default lock time-to-live |
| `lock.heartbeat_interval` | 10s | Lock heartbeat interval |
| `context.sync_interval` | 5s | Context synchronization interval |
| `token.daily_limit` | 200000 | Daily API token limit |
| `embedding.provider` | (auto) | Embedding provider (openai, anthropic, google, ollama, mock) |
| `embedding.model` | (provider default) | Embedding model to use |
| `ui.theme` | dark | UI theme (dark/light) |

## Multi-Model Support

agent-collab auto-detects available AI providers based on environment variables:

### OpenAI
```bash
export OPENAI_API_KEY=sk-...
# Uses text-embedding-3-small by default
```

### Anthropic (via Voyage AI)
```bash
export ANTHROPIC_API_KEY=ant-...
# Uses voyage-2 embedding model
```

### Google AI
```bash
export GOOGLE_API_KEY=...
# Uses text-embedding-004 by default
```

### Ollama (Local)
```bash
# No API key needed, just run Ollama locally
ollama run nomic-embed-text
# agent-collab will auto-detect at localhost:11434
```

### Manual Configuration
```bash
# Override auto-detection
agent-collab config set embedding.provider openai
agent-collab config set embedding.model text-embedding-3-large
```

## Daemon Mode

The daemon allows multiple AI agents (Claude sessions, Gemini CLI, etc.) to share the same cluster connection:

```bash
# Start the daemon
agent-collab daemon start

# Check status
agent-collab daemon status

# Stop when done
agent-collab daemon stop
```

When the daemon is running, all MCP clients automatically connect to it, sharing:
- Cluster connection and peer discovery
- Lock state and coordination
- Embedding service
- Vector store
- Real-time events via lightweight IPC (events.sock)

## MCP Integration

Connect external AI agents via Model Context Protocol:

### Claude Desktop
Add to `claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "agent-collab": {
      "command": "agent-collab",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Claude Code
```bash
claude mcp add agent-collab -- agent-collab mcp serve
```

### Recommended Setup

For the best experience, start the daemon before using Claude:

```bash
# 1. Start daemon (once)
agent-collab daemon start

# 2. Use Claude Code normally - it will auto-connect to the daemon
claude

# The MCP server will detect the running daemon and share the cluster connection
```

### Available MCP Tools
- `acquire_lock` - Acquire a semantic lock on a code region
- `release_lock` - Release a previously acquired lock
- `list_locks` - List all active locks
- `share_context` - Share context with other agents
- `embed_text` - Generate embeddings for text
- `search_similar` - Search for similar content
- `cluster_status` - Get cluster status
- `list_agents` - List connected agents
- `get_events` - Get recent cluster events (lock changes, agent joins, etc.)
- `get_warnings` - Get pending warnings about events that may affect your work

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Developer Machine A                       │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐      │
│  │ Claude Code │───▶│ agent-collab │◀──▶│ Vector DB   │      │
│  │   (MCP)     │    └──────┬──────┘    └─────────────┘      │
│  └─────────────┘           │                                  │
└────────────────────────────┼─────────────────────────────────┘
                              │ libp2p / Gossipsub
┌────────────────────────────┼─────────────────────────────────┐
│                     Developer Machine B                       │
│  ┌─────────────┐    ┌──────┴──────┐    ┌─────────────┐      │
│  │ Gemini CLI  │───▶│ agent-collab │◀──▶│ Vector DB   │      │
│  │   (MCP)     │    └─────────────┘    └─────────────┘      │
│  └─────────────┘                                              │
└──────────────────────────────────────────────────────────────┘
```

### Core Components

- **Agent Registry**: Manages connected AI agents with capabilities tracking
- **Embedding Service**: Multi-provider embedding generation (OpenAI, Anthropic, Google, Ollama)
- **Semantic Lock Service**: Manages locks on code regions with negotiation support
- **Context Sync Manager**: CRDT-based context synchronization across peers
- **MCP Server**: Model Context Protocol server for external agent integration
- **Token Tracker**: Tracks API token usage with persistence
- **Vector Store**: Local vector storage for embeddings
- **libp2p Node**: P2P networking with Kademlia DHT and Gossipsub

## Data Directory

agent-collab stores data in `~/.agent-collab/`:

```
~/.agent-collab/
├── key.json        # Node identity keys
├── vectors/        # Vector embeddings
├── metrics/        # Usage metrics
├── daemon.sock     # Unix socket for daemon HTTP API (when running)
├── daemon.pid      # Daemon process ID file (when running)
└── events.sock     # Unix socket for real-time event streaming (when running)
```

## Development

```bash
# Run tests
go test ./...

# Build with verbose output
go build -v ./cmd/agent-collab

# Run with debug logging
agent-collab --verbose status
```

## License

MIT License
