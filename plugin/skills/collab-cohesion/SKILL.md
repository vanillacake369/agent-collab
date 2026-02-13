---
name: collab-cohesion
description: Use when user says "check alignment", "정합성 확인", "does this conflict", "충돌 확인", "before I start", "작업 전 확인", "after I'm done", "will this break", "is this consistent", or wants to verify their work aligns with team context to prevent conflicts.
version: 1.0.0
---

# Check Work Cohesion

Verify your work aligns with existing team context to prevent conflicts with other agents.

## When to Use

### Before Starting Work
- When planning significant changes
- When the task might affect shared code or architecture
- Examples: "refactor authentication", "change database schema"

### After Completing Work
- Before sharing context
- When making significant changes
- To verify changes don't conflict with team decisions

## Workflow

### For "Before" Check
```json
{"name": "check_cohesion", "arguments": {"type": "before", "intention": "<what user plans to do>"}}
```

### For "After" Check
```json
{"name": "check_cohesion", "arguments": {"type": "after", "intention": "<what was done>", "file_path": "<modified file>"}}
```

## Interpreting Results

### Cohesive (Safe)
```
## Cohesion Check: OK

Your work aligns with existing context.

Related contexts:
- Agent-A: "JWT token validation" (auth/handler.go)

Safe to proceed.
```

### Conflict Detected
```
## Cohesion Check: CONFLICT

Your intention may conflict with existing work:

Your plan: "Switch to session-based authentication"

Conflicts with:
- Agent-A: "JWT-based stateless authentication" (auth/handler.go)
  Reason: Different authentication approach

Suggestions:
1. Review existing JWT implementation first
2. Discuss with team if direction change is intended
3. If proceeding, share context to inform other agents
```

### Uncertain
```
## Cohesion Check: UNCERTAIN

No direct conflicts found, but limited context available.

Proceed with caution and share context when done.
```

## Recommended Workflow

```
1. /collab:start                    # Check cluster
2. /collab:cohesion before: ...     # Verify plan
3. [Do the work]
4. /collab:cohesion after: ...      # Verify result
5. /collab:share                    # Share with team
```

## Example

User: "I'm going to refactor the authentication to use sessions instead of JWT"

Before check response:
```
## Cohesion Check: CONFLICT

Your intention "refactor to session-based auth" may conflict with:
- Agent-A (2h ago): "Implemented JWT RS256 token validation"
  File: auth/jwt.go
  Summary: Added stateless JWT authentication

This appears to be a significant direction change.

Options:
1. Review Agent-A's work first
2. Proceed if this is an intentional change (inform team)
3. Cancel and discuss with team
```
