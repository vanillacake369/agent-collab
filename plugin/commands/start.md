---
description: Start collaborative session - check cluster, warnings, and related context
argument-hint: [work description]
allowed-tools: [Bash]
---

# /collab:start

Start a collaborative work session by checking the cluster status and finding relevant context.

## What This Command Does

1. **Check cluster status** - Verify connection to other agents
2. **Get warnings** - See any conflicts or important updates
3. **Search related context** - Find what other agents have shared about your topic
4. **Verify cohesion** - Check if your planned work aligns with team direction

## Usage

```
/collab:start
/collab:start implement JWT authentication
/collab:start 인증 기능 구현
```

## Arguments

User's work description: $ARGUMENTS

## Instructions

Execute these MCP tool calls in sequence:

### 1. Check Cluster Status
```
Call MCP tool: cluster_status
Arguments: {}
```

### 2. Get Warnings
```
Call MCP tool: get_warnings
Arguments: {}
```

### 3. Get Recent Events
```
Call MCP tool: get_events
Arguments: {"limit": 10}
```

### 4. Search Related Context (if arguments provided)
```
Call MCP tool: search_similar
Arguments: {"query": "$ARGUMENTS", "limit": 5}
```

### 5. Check Cohesion (if arguments provided)
```
Call MCP tool: check_cohesion
Arguments: {"type": "before", "intention": "$ARGUMENTS"}
```

## Output Format

```
## Cluster Status
- Connected: Yes/No
- Peers: N agents online
- Node ID: xxx

## Warnings
- [List any warnings or "No warnings"]

## Recent Activity
- [List recent events from other agents]

## Related Context
- [List relevant shared contexts]

## Cohesion Check
- [Alignment status with team direction]
```

If there are conflicts or cohesion issues, ask the user how they want to proceed.
