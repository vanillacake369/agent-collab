---
name: collab-check
description: Use when user says "check cluster", "누가 작업중", "what's happening", "클러스터 상태", "find context", "search for", "컨텍스트 검색", "who is working on", "락 확인", "list locks", or wants to see cluster status and search for context shared by other agents.
version: 1.0.0
---

# Check Cluster and Search Context

Check the cluster status and search for context shared by other agents.

## When to Use

- When user asks about cluster status
- When searching for related work by other agents
- When checking who is working on what
- When looking for specific context or information

## Workflow

### Step 1: Check Cluster Status
```json
{"name": "cluster_status", "arguments": {}}
```

### Step 2: List Active Locks
```json
{"name": "list_locks", "arguments": {}}
```

### Step 3: Search for Context (if query provided)
```json
{"name": "search_similar", "arguments": {"query": "<user's query>", "limit": 10}}
```

## Output Format

```
## Cluster Status
- Status: Connected/Disconnected
- Peers: N agents online
- Node ID: xxx

## Active Locks
| File | Agent | Intention | Duration |
|------|-------|-----------|----------|
| auth/handler.go | Agent-A | JWT validation | 5m |

## Search Results (if query provided)
| Relevance | File | Summary | Agent |
|-----------|------|---------|-------|
| 0.95 | auth/jwt.go | JWT token validation | Agent-A |
| 0.82 | api/middleware.go | Auth middleware | Agent-B |
```

## Search Tips

Effective search queries:
- File names: "auth.go", "database"
- Function names: "Login", "ValidateToken"
- Concepts: "authentication", "error handling"
- Korean: "인증", "데이터베이스"
