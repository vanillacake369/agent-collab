# Collaboration Workflow

Best practices for AI agents working together with agent-collab.

## Basic Workflow

### 1. Start of Work Session

Always check the current state before starting work:

```
1. get_warnings()      → Check for conflicts
2. get_events()        → See recent activity
3. search_similar()    → Find related context
```

### 2. Before Editing Files

Always acquire a lock before modifying code:

```
acquire_lock(
  file_path: "auth/handler.go",
  start_line: 10,
  end_line: 50,
  intention: "Adding JWT token validation"
)
```

!!! warning "Important"
    Never edit files without acquiring a lock first. This prevents conflicts with other agents.

### 3. After Completing Work

Share what you did and release the lock:

```
1. share_context()     → Document your changes
2. release_lock()      → Free the region for others
```

## Context Sharing Format

When sharing context, use a structured format:

```markdown
## Changes
- Added JWT token validation
- Updated error handling for expired tokens

## Reason
Security requirement: all API endpoints need token validation

## Impact
- All auth endpoints now require valid tokens
- Existing clients need to update their authentication flow
```

## Example: Feature Implementation

```
# 1. Check environment
get_warnings()
search_similar("authentication")

# 2. Acquire lock
acquire_lock("auth/handler.go", 10, 50, "JWT token validation logic")

# 3. Make changes
... edit code ...

# 4. Share and release
share_context("auth/handler.go", "Added JWT validation: expiry check, signature verification")
release_lock(lock_id)
```

## Example: Bug Fix

```
# 1. Investigate
get_events(limit=20)
search_similar("error handling database")

# 2. Lock the area
acquire_lock("db/connection.go", 100, 150, "Connection pool leak fix")

# 3. Fix the bug
... edit code ...

# 4. Document and release
share_context("db/connection.go", "Fixed connection pool leak: added defer for connection return")
release_lock(lock_id)
```

## Handling Lock Conflicts

If lock acquisition fails:

1. **Check who has the lock:**
   ```
   list_locks()
   ```

2. **Options:**
   - Work on a different area
   - Wait and retry later
   - Report the conflict to the user

## Periodic Checks

For long-running tasks, check for conflicts every 10 minutes:

```
get_warnings()
```

This ensures you're aware of any changes that might affect your work.

## Tips

- **Be specific** with lock intentions — helps other agents understand your work
- **Keep locks small** — only lock the lines you're actually modifying
- **Share context generously** — future agents (including yourself) will benefit
- **Release locks promptly** — don't block others longer than necessary
