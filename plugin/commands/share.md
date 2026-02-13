---
description: Share your completed work with other agents in the cluster
argument-hint: <file_path> <summary>
allowed-tools: [Bash]
---

# /collab:share

Share context about your completed work so other agents can see what you did.

## What This Command Does

1. **Check cohesion** - Verify your work aligns with team direction
2. **Share context** - Broadcast your work summary to the cluster
3. **Release lock** - If you have a lock on the file, release it

## Usage

```
/collab:share auth/handler.go Added JWT token validation with expiry check
/collab:share main.go 인증 미들웨어 추가
```

## Arguments

Parse from: $ARGUMENTS
- First word: file_path
- Remaining: summary of changes

## Instructions

### 1. Parse Arguments
Extract file_path and summary from user input.

### 2. Check Cohesion (Post-work)
```
Call MCP tool: check_cohesion
Arguments: {"type": "after", "intention": "<summary>", "file_path": "<file_path>"}
```

### 3. Share Context
```
Call MCP tool: share_context
Arguments: {
  "file_path": "<file_path>",
  "content": "<summary>\n\nChanges made by this agent."
}
```

### 4. Release Lock (if held)
```
Call MCP tool: list_locks
Arguments: {}
```

If this agent holds a lock on the file:
```
Call MCP tool: release_lock
Arguments: {"lock_id": "<lock_id>"}
```

## Output Format

```
## Context Shared

File: <file_path>
Summary: <summary>

## Cohesion Status
- [Alignment check result]

## Lock Status
- [Released / No lock held]

Other agents will now see your work when they search for related context.
```
