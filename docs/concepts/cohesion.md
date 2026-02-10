# Cohesion Checking

How agents verify their work aligns with team context.

## What is Cohesion?

Cohesion checking ensures that an agent's intended work or completed result aligns with existing team context. It helps prevent conflicting changes when multiple agents work on the same codebase.

```mermaid
flowchart TB
    subgraph Before["Before Work"]
        B1[User gives task]
        B2[Agent checks cohesion]
        B3{Conflicts?}
        B4[Warn user]
        B5[Proceed]
    end

    subgraph After["After Work"]
        A1[Agent completes work]
        A2[Agent checks cohesion]
        A3{Conflicts?}
        A4[Notify about changes]
        A5[Share context]
    end

    B1 --> B2 --> B3
    B3 -->|Yes| B4 --> B5
    B3 -->|No| B5

    A1 --> A2 --> A3
    A3 -->|Yes| A4 --> A5
    A3 -->|No| A5
```

## Why Check Cohesion?

### Scenario: Conflicting Approaches

```
Agent A (Seoul): "Implemented JWT authentication"
Agent B (US): User asks "Switch to session-based auth"

Without cohesion check:
→ Agent B implements sessions
→ Conflicts with Agent A's JWT work
→ Inconsistent codebase

With cohesion check:
→ Agent B detects existing JWT context
→ Warns user about potential conflict
→ User decides how to proceed
```

## How It Works

### Before Check

When starting work, the agent checks if their intention conflicts with existing context:

```mermaid
sequenceDiagram
    participant User
    participant Agent
    participant MCP as agent-collab
    participant Store as Vector Store

    User->>Agent: "Switch to session auth"
    Agent->>MCP: check_cohesion(before, "session auth")
    MCP->>Store: Search similar contexts
    Store-->>MCP: "JWT auth implemented"
    MCP-->>Agent: Conflict detected
    Agent->>User: "Existing JWT work found. Proceed?"
```

### After Check

After completing work, the agent verifies the result aligns with team decisions:

```mermaid
sequenceDiagram
    participant Agent
    participant MCP as agent-collab
    participant Store as Vector Store
    participant Other as Other Agents

    Agent->>MCP: check_cohesion(after, "replaced JWT")
    MCP->>Store: Search similar contexts
    Store-->>MCP: "JWT auth implemented"
    MCP-->>Agent: Breaking change detected
    Agent->>MCP: share_context("JWT replaced with sessions")
    MCP->>Other: Broadcast context
```

## Using check_cohesion

### MCP Tool

```json
// Before check
{
  "tool": "check_cohesion",
  "arguments": {
    "type": "before",
    "intention": "Implement session-based authentication"
  }
}

// After check
{
  "tool": "check_cohesion",
  "arguments": {
    "type": "after",
    "result": "Replaced JWT with session cookies",
    "files_changed": ["auth/handler.go", "auth/session.go"]
  }
}
```

### Response Format

```json
{
  "verdict": "conflict",
  "confidence": 0.85,
  "related_contexts": [
    {
      "id": "ctx-abc123",
      "agent": "Agent-A",
      "file_path": "auth/handler.go",
      "content": "Implemented JWT-based authentication",
      "similarity": 0.89
    }
  ],
  "potential_conflicts": [
    {
      "context": { ... },
      "reason": "Conflicting authentication approach",
      "severity": "high"
    }
  ],
  "suggestions": [
    "Review the related contexts before proceeding",
    "Consider discussing with the team",
    "Share context after completing work"
  ],
  "message": "Potential conflict detected with 1 existing context(s)"
}
```

## Verdict Types

| Verdict | Meaning | Action |
|---------|---------|--------|
| `cohesive` | Work aligns with existing context | Safe to proceed |
| `conflict` | Potential conflict detected | Review and confirm with user |
| `uncertain` | Unable to determine | Proceed with caution |

## Conflict Detection

### Conflict Indicators

Words that suggest potential conflicts:
- "instead", "replace", "remove", "switch to"
- "migrate", "deprecated", "no longer"
- Korean: "대신", "변경", "제거", "전환"

### Opposing Patterns

Automatically detected conflicts:
- JWT ↔ Session (authentication approach)
- REST ↔ GraphQL (API style)
- SQL ↔ NoSQL (database approach)
- Monolith ↔ Microservice (architecture)
- Sync ↔ Async (execution model)

## Best Practices

### 1. Check Before Starting

```
/collab-start implement new auth system
```

Always check cohesion before significant work.

### 2. Be Specific

Good:
```
"Switch authentication from JWT to session-based with Redis storage"
```

Bad:
```
"Change auth"
```

### 3. Share After Changes

Even if no conflict was detected, share context:
```
/collab-share auth/handler.go Replaced JWT with sessions, Redis for session storage
```

### 4. Respect Conflicts

When a conflict is detected:
1. Review the existing context
2. Discuss with user if this is intentional
3. If proceeding, share context to inform others

## Integration with Workflow

```mermaid
flowchart TB
    START[Start Task] --> CHECK1["/collab-start"]
    CHECK1 --> COHESION1{Cohesive?}
    COHESION1 -->|Yes| WORK[Do Work]
    COHESION1 -->|Conflict| DISCUSS[Discuss with User]
    DISCUSS -->|Proceed| WORK
    DISCUSS -->|Abort| END[End]
    WORK --> CHECK2["/collab-share"]
    CHECK2 --> COHESION2{Cohesive?}
    COHESION2 -->|Yes| SHARE[Share Context]
    COHESION2 -->|Conflict| WARN[Warn & Share]
    WARN --> SHARE
    SHARE --> END
```

## Technical Details

### Similarity Thresholds

| Threshold | Value | Meaning |
|-----------|-------|---------|
| High | 0.85 | Strong relevance |
| Medium | 0.70 | Moderate relevance |
| Low | 0.55 | Weak relevance |

### Conflict Severity

| Severity | Trigger |
|----------|---------|
| High | High similarity + conflict indicators |
| Medium | Opposing patterns detected |
| Low | Related context in same area |

## Examples

### Example 1: Safe to Proceed

```
Intention: "Add JWT refresh token support"
Existing: "JWT authentication implemented"

Result: cohesive
Message: "Your work aligns with existing context"
```

### Example 2: Conflict Detected

```
Intention: "Switch to GraphQL API"
Existing: "REST API with JSON responses"

Result: conflict
Reason: "Conflicting API style"
Severity: high
```

### Example 3: Direction Change

```
Intention: "Migrate from SQL to MongoDB"
Existing: "PostgreSQL schema with foreign keys"

Result: conflict
Suggestions:
- "This is a significant architecture change"
- "Ensure team alignment before proceeding"
```
