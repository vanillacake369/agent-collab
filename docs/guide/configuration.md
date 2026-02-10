# Configuration

Customize agent-collab for your workflow.

## View Current Configuration

```bash
agent-collab config show
```

## Set Configuration Values

```bash
agent-collab config set <key> <value>
```

## Configuration Options

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

## Embedding Providers

agent-collab auto-detects available AI providers for embeddings:

| Provider | Environment Variable | Default Model |
|----------|---------------------|---------------|
| OpenAI | `OPENAI_API_KEY` | text-embedding-3-small |
| Anthropic | `ANTHROPIC_API_KEY` | voyage-2 |
| Google AI | `GOOGLE_API_KEY` | text-embedding-004 |
| Ollama | (auto-detect) | nomic-embed-text |

### Manual Override

```bash
agent-collab config set embedding.provider openai
agent-collab config set embedding.model text-embedding-3-large
```

## Data Directory

All data is stored in `~/.agent-collab/`:

```
~/.agent-collab/
├── key.json        # Node identity
├── vectors/        # Embeddings
├── metrics/        # Usage stats
├── daemon.sock     # Daemon API socket
├── daemon.pid      # Daemon PID
└── events.sock     # Event stream socket
```
