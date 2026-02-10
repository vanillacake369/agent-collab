# Claude Code Integration

Connect Claude Code to agent-collab for seamless multi-agent collaboration.

## Setup

### 1. Ensure Daemon is Running

```bash
agent-collab daemon start
```

### 2. Add MCP Server

```bash
claude mcp add agent-collab -- agent-collab mcp serve
```

### 3. Verify Connection

In Claude Code, the agent-collab tools should now be available.

## Available Tools

Once connected, Claude Code has access to:

| Tool | Purpose |
|------|---------|
| `acquire_lock` | Lock code regions before editing |
| `release_lock` | Release locks when done |
| `list_locks` | See active locks |
| `share_context` | Share knowledge |
| `search_similar` | Semantic search |
| `get_warnings` | Check for conflicts |
| `get_events` | Recent activity |
| `cluster_status` | Cluster health |

See [MCP Tools](../guide/mcp-tools.md) for detailed documentation.

## Using Skills

agent-collab provides Claude Code skills for common workflows:

### /collab-start

Start a collaboration session:

- Checks for warnings
- Reviews recent activity
- Prepares the workspace

### /collab-check

Check cluster status:

- Views active locks
- Searches related context
- Identifies potential conflicts

### /collab-share

Share completed work:

- Documents changes
- Releases locks
- Notifies other agents

## CLAUDE.md Integration

Add collaboration guidelines to your project's `CLAUDE.md`:

```markdown
## Agent Collaboration

This project uses agent-collab for multi-agent coordination.

### Before Editing Files
1. Call `get_warnings()` to check for conflicts
2. Call `acquire_lock()` on the region you'll modify

### After Completing Work
1. Call `share_context()` to document changes
2. Call `release_lock()` to free the region
```

## Troubleshooting

### Tools Not Available

1. Check daemon is running:
   ```bash
   agent-collab daemon status
   ```

2. Verify MCP registration:
   ```bash
   claude mcp list
   ```

3. Re-add if needed:
   ```bash
   claude mcp remove agent-collab
   claude mcp add agent-collab -- agent-collab mcp serve
   ```

### Connection Issues

Check cluster connectivity:

```bash
agent-collab status
```

If no peers are connected, verify:

- Network connectivity
- Firewall settings (port 4001)
- Invite token validity
