---
name: collab-start
description: Start a collaborative work session. Checks for warnings, recent activity, and cohesion with team context before beginning work. Use this at the START of any task.
allowed-tools: Bash
user-invocable: true
disable-model-invocation: false
---

# Start Collaborative Session

Before starting any work, check the cluster and verify your work aligns with team context.

## Steps

1. **Check warnings** - Call `get_warnings` to see if there are any conflicts or important updates:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_warnings","arguments":{}}}' | agent-collab mcp serve 2>/dev/null
```

2. **Get recent events** - Call `get_events` to see what other agents have been doing:
```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_events","arguments":{"limit":10}}}' | agent-collab mcp serve 2>/dev/null
```

3. **Search for related context** - If working on a specific feature, search for related work:
```bash
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_similar","arguments":{"query":"$ARGUMENTS","limit":5}}}' | agent-collab mcp serve 2>/dev/null
```

4. **Check cohesion** - If the user has described their intended work, verify it aligns with existing context:
```bash
echo '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"check_cohesion","arguments":{"type":"before","intention":"$ARGUMENTS"}}}' | agent-collab mcp serve 2>/dev/null
```

## Output

Report to the user:
- Any warnings that need attention
- Recent activity from other agents that might be relevant
- Related context that was found
- **Cohesion check result** - whether the intended work aligns with team context

If there are lock conflicts, suggest working on a different file or waiting.

If cohesion check shows potential conflicts:
- Show the conflicting contexts
- Ask user to confirm if they want to proceed
- Suggest discussing with the team if it's a significant direction change

## Example

User: `/collab-start refactor authentication to use sessions`

Response:
```
📡 Cluster Status: Connected (3 agents online)

⚠️ Cohesion Alert:
Your intention "refactor authentication to use sessions" may conflict with:
- Agent-A: "JWT-based authentication implemented" (auth/handler.go)

Recent Activity:
- 30m ago: Agent-A shared context about JWT validation
- 1h ago: Agent-B released lock on api/routes.go

Do you want to proceed? If this is an intentional direction change,
consider using /collab-share after completing work to inform the team.
```
