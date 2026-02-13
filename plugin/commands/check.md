---
description: Check cluster status, locks, and search for context
argument-hint: [search query]
allowed-tools: [Bash]
---

# /collab:check

Quick check of cluster status and optional context search.

## What This Command Does

1. **Show cluster status** - Connection, peers, health
2. **List active locks** - See what files are being edited
3. **Search context** - Find related work (if query provided)

## Usage

```
/collab:check
/collab:check database connection
/collab:check 인증
```

## Arguments

Optional search query: $ARGUMENTS

## Instructions

### 1. Get Cluster Status
```
Call MCP tool: cluster_status
Arguments: {}
```

### 2. List Active Locks
```
Call MCP tool: list_locks
Arguments: {}
```

### 3. Search Context (if query provided)
```
Call MCP tool: search_similar
Arguments: {"query": "$ARGUMENTS", "limit": 10}
```

## Output Format

```
## Cluster Status
- Status: Connected/Disconnected
- Peers: N agents
- Uptime: X minutes

## Active Locks
| File | Agent | Since | Intention |
|------|-------|-------|-----------|
| ... | ... | ... | ... |

## Related Context (if searched)
| File | Summary | Agent | Time |
|------|---------|-------|------|
| ... | ... | ... | ... |
```
