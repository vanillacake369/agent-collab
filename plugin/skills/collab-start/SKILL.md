---
name: collab-start
description: Start a collaborative work session. Checks for warnings and recent activity from other agents before beginning work. Use this at the START of any task.
allowed-tools: Bash
user-invocable: true
disable-model-invocation: false
---

# Start Collaborative Session

Before starting any work, check the cluster for:
1. Warnings about conflicts or other agent activity
2. Recent events from other agents
3. Related context that might affect your work

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

## Output

Report to the user:
- Any warnings that need attention
- Recent activity from other agents that might be relevant
- Related context that was found

If there are lock conflicts, suggest working on a different file or waiting.
