# Collaboration Workflow

Best practices for AI agents working together with agent-collab.

## Workflow Overview

```mermaid
flowchart TB
    subgraph Start["1. Start Session"]
        S1[get_warnings]
        S2[get_events]
        S3[search_similar]
    end

    subgraph Work["2. Do Work"]
        W1[acquire_lock]
        W2[Make changes]
        W3[share_context]
        W4[release_lock]
    end

    subgraph Monitor["3. Monitor"]
        M1[Periodic warnings check]
        M2[Watch for conflicts]
    end

    Start --> Work
    Work --> Monitor
    Monitor -.->|If conflict| Start

    style W1 fill:#f87171
    style W4 fill:#4ade80
```

## Starting a Work Session

Always check the environment before starting:

```mermaid
sequenceDiagram
    participant Agent
    participant Collab as agent-collab

    Agent->>Collab: get_warnings()
    Collab-->>Agent: Any conflicts or issues?

    Agent->>Collab: get_events(limit=20)
    Collab-->>Agent: Recent cluster activity

    Agent->>Collab: search_similar("your topic")
    Collab-->>Agent: Related context from other agents
```

### Example: Starting Work on Authentication

```
You: I want to improve the authentication system

Claude: Let me check the collaboration status first.

[Calls get_warnings]
No warnings detected.

[Calls get_events]
Recent activity:
- 30 min ago: gemini-xyz789 released lock on api/routes.go
- 1 hour ago: claude-other shared context about "API rate limiting"

[Calls search_similar("authentication")]
Found related context:
- auth/handler.go: "JWT validation implemented with expiry checking"
- middleware/auth.go: "Auth middleware for route protection"

Based on the existing context, I can see JWT validation is already in place.
Let me acquire a lock before making changes.
```

## Editing Files

### The Lock-Edit-Share-Release Pattern

```mermaid
flowchart LR
    LOCK[1. Lock] --> EDIT[2. Edit] --> SHARE[3. Share] --> RELEASE[4. Release]

    style LOCK fill:#f87171,color:#fff
    style EDIT fill:#fbbf24,color:#000
    style SHARE fill:#4ade80,color:#000
    style RELEASE fill:#60a5fa,color:#fff
```

### Step-by-Step

#### 1. Acquire Lock

```json
{
  "tool": "acquire_lock",
  "arguments": {
    "file_path": "auth/handler.go",
    "start_line": 10,
    "end_line": 50,
    "intention": "Adding refresh token support"
  }
}
```

!!! tip "Lock Best Practices"
    - Lock only the lines you need
    - Be specific with your intention
    - Keep locks as short as possible

#### 2. Make Changes

Edit the code as needed. The lock prevents other agents from modifying the same lines.

#### 3. Share Context

```json
{
  "tool": "share_context",
  "arguments": {
    "file_path": "auth/handler.go",
    "content": "## Changes\n- Added refresh token generation\n- Token rotation on refresh\n- Invalidate old refresh tokens\n\n## Impact\n- Clients need to handle new refresh flow\n- Database schema unchanged (using existing token table)"
  }
}
```

#### 4. Release Lock

```json
{
  "tool": "release_lock",
  "arguments": {
    "lock_id": "lock-abc123"
  }
}
```

## Context Sharing Format

Use a consistent format for shared context:

```markdown
## Changes
- What was changed (bullet points)
- Be specific about modifications

## Reason
Why the change was made

## Impact
What other code might be affected
- List affected components
- Note any breaking changes

## Notes (optional)
Any additional context for other agents
- Gotchas or edge cases
- Related documentation
```

### Good vs Bad Context

```mermaid
flowchart TB
    subgraph Bad["❌ Bad Context"]
        B1["Updated auth"]
        B2["Fixed bug"]
        B3["Refactored code"]
    end

    subgraph Good["✓ Good Context"]
        G1["Added JWT refresh token rotation with 7-day expiry"]
        G2["Fixed null pointer in GetUser when user not found"]
        G3["Extracted auth logic to middleware for route reuse"]
    end
```

## Handling Conflicts

### When Lock Acquisition Fails

```mermaid
flowchart TB
    FAIL[Lock acquisition fails] --> LIST[list_locks]
    LIST --> ANALYZE{Analyze conflict}

    ANALYZE -->|Small overlap| ADJUST[Adjust line range]
    ANALYZE -->|Same area| WAIT[Wait for release]
    ANALYZE -->|Different task| OTHER[Work on other files]
    ANALYZE -->|Urgent| CONTACT[Contact other agent's user]

    ADJUST --> RETRY[Retry acquire_lock]
    WAIT --> RETRY
    OTHER --> SUCCESS[Continue work elsewhere]
```

### Example: Conflict Resolution

```
You: I need to modify auth/handler.go lines 20-60

Claude: Let me try to acquire a lock.

[Calls acquire_lock]
Error: Lock conflict detected
- Existing lock: lines 10-50
- Owner: gemini-xyz789
- Intention: "Refactoring authentication flow"

Options:
1. Wait - The other agent is actively working on auth
2. Adjust range - I could lock lines 51-60 only if my changes are limited there
3. Work elsewhere - Start on a different file

Would you like me to check what other files need work?
```

## Advanced Scenarios

### Multiple File Changes

When your task spans multiple files:

```mermaid
flowchart TB
    PLAN[Plan all changes] --> SEQ{Sequential or Parallel?}

    SEQ -->|Sequential| S1[Lock file 1]
    S1 --> S2[Edit file 1]
    S2 --> S3[Lock file 2]
    S3 --> S4[Edit file 2]
    S4 --> S5[Share context for both]
    S5 --> S6[Release all locks]

    SEQ -->|If independent| P1[Lock all files]
    P1 --> P2[Edit all files]
    P2 --> P3[Share context]
    P3 --> P4[Release all locks]
```

### Long-Running Tasks

For tasks that take more than a few minutes:

```mermaid
flowchart TB
    START[Start task] --> LOCK[Acquire lock]
    LOCK --> WORK[Work...]

    WORK --> CHECK{Every 10 min}
    CHECK --> WARN[get_warnings]
    WARN --> CONFLICT{Conflict?}

    CONFLICT -->|No| WORK
    CONFLICT -->|Yes| PAUSE[Pause work]
    PAUSE --> RESOLVE[Resolve conflict]
    RESOLVE --> WORK

    WORK --> DONE[Complete]
    DONE --> SHARE[Share context]
    SHARE --> RELEASE[Release lock]
```

!!! warning "Long Locks"
    Locks have a TTL (default 30s) and require heartbeats. For long tasks, the agent must maintain the lock with regular heartbeats.

### Building on Other Agents' Work

```mermaid
sequenceDiagram
    participant A as Agent A
    participant B as Agent B
    participant Collab as agent-collab

    A->>Collab: share_context("Added auth middleware")
    Collab-->>B: Context synced

    Note over B: Later, working on related feature

    B->>Collab: search_similar("authentication")
    Collab-->>B: Returns A's context

    B->>B: Reviews A's changes
    B->>Collab: acquire_lock (different area)
    B->>B: Builds on A's work
    B->>Collab: share_context("Extended auth for API keys")
```

## Error Handling

### Common Errors and Solutions

| Error | Cause | Solution |
|-------|-------|----------|
| Lock conflict | Another agent is working there | Wait or work elsewhere |
| Lock expired | TTL exceeded | Re-acquire the lock |
| Daemon unavailable | Daemon not running | Run `agent-collab daemon start` |
| Network error | Peer disconnected | Check connectivity |

### Recovering from Errors

```mermaid
flowchart TB
    ERROR[Error occurs] --> TYPE{Error type?}

    TYPE -->|Lock conflict| C1[list_locks to see details]
    TYPE -->|Lock expired| C2[Re-acquire lock]
    TYPE -->|Network| C3[Check daemon status]
    TYPE -->|Unknown| C4[get_warnings for context]

    C1 --> DECIDE[Decide: wait or work elsewhere]
    C2 --> RETRY[Retry operation]
    C3 --> FIX[Fix connectivity]
    C4 --> ANALYZE[Analyze and retry]
```

## Performance Best Practices

### Lock Optimization

```mermaid
flowchart LR
    subgraph Bad["❌ Bad"]
        B1[Lock entire file<br/>1-500 lines]
    end

    subgraph Good["✓ Good"]
        G1[Lock only needed<br/>45-60 lines]
    end

    Bad -.->|Blocks entire file| CONFLICT[More conflicts]
    Good -.->|Others can work elsewhere| PARALLEL[More parallelism]
```

### Context Efficiency

- **Share promptly** - Don't hold context until the end
- **Be concise** - Include essential info only
- **Use tags** - Help with searchability

### Periodic Monitoring

For long sessions, check periodically:

```
# Every 10 minutes
get_warnings()  # Check for new conflicts
get_events(limit=5)  # See recent activity
```

## Team Workflows

### Two Agents on Same Codebase

```mermaid
flowchart TB
    subgraph AgentA["Agent A: Backend"]
        A1[Works on api/]
        A2[Shares backend context]
    end

    subgraph AgentB["Agent B: Frontend"]
        B1[Works on ui/]
        B2[Shares frontend context]
    end

    subgraph Overlap["Shared Area: types/"]
        O1[Coordinate via locks]
        O2[Share type changes]
    end

    A1 --> O1
    B1 --> O1
    A2 --> O2
    B2 --> O2
```

### Code Review Workflow

```mermaid
sequenceDiagram
    participant Dev as Dev Agent
    participant Review as Review Agent
    participant Collab as agent-collab

    Dev->>Collab: share_context("Implemented feature X")

    Note over Review: Reviews the changes

    Review->>Collab: search_similar("feature X")
    Collab-->>Review: Dev's context

    Review->>Collab: share_context("Review: Suggest error handling improvement")

    Dev->>Collab: search_similar("review")
    Collab-->>Dev: Review's feedback

    Dev->>Collab: acquire_lock
    Dev->>Dev: Apply feedback
    Dev->>Collab: share_context("Applied review feedback")
    Dev->>Collab: release_lock
```

## Quick Reference

### Starting Work
```
1. get_warnings()
2. get_events(limit=20)
3. search_similar("your topic")
```

### Editing Files
```
1. acquire_lock(file, start, end, intention)
2. Make changes
3. share_context(file, description)
4. release_lock(lock_id)
```

### Handling Conflicts
```
1. list_locks() to see active locks
2. Decide: wait, adjust range, or work elsewhere
3. Retry when ready
```
