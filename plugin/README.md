# agent-collab Plugin for Claude Code

Multi-agent collaboration tools for Claude Code. Enables context sharing, lock management, cohesion checking, and event notifications across multiple Claude instances.

## Features

- **Context Sharing**: Share your work with other Claude agents via P2P network
- **Cohesion Checking**: Verify your work aligns with team context before and after changes
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

### `/collab-start [intention]`
Start a collaborative session. Checks for warnings, recent activity, and verifies your intention aligns with team context.

```
/collab-start implement session-based authentication
```

### `/collab-cohesion <before|after>: <description>`
Check if your work aligns with existing team context.

```
# Before starting work
/collab-cohesion before: switch authentication from JWT to sessions

# After completing work
/collab-cohesion after: replaced JWT tokens with session cookies
```

### `/collab-share <file> <summary>`
Share your completed work with other agents. Automatically checks cohesion before sharing.

```
/collab-share auth/login.go Added JWT token validation with 24h expiry
```

### `/collab-check [query]`
Check cluster status and search for context.

```
/collab-check database connection pool
```

## Recommended Workflow

```
1. /collab-start <what you plan to do>
   └── Checks cluster, finds related context, verifies cohesion

2. Work on the code
   └── Locks are auto-acquired when editing

3. /collab-share <file> <summary>
   └── Checks cohesion, shares context with team
```

### Example Session

```
You: /collab-start refactor authentication to use sessions

Claude: 📡 Cluster Status: Connected (2 agents online)

⚠️ Cohesion Alert:
Your intention may conflict with existing context:
- Agent-A (30m ago): "Implemented JWT-based authentication" (auth/handler.go)

Suggestions:
- Review Agent-A's JWT implementation before proceeding
- If this is an intentional change, inform the team

Do you want to proceed?

You: Yes, we decided to switch to sessions

[... work on code ...]

You: /collab-share auth/handler.go Replaced JWT with session-based auth

Claude: ✓ Context shared successfully.

Note: This marks a direction change from previous JWT approach.
Other agents will be notified of this change.
```

## Hooks

The plugin includes automatic hooks:

- **PreToolUse (Edit/Write)**: Attempts to acquire lock before editing files
- **PostToolUse (Edit/Write)**: Logs file modifications
- **SessionStart**: Checks cluster connection status

## MCP Tools

| Tool | Description |
|------|-------------|
| `acquire_lock` | Lock a code region before editing |
| `release_lock` | Release a lock when done |
| `list_locks` | See what other agents are working on |
| `share_context` | Share knowledge with other agents |
| `search_similar` | Find related context via semantic search |
| `check_cohesion` | Verify work aligns with team context |
| `get_warnings` | Get alerts about conflicts or relevant changes |
| `cluster_status` | View cluster health and connected peers |

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

### "Cohesion conflict detected"
Your intended work may conflict with existing team decisions. Either:
1. Review the conflicting context and adjust your approach
2. Proceed if this is an intentional direction change
3. Discuss with the team before making breaking changes

### "Lock acquisition failed"
Another agent is working on that file. Check with:
```bash
/collab-check
```

### "Context not found"
P2P propagation may take a few seconds. Wait and search again.

## License

MIT
