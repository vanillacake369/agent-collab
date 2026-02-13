---
name: collab-start
description: Use when user says "start collaboration", "begin teamwork", "협업 시작", "작업 전 확인", "check team status", "what are others doing", "다른 에이전트 확인", or wants to verify cluster connection and team context before starting work on a shared codebase.
version: 1.0.0
---

# Collaborative Session Start

This skill helps you start a collaborative work session by checking the cluster status, finding relevant context from other agents, and verifying your work aligns with team direction.

## When to Use

- Before starting any work in a multi-agent environment
- When you want to see what other agents have been working on
- When you need to check for potential conflicts
- When the user mentions "collaboration", "teamwork", or "other agents"

## Workflow

### Step 1: Check Cluster Status
Call `cluster_status` MCP tool to verify connection:
```json
{"name": "cluster_status", "arguments": {}}
```

### Step 2: Get Warnings
Call `get_warnings` to see any conflicts or alerts:
```json
{"name": "get_warnings", "arguments": {}}
```

### Step 3: Get Recent Events
Call `get_events` to see recent activity:
```json
{"name": "get_events", "arguments": {"limit": 10}}
```

### Step 4: Search Related Context
If the user described their intended work, search for related context:
```json
{"name": "search_similar", "arguments": {"query": "<user's intention>", "limit": 5}}
```

### Step 5: Check Cohesion
Verify the user's intention aligns with existing work:
```json
{"name": "check_cohesion", "arguments": {"type": "before", "intention": "<user's intention>"}}
```

## Output Guidelines

Report to the user:
1. Cluster connection status and peer count
2. Any warnings that need attention
3. Recent activity from other agents
4. Related context that was found
5. Cohesion check results

If there are conflicts, suggest alternatives or ask how to proceed.

## Example Interaction

User: "I want to start working on the authentication module"

Response:
```
## Cluster Status
Connected with 2 other agents

## Recent Activity
- 30m ago: Agent-A shared context about JWT validation (auth/jwt.go)
- 1h ago: Agent-B released lock on api/middleware.go

## Related Context Found
- "JWT token validation with RS256" (auth/jwt.go) - Agent-A
- "Auth middleware integration" (api/middleware.go) - Agent-B

## Cohesion Check: OK
Your work aligns with existing authentication approach.

Ready to proceed. Remember to use /collab:share when done.
```
