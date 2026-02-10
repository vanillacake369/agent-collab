# How Locks Work

Semantic locks are the core mechanism for preventing conflicts between AI agents.

## What is a Semantic Lock?

A semantic lock is more than just a file lock. It captures:

- **What** file and line range is being modified
- **Why** the agent needs to modify it (intention)
- **Who** holds the lock (agent ID)
- **When** the lock was acquired and expires

```mermaid
flowchart LR
    subgraph Lock["Semantic Lock"]
        FILE[file: auth/handler.go]
        LINES[lines: 10-50]
        INTENT[intention: Adding JWT validation]
        OWNER[owner: claude-abc123]
        TTL[ttl: 30s]
    end
```

## Lock Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Requested: acquire_lock()
    Requested --> Active: Lock granted
    Requested --> Rejected: Conflict exists

    Active --> Active: Heartbeat extends TTL
    Active --> Released: release_lock()
    Active --> Expired: TTL exceeded

    Released --> [*]
    Expired --> [*]
    Rejected --> [*]
```

### States

| State | Description |
|-------|-------------|
| **Requested** | Agent requests a lock |
| **Active** | Lock is held by agent |
| **Released** | Agent explicitly released |
| **Expired** | TTL exceeded without heartbeat |
| **Rejected** | Conflicting lock exists |

## Conflict Detection

```mermaid
flowchart TB
    subgraph File["auth/handler.go"]
        R1[Lines 1-9]
        R2[Lines 10-30<br/>Lock A: JWT validation]
        R3[Lines 31-50]
        R4[Lines 51-100]
    end

    subgraph Requests
        REQ1[Request: Lines 25-40]
        REQ2[Request: Lines 60-80]
    end

    REQ1 -->|CONFLICT| R2
    REQ2 -->|OK| R4

    style R2 fill:#ef4444,color:#fff
    style REQ1 fill:#fca5a5
    style REQ2 fill:#86efac
```

Locks conflict when:

1. **Same file** AND
2. **Overlapping line ranges**

Non-overlapping regions in the same file can be locked by different agents.

## Lock Acquisition Flow

```mermaid
sequenceDiagram
    participant Agent as AI Agent
    participant Local as Local Lock Service
    participant P2P as P2P Network
    participant Remote as Remote Peers

    Agent->>Local: acquire_lock(file, 10, 50, "JWT validation")

    Local->>Local: Check local locks

    alt No local conflict
        Local->>P2P: Request distributed lock
        P2P->>Remote: Broadcast lock request

        alt No remote conflict
            Remote-->>P2P: ACK
            P2P-->>Local: Lock confirmed
            Local->>Local: Store lock
            Local-->>Agent: Success (lock_id)
        else Remote conflict
            Remote-->>P2P: NACK (conflict info)
            P2P-->>Local: Conflict detected
            Local-->>Agent: Error (conflict details)
        end
    else Local conflict exists
        Local-->>Agent: Error (conflict details)
    end
```

## Heartbeat Mechanism

Locks require periodic heartbeats to stay active:

```mermaid
sequenceDiagram
    participant Agent as AI Agent
    participant Lock as Lock Service
    participant Timer as TTL Timer

    Agent->>Lock: acquire_lock()
    Lock->>Timer: Start TTL (30s)
    Lock-->>Agent: lock_id

    loop Every 10s
        Agent->>Lock: heartbeat(lock_id)
        Lock->>Timer: Reset TTL
    end

    alt Agent disconnects
        Timer->>Timer: TTL expires
        Timer->>Lock: Expire lock
        Lock->>Lock: Remove lock
    end
```

### Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `lock.default_ttl` | 30s | Lock time-to-live |
| `lock.heartbeat_interval` | 10s | Heartbeat frequency |

## Viewing Active Locks

### CLI

```bash
$ agent-collab lock list

┌─────────┬──────────────────────┬───────────┬─────────────────────────┬─────────────────────┐
│ ID      │ File                 │ Lines     │ Intention               │ Owner               │
├─────────┼──────────────────────┼───────────┼─────────────────────────┼─────────────────────┤
│ lock-1  │ auth/handler.go      │ 10-50     │ Adding JWT validation   │ claude-abc123       │
│ lock-2  │ db/connection.go     │ 100-150   │ Fixing connection pool  │ gemini-xyz789       │
└─────────┴──────────────────────┴───────────┴─────────────────────────┴─────────────────────┘
```

### MCP Tool

```json
// Request
{"tool": "list_locks"}

// Response
{
  "locks": [
    {
      "id": "lock-1",
      "file_path": "auth/handler.go",
      "start_line": 10,
      "end_line": 50,
      "intention": "Adding JWT validation",
      "owner": "claude-abc123",
      "created_at": "2024-01-15T10:30:00Z",
      "expires_at": "2024-01-15T10:30:30Z"
    }
  ]
}
```

## Handling Lock Conflicts

When a lock conflict occurs:

```mermaid
flowchart TB
    START[acquire_lock fails]

    START --> LIST[list_locks to see who]
    LIST --> DECIDE{Decision}

    DECIDE -->|Wait| RETRY[Retry later]
    DECIDE -->|Work elsewhere| OTHER[Lock different region]
    DECIDE -->|Urgent| FORCE[Force release<br/>⚠️ Use with caution]

    RETRY --> SUCCESS[Lock acquired]
    OTHER --> SUCCESS
    FORCE --> SUCCESS
```

### Best Practices

!!! tip "Keep locks small"
    Only lock the lines you're actually modifying. Smaller locks = fewer conflicts.

!!! tip "Be specific with intentions"
    Clear intentions help other agents understand and avoid your work area.

!!! warning "Release promptly"
    Don't hold locks longer than necessary. Release as soon as you're done.

## Lock History

View recent lock activity:

```bash
$ agent-collab lock history

Recent lock activity (last 10):
┌─────────────────────┬──────────────────────┬───────────────┬─────────────────────────┐
│ Time                │ File                 │ Action        │ Agent                   │
├─────────────────────┼──────────────────────┼───────────────┼─────────────────────────┤
│ 10:45:23            │ auth/handler.go      │ acquired      │ claude-abc123           │
│ 10:42:15            │ auth/handler.go      │ released      │ claude-abc123           │
│ 10:30:00            │ db/connection.go     │ acquired      │ gemini-xyz789           │
└─────────────────────┴──────────────────────┴───────────────┴─────────────────────────┘
```

## Distributed Consensus

Locks are synchronized across all peers using a consensus protocol:

```mermaid
flowchart TB
    subgraph Peer1["Peer A"]
        L1[Lock Store]
    end

    subgraph Peer2["Peer B"]
        L2[Lock Store]
    end

    subgraph Peer3["Peer C"]
        L3[Lock Store]
    end

    L1 <-->|CRDT Sync| L2
    L2 <-->|CRDT Sync| L3
    L3 <-->|CRDT Sync| L1

    note[All peers have<br/>consistent lock state]
```

The CRDT-based approach ensures:

- **Eventual consistency** across all peers
- **Partition tolerance** - works even with network splits
- **No single point of failure** - any peer can process locks
