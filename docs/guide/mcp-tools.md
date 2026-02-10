# MCP Tools

Once connected via MCP, your AI agent has access to these tools.

## Lock Tools

### acquire_lock

Lock a code region before editing.

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `file_path` | string | File to lock |
| `start_line` | int | Start line number |
| `end_line` | int | End line number |
| `intention` | string | What you plan to do |

**Example:**

```json
{
  "file_path": "auth/handler.go",
  "start_line": 10,
  "end_line": 50,
  "intention": "Adding JWT token validation logic"
}
```

### release_lock

Release a lock when done editing.

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `lock_id` | string | Lock ID to release |

### list_locks

See what other agents are working on.

**Returns:** List of active locks with file, lines, intention, and owner.

## Context Tools

### share_context

Share knowledge with other agents.

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `file_path` | string | Related file path |
| `content` | string | Context to share |

**Example:**

```json
{
  "file_path": "auth/handler.go",
  "content": "## Changes\n- Added JWT validation\n- Updated error handling\n\n## Impact\nAll auth endpoints now require valid tokens"
}
```

### search_similar

Find related context via semantic search.

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `query` | string | Search query |

**Returns:** Related context entries ranked by similarity.

## Monitoring Tools

### get_warnings

Get alerts about conflicts or relevant changes.

**Returns:** List of warnings about:

- Overlapping locks
- Recent changes to related files
- Potential conflicts

### get_events

Get recent cluster events.

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `limit` | int | Max events to return (default: 20) |

**Returns:** Recent events like lock acquisitions, context shares, peer joins.

### cluster_status

View cluster health and connected peers.

**Returns:** Cluster info including:

- Connected peer count
- Cluster uptime
- Network health

## Usage in Claude Code

After connecting with:

```bash
claude mcp add agent-collab -- agent-collab mcp serve
```

Claude Code can use these tools automatically. See [Collaboration Workflow](collaboration.md) for best practices.
