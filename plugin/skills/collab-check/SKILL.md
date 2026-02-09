---
name: collab-check
description: Check cluster status and search for context shared by other agents. Use to find relevant work or verify cluster connectivity.
allowed-tools: Bash
user-invocable: true
disable-model-invocation: false
---

# Check Cluster and Search Context

Check the cluster status and search for context shared by other agents.

## Usage

```
/collab-check                    # Check cluster status only
/collab-check <search query>     # Search for related context
```

## Steps

1. **Check cluster status**:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cluster_status","arguments":{}}}' | agent-collab mcp serve 2>/dev/null
```

2. **Search for context** (if query provided):
```bash
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_similar","arguments":{"query":"$ARGUMENTS","limit":10}}}' | agent-collab mcp serve 2>/dev/null
```

## Output

Report:
- Cluster connection status (running, peer count)
- Project name
- Search results with relevance scores
- Source of each result (which agent shared it)

## Tips

Search queries that work well:
- File names: "auth.go", "database connection"
- Function names: "Login", "HashPassword"
- Concepts: "authentication", "error handling"
- Keywords from error messages or requirements
