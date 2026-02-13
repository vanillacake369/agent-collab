---
name: collab-share
description: Use when user says "share my work", "tell others", "공유", "다른 에이전트에게 알려", "broadcast changes", "finished working on", "completed", "작업 완료", "done with", or wants to share completed work with other agents in the cluster.
version: 1.0.0
---

# Share Work with Team

This skill helps you share your completed work with other agents so they can see what you did and avoid conflicts.

## When to Use

- After completing work on a file
- When the user says "share", "tell others", "broadcast", "공유"
- When the user mentions they finished a task
- When you want to inform the team about changes

## Workflow

### Step 1: Identify What to Share
Determine the file path and summary from user's message or recent edits.

### Step 2: Check Cohesion (Post-work)
Verify the completed work aligns with team direction:
```json
{"name": "check_cohesion", "arguments": {"type": "after", "intention": "<summary>", "file_path": "<file_path>"}}
```

### Step 3: Share Context
Broadcast the work to the cluster:
```json
{"name": "share_context", "arguments": {"file_path": "<file_path>", "content": "<detailed summary>"}}
```

### Step 4: Release Lock
If holding a lock on this file, release it:
```json
{"name": "list_locks", "arguments": {}}
```

Then:
```json
{"name": "release_lock", "arguments": {"lock_id": "<lock_id>"}}
```

## Content Guidelines

When sharing context, include:
1. **What changed** - Brief description of modifications
2. **Why** - Reason for the changes
3. **How to use** - If you added functions/APIs, how should others call them
4. **Dependencies** - Any new imports or requirements

Good summary examples:
- "Added bcrypt password hashing to User.SetPassword(). Login() now uses ComparePassword()."
- "Fixed connection leak in DB.Query(). Added defer conn.Close() to all query paths."
- "Refactored AuthMiddleware to use JWT claims. Breaking change: old session tokens invalid."

## Example Interaction

User: "Share that I added JWT validation to the auth handler"

Response:
```
## Context Shared

File: auth/handler.go
Summary: Added JWT token validation with RS256 signing

Content shared:
- Added `ValidateToken(token string) (*Claims, error)` function
- Checks token expiry and signature
- Returns parsed claims on success
- Requires `JWT_PUBLIC_KEY` environment variable

## Cohesion: OK
Aligns with existing authentication approach.

## Lock Released
Released lock on auth/handler.go

Other agents will now see this context when searching for authentication-related work.
```

## Handling Conflicts

If cohesion check shows conflict:

```
## Cohesion Warning

Your change may conflict with existing context:
- Agent-A: "JWT-based stateless authentication" (auth/handler.go)
  Reason: Conflicting authentication approach

This appears to be a significant direction change.
Do you want to share anyway? This will inform other agents of the change.
```

Ask user to confirm before proceeding.
