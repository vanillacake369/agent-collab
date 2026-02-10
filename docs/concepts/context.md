# Context Sharing

How agents share knowledge and stay synchronized.

## What is Context?

Context is knowledge that agents share with each other:

- What changes were made to a file
- Why those changes were made
- How changes affect other parts of the codebase
- Insights discovered during analysis

```mermaid
flowchart LR
    subgraph Context["Shared Context"]
        FILE[file: auth/handler.go]
        CONTENT[content: Added JWT validation<br/>with expiry check]
        META[metadata: security, auth]
        EMBED[embedding: vector[384]]
    end
```

## Context Flow

```mermaid
sequenceDiagram
    participant A as Agent A
    participant VA as Vector DB A
    participant P2P as P2P Network
    participant VB as Vector DB B
    participant B as Agent B

    Note over A: Completes work on auth

    A->>VA: share_context("auth/handler.go", "Added JWT...")
    VA->>VA: Generate embedding
    VA->>P2P: Broadcast context
    P2P->>VB: Replicate
    VB->>VB: Store with embedding

    Note over B: Later, working on related code

    B->>VB: search_similar("authentication")
    VB-->>B: Returns related contexts
```

## Semantic Search

Context is stored with vector embeddings, enabling semantic search:

```mermaid
flowchart TB
    subgraph Query["Search Query"]
        Q[How does authentication work?]
    end

    subgraph Embedding["Embedding"]
        QE[Query Vector<br/>dim=384]
    end

    subgraph VectorDB["Vector Database"]
        V1[Context 1: JWT validation]
        V2[Context 2: Database setup]
        V3[Context 3: Auth middleware]
    end

    subgraph Results["Similar Results"]
        R1[1. Auth middleware<br/>score: 0.92]
        R2[2. JWT validation<br/>score: 0.87]
    end

    Q --> QE
    QE --> VectorDB
    VectorDB --> Results
```

### Embedding Providers

| Provider | Model | Dimensions |
|----------|-------|------------|
| OpenAI | text-embedding-3-small | 1536 |
| Anthropic | voyage-2 | 1024 |
| Google AI | text-embedding-004 | 768 |
| Ollama | nomic-embed-text | 768 |

## CRDT Synchronization

Context uses CRDTs (Conflict-free Replicated Data Types) for synchronization:

```mermaid
flowchart TB
    subgraph Peer1["Peer A"]
        C1A[Context 1]
        C2A[Context 2]
    end

    subgraph Peer2["Peer B"]
        C1B[Context 1]
        C3B[Context 3]
    end

    subgraph Merge["After Sync"]
        C1[Context 1]
        C2[Context 2]
        C3[Context 3]
    end

    Peer1 -->|Sync| Merge
    Peer2 -->|Sync| Merge

    note[All contexts merged<br/>without conflicts]
```

### Benefits of CRDTs

- **No conflicts** - Concurrent updates merge automatically
- **Offline support** - Work offline, sync when connected
- **Partition tolerance** - Network splits don't cause issues

## Sharing Context

### Via MCP Tool

```json
// Request
{
  "tool": "share_context",
  "arguments": {
    "file_path": "auth/handler.go",
    "content": "## Changes\n- Added JWT token validation\n- Checks expiry time\n- Validates signature\n\n## Impact\nAll auth endpoints now require valid tokens"
  }
}

// Response
{
  "success": true,
  "context_id": "ctx-abc123"
}
```

### Best Practices for Content

!!! tip "Structure your context"
    Use a consistent format:
    ```markdown
    ## Changes
    - What was changed

    ## Reason
    Why the change was made

    ## Impact
    What other code might be affected
    ```

!!! tip "Be specific"
    "Added JWT validation with expiry check" is better than "Updated auth"

## Searching Context

### Via MCP Tool

```json
// Request
{
  "tool": "search_similar",
  "arguments": {
    "query": "How is authentication handled?",
    "limit": 5
  }
}

// Response
{
  "results": [
    {
      "file_path": "auth/handler.go",
      "content": "Added JWT token validation...",
      "similarity": 0.92,
      "created_at": "2024-01-15T10:30:00Z",
      "agent": "claude-abc123"
    },
    {
      "file_path": "middleware/auth.go",
      "content": "Implemented auth middleware...",
      "similarity": 0.87,
      "created_at": "2024-01-15T09:15:00Z",
      "agent": "gemini-xyz789"
    }
  ]
}
```

## Context Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Created: share_context()
    Created --> Embedded: Generate embedding
    Embedded --> Stored: Save to Vector DB
    Stored --> Synced: Broadcast to peers

    Synced --> Searchable: Available for queries
    Searchable --> Searchable: search_similar()

    Searchable --> Archived: Old context
    Archived --> [*]: Cleanup
```

## Data Storage

Context is stored in two places:

```mermaid
flowchart LR
    subgraph Storage["~/.agent-collab/"]
        subgraph VDB["vectors/"]
            EMB[Embeddings<br/>HNSW Index]
            META[Metadata<br/>file, time, agent]
        end

        subgraph BDB["badger/"]
            RAW[Raw Content]
            IDX[Content Index]
        end
    end

    EMB <--> META
    META <--> RAW
```

## Context vs Locks

| Aspect | Locks | Context |
|--------|-------|---------|
| **Purpose** | Prevent conflicts | Share knowledge |
| **Lifetime** | Short (seconds) | Long (persistent) |
| **Scope** | File + line range | File + content |
| **Sync** | Real-time required | Eventually consistent |

```mermaid
flowchart TB
    subgraph Workflow
        LOCK[1. Acquire Lock]
        WORK[2. Do Work]
        SHARE[3. Share Context]
        RELEASE[4. Release Lock]
    end

    LOCK --> WORK
    WORK --> SHARE
    SHARE --> RELEASE

    style LOCK fill:#f87171
    style SHARE fill:#4ade80
```

## Monitoring Context

### Get Events

```json
// Request
{
  "tool": "get_events",
  "arguments": {
    "type": "context.updated",
    "limit": 10
  }
}

// Response
{
  "events": [
    {
      "type": "context.updated",
      "file_path": "auth/handler.go",
      "agent": "claude-abc123",
      "timestamp": "2024-01-15T10:30:00Z"
    }
  ]
}
```

### Supported Event Types

| Event | Description |
|-------|-------------|
| `context.updated` | New context shared |
| `lock.acquired` | Lock obtained |
| `lock.released` | Lock released |
| `lock.conflict` | Lock conflict detected |
| `agent.joined` | New agent connected |
| `peer.connected` | New peer in cluster |
