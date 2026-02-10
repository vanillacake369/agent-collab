---
name: collab-cohesion
description: Check if your work aligns with team context. Use BEFORE starting work to check intention, or AFTER completing work to verify alignment. Helps prevent conflicting changes with other agents.
allowed-tools: Bash
user-invocable: true
disable-model-invocation: false
---

# Check Work Cohesion

Ensure your work aligns with existing team context to prevent conflicts.

## When to Use

### Before Starting Work (Recommended)
When you receive a task that might affect shared code or architecture:
- "Change authentication to session-based"
- "Refactor the API layer"
- "Migrate database to NoSQL"

### After Completing Work
After making significant changes, verify they align with team decisions.

## Usage

### Before Check
```
/collab-cohesion before: implement session-based authentication
```

### After Check
```
/collab-cohesion after: replaced JWT with session tokens in auth/handler.go
```

## How It Works

1. **Before Check**: Analyzes your intention against existing shared contexts
   - Finds related work by other agents
   - Detects potential conflicts (e.g., JWT vs Session)
   - Suggests reviewing related contexts before proceeding

2. **After Check**: Validates completed work against existing contexts
   - Identifies if your changes conflict with previous decisions
   - Recommends sharing context to inform other agents

## Steps

1. **Parse the check type and content** from the user's input

2. **Call check_cohesion**:
```bash
# For 'before' check
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"check_cohesion","arguments":{"type":"before","intention":"$INTENTION"}}}' | agent-collab mcp serve 2>/dev/null

# For 'after' check
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"check_cohesion","arguments":{"type":"after","result":"$RESULT"}}}' | agent-collab mcp serve 2>/dev/null
```

3. **Interpret the result**:
   - `cohesive`: Safe to proceed
   - `conflict`: Review related contexts, discuss with user
   - `uncertain`: Proceed with caution, consider sharing context

## Example Output

### Cohesive (Safe)
```
✓ Your work aligns with existing context.

Related contexts found:
- Agent-A: "JWT token validation implemented" (auth/handler.go)

You can proceed safely. Consider sharing context when done.
```

### Conflict Detected
```
⚠ Potential conflict detected!

Your intention: "Switch to session-based authentication"

Conflicts with:
- Agent-A: "JWT-based stateless authentication" (auth/handler.go)
  Reason: Conflicting authentication approach

Suggestions:
1. Review the existing JWT implementation
2. Discuss with team if this direction change is intended
3. If proceeding, share context to inform other agents
```

## Integration with Workflow

Recommended workflow:
1. `/collab-start` - Check cluster status
2. `/collab-cohesion before: ...` - Verify intention alignment
3. Work on the task
4. `/collab-cohesion after: ...` - Verify result alignment
5. `/collab-share` - Share your completed work
