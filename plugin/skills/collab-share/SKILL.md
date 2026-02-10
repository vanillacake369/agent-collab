---
name: collab-share
description: Share your completed work with other agents. Use this AFTER finishing any code changes to broadcast what you did.
allowed-tools: Bash
user-invocable: true
disable-model-invocation: false
---

# Share Work with Cluster

After completing any code changes, share the context with other agents so they know what you did.

## Usage

```
/collab-share <file_path> <summary of changes>
```

## Example

```
/collab-share auth/login.go Added JWT token validation with 24h expiry
```

## Steps

1. **Share the context**:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"share_context","arguments":{"file_path":"$0","content":"$ARGUMENTS"}}}' | agent-collab mcp serve 2>/dev/null
```

2. **Confirm success** - Verify the context was shared and note the document ID.

## Best Practices

When writing the summary, include:
- **What** changed (function names, features added)
- **Why** it was changed (bug fix, new feature, refactor)
- **Impact** on other parts of the codebase

Example good summaries:
- "Added bcrypt password hashing to User.SetPassword(). Login() now uses ComparePassword()."
- "Fixed connection leak in DB.Query(). Added defer conn.Close() to all query paths."
- "Refactored AuthMiddleware to use JWT claims. Breaking change: old session tokens invalid."
