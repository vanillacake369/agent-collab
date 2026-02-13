---
description: Show detailed cluster and daemon status
allowed-tools: [Bash]
---

# /collab:status

Show comprehensive status of the agent-collab system.

## What This Command Does

1. **Daemon status** - Is the local daemon running?
2. **Cluster health** - Connection to P2P network
3. **Peer information** - Who else is connected
4. **Recent activity** - What's happening in the cluster

## Usage

```
/collab:status
```

## Instructions

### 1. Check Daemon
```bash
agent-collab status
```

### 2. Get Cluster Status
```
Call MCP tool: cluster_status
Arguments: {}
```

### 3. Get Recent Events
```
Call MCP tool: get_events
Arguments: {"limit": 20}
```

### 4. Get Warnings
```
Call MCP tool: get_warnings
Arguments: {}
```

## Output Format

```
## Daemon Status
- Running: Yes/No
- PID: <pid>
- Uptime: <duration>

## Cluster Connection
- Status: Connected/Disconnected
- Node ID: <node_id>
- Peers: N connected

## Peer List
| Peer ID | Status | Last Seen |
|---------|--------|-----------|

## Active Warnings
- [List warnings or "None"]

## Recent Events (Last 20)
| Time | Type | Peer | Details |
|------|------|------|---------|
```

If daemon is not running, suggest:
```
To start: agent-collab daemon start
```
