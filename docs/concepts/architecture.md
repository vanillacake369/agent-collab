# Architecture

Understanding how agent-collab works under the hood.

## System Overview

```mermaid
flowchart TB
    subgraph Clients["AI Agent Clients"]
        CC[Claude Code]
        GC[Gemini CLI]
        CA[Custom Agent]
    end

    subgraph Daemon["agent-collab daemon"]
        MCP[MCP Server<br/>stdio]
        API[API Server]
        EB[Event Bus]

        subgraph Controllers
            LC[Lock Controller]
            CTC[Context Controller]
            AC[Agent Controller]
        end

        subgraph Domain
            LS[Lock Service]
            CS[Context Service]
            AS[Agent Registry]
        end
    end

    subgraph Infra["Infrastructure"]
        P2P[libp2p Network]
        WG[WireGuard VPN]
        VDB[(Vector DB)]
        BDB[(BadgerDB)]
    end

    CC -->|MCP| MCP
    GC -->|MCP| MCP
    CA -->|MCP| MCP

    MCP --> API
    API --> EB
    EB --> Controllers
    Controllers --> Domain
    Domain --> Infra

    P2P <-->|GossipSub| OtherPeers[Other Peers]
```

## Layered Architecture

agent-collab follows a clean architecture pattern:

```mermaid
flowchart TB
    subgraph Interface["Interface Layer"]
        CLI[CLI Commands]
        MCP[MCP Server]
        TUI[TUI Dashboard]
        DAEMON[Daemon Server]
    end

    subgraph Application["Application Layer"]
        INIT[Init Service]
        JOIN[Join Service]
        LOCK[Lock Service]
        STATUS[Status Service]
    end

    subgraph Domain["Domain Layer"]
        AGENT[Agent]
        LOCKD[Lock]
        CONTEXT[Context]
        PEER[Peer]
        TOKEN[Token]
    end

    subgraph Infrastructure["Infrastructure Layer"]
        NET[Network<br/>libp2p/WireGuard]
        STORE[Storage<br/>BadgerDB]
        EMBED[Embedding<br/>OpenAI/Ollama]
        CRYPTO[Crypto<br/>Node Keys]
    end

    Interface --> Application
    Application --> Domain
    Domain --> Infrastructure

    style Interface fill:#6366f1,color:#fff
    style Application fill:#8b5cf6,color:#fff
    style Domain fill:#a855f7,color:#fff
    style Infrastructure fill:#c084fc,color:#fff
```

### Layer Responsibilities

| Layer | Responsibility | Examples |
|-------|---------------|----------|
| **Interface** | User interaction | CLI, MCP Server, TUI |
| **Application** | Use case orchestration | Init cluster, Join cluster |
| **Domain** | Business logic | Lock management, Context sync |
| **Infrastructure** | Technical details | P2P networking, Database |

## P2P Network

agent-collab uses [libp2p](https://libp2p.io/) for peer-to-peer communication:

```mermaid
flowchart LR
    subgraph Cluster["agent-collab Cluster"]
        P1[Peer A<br/>12D3Koo...]
        P2[Peer B<br/>12D3Koo...]
        P3[Peer C<br/>12D3Koo...]
    end

    P1 <-->|GossipSub| P2
    P2 <-->|GossipSub| P3
    P3 <-->|GossipSub| P1

    subgraph Topics["PubSub Topics"]
        T1[/agent-collab/locks]
        T2[/agent-collab/context]
        T3[/agent-collab/events]
    end
```

### Key Components

- **GossipSub**: Efficient message propagation across peers
- **DHT**: Distributed hash table for peer discovery
- **mDNS**: Local network peer discovery
- **NAT Traversal**: Automatic hole punching for firewalled networks

## Data Flow

### Lock Acquisition Flow

```mermaid
sequenceDiagram
    participant Agent as Claude Code
    participant MCP as MCP Server
    participant LC as Lock Controller
    participant LS as Lock Service
    participant P2P as P2P Network
    participant Other as Other Peers

    Agent->>MCP: acquire_lock(file, lines, intention)
    MCP->>LC: HandleAcquireLock()
    LC->>LS: TryAcquire()

    alt Lock Available
        LS->>P2P: Broadcast lock
        P2P->>Other: GossipSub publish
        LS-->>LC: Lock acquired
        LC-->>MCP: Success + lock_id
        MCP-->>Agent: {"lock_id": "..."}
    else Lock Conflict
        LS-->>LC: Conflict with existing lock
        LC-->>MCP: Error
        MCP-->>Agent: {"error": "Lock conflict"}
    end
```

### Context Sharing Flow

```mermaid
sequenceDiagram
    participant A as Agent A
    participant MCP1 as MCP Server A
    participant CS1 as Context Service A
    participant VDB1 as Vector DB A
    participant P2P as P2P Network
    participant CS2 as Context Service B
    participant VDB2 as Vector DB B

    A->>MCP1: share_context(file, content)
    MCP1->>CS1: Store context
    CS1->>VDB1: Generate embedding
    VDB1-->>CS1: Stored
    CS1->>P2P: Broadcast context
    P2P->>CS2: Receive context
    CS2->>VDB2: Store with embedding
    CS2-->>P2P: ACK
```

## Storage Architecture

```mermaid
flowchart LR
    subgraph Storage["~/.agent-collab/"]
        KEY[key.json<br/>Node Identity]

        subgraph BadgerDB["badger/"]
            LOCKS[Locks]
            AGENTS[Agents]
            CONFIG[Config]
            METRICS[Metrics]
        end

        subgraph VectorDB["vectors/"]
            CTX[Context Embeddings]
            IDX[HNSW Index]
        end

        SOCK[daemon.sock<br/>IPC Socket]
        PID[daemon.pid]
    end
```

### Data Stores

| Store | Purpose | Technology |
|-------|---------|------------|
| **BadgerDB** | Persistent key-value storage | Embedded Go database |
| **Vector DB** | Semantic search | HNSW index with embeddings |
| **IPC Socket** | Daemon communication | Unix domain socket |

## Event System

```mermaid
flowchart TB
    subgraph Events["Event Types"]
        E1[lock.acquired]
        E2[lock.released]
        E3[lock.conflict]
        E4[context.updated]
        E5[agent.joined]
        E6[peer.connected]
    end

    subgraph Bus["Event Bus"]
        PUB[Publisher]
        SUB[Subscribers]
    end

    subgraph Handlers["Event Handlers"]
        H1[TUI Update]
        H2[MCP Notification]
        H3[P2P Broadcast]
    end

    Events --> PUB
    PUB --> SUB
    SUB --> Handlers
```

## WireGuard Integration (Optional)

For secure, encrypted communication:

```mermaid
flowchart TB
    subgraph WG["WireGuard VPN"]
        WG1[Peer A<br/>10.100.0.1]
        WG2[Peer B<br/>10.100.0.2]
        WG3[Peer C<br/>10.100.0.3]
    end

    WG1 <-->|Encrypted Tunnel| WG2
    WG2 <-->|Encrypted Tunnel| WG3
    WG1 <-->|Encrypted Tunnel| WG3

    subgraph P2P["libp2p over WireGuard"]
        L1[agent-collab A]
        L2[agent-collab B]
        L3[agent-collab C]
    end

    WG1 --- L1
    WG2 --- L2
    WG3 --- L3
```

Enable WireGuard with:

```bash
agent-collab init -p my-project --wireguard
```

## Component Dependencies

```mermaid
graph TD
    CLI --> Daemon
    MCP --> Daemon
    TUI --> Daemon

    Daemon --> LockController
    Daemon --> ContextController
    Daemon --> AgentController

    LockController --> LockService
    ContextController --> ContextService
    AgentController --> AgentRegistry

    LockService --> BadgerDB
    LockService --> P2P

    ContextService --> VectorDB
    ContextService --> EmbeddingProvider
    ContextService --> P2P

    AgentRegistry --> BadgerDB

    subgraph External
        EmbeddingProvider --> OpenAI
        EmbeddingProvider --> Ollama
        EmbeddingProvider --> GoogleAI
    end
```
