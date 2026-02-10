# agent-collab Plugin for Claude Code

Multi-agent collaboration tools for Claude Code. Enables context sharing, lock management, and event notifications across multiple Claude instances.

## Features

- **Context Sharing**: Share your work with other Claude agents via P2P network
- **Lock Management**: Prevent conflicts by acquiring locks before editing files
- **Event Notifications**: See what other agents are doing in real-time
- **Semantic Search**: Find related context shared by other agents

## Installation

### Prerequisites

1. Install agent-collab daemon:
```bash
# macOS
brew install agent-collab

# or from source
go install github.com/vanillacake369/agent-collab/cmd/agent-collab@latest
```

2. Start the daemon and join/create a cluster:
```bash
# Create a new cluster
agent-collab daemon start
agent-collab init --project my-project

# Or join existing cluster
agent-collab daemon start
agent-collab join <invite-token>
```

### Install Plugin

```bash
# Add marketplace (if not already added)
/plugin marketplace add vanillacake369/agent-collab

# Install plugin
/plugin install agent-collab
```

Or install directly from git:
```bash
/plugin install https://github.com/vanillacake369/agent-collab --subdir plugin
```

## Skills

### `/collab-start [query]`
Start a collaborative session. Checks for warnings and recent activity from other agents.

```
/collab-start authentication
```

### `/collab-share <file> <summary>`
Share your completed work with other agents.

```
/collab-share auth/login.go Added JWT token validation with 24h expiry
```

### `/collab-check [query]`
Check cluster status and search for context.

```
/collab-check database connection pool
```

## Hooks

The plugin includes automatic hooks:

- **PreToolUse (Edit/Write)**: Attempts to acquire lock before editing files
- **PostToolUse (Edit/Write)**: Logs file modifications
- **SessionStart**: Checks cluster connection status

## Workflow

1. **Start session**: `/collab-start` to check for warnings
2. **Work on code**: Edit files normally (locks are auto-acquired)
3. **Share work**: `/collab-share` to broadcast your changes
4. **Check others**: `/collab-check` to find related context

## Configuration

### MCP Server

Add to `~/.claude/claude_desktop_config.json`:

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

### Cluster Settings

Cluster configuration is stored in `~/.agent-collab/config.json`.

## Troubleshooting

### "Not connected to cluster"
```bash
agent-collab daemon status  # Check daemon
agent-collab daemon start   # Start if not running
```

### "Lock acquisition failed"
Another agent is working on that file. Check with:
```bash
/collab-check
```

### "Context not found"
P2P propagation may take a few seconds. Wait and search again.

## License

MIT
