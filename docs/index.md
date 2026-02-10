# agent-collab

**P2P distributed collaboration for AI agents**

Share context and coordinate work across your team without a central server.

---

## Why agent-collab?

When multiple AI agents work on the same codebase, conflicts happen. One agent modifies a file while another is analyzing it. Changes get overwritten. Context is lost.

`agent-collab` solves this with:

| Feature | Description |
|---------|-------------|
| **No Server Required** | Direct P2P communication via libp2p |
| **Semantic Locks** | Prevent conflicts with intention-based locking |
| **Context Sharing** | Keep all agents in sync with CRDT-based synchronization |
| **MCP Integration** | Connect Claude Code, Gemini CLI, or any MCP-compatible agent |

## Who Should Use This?

- **Teams using multiple AI agents** on the same codebase
- **Developers with Claude Code + other AI tools** working in parallel
- **Organizations** wanting to coordinate AI-assisted development

## How It Works

```mermaid
flowchart TB
    subgraph MachineA["Developer Machine A"]
        CC[Claude Code<br/>MCP Client]
        AC1[agent-collab<br/>daemon]
        VDB1[(Vector DB)]
        CC -->|MCP| AC1
        AC1 <--> VDB1
    end

    subgraph MachineB["Developer Machine B"]
        GC[Gemini CLI<br/>MCP Client]
        AC2[agent-collab<br/>daemon]
        VDB2[(Vector DB)]
        GC -->|MCP| AC2
        AC2 <--> VDB2
    end

    AC1 <-->|P2P<br/>libp2p + GossipSub| AC2

    style CC fill:#6366f1,color:#fff
    style GC fill:#10b981,color:#fff
    style AC1 fill:#8b5cf6,color:#fff
    style AC2 fill:#8b5cf6,color:#fff
```

### Core Concepts

```mermaid
flowchart LR
    subgraph Locks["Semantic Locks"]
        L1[Agent A locks<br/>auth/handler.go:10-50]
        L2[Agent B sees lock,<br/>works elsewhere]
    end

    subgraph Context["Context Sharing"]
        C1[Agent A shares:<br/>'Added JWT validation']
        C2[Agent B receives<br/>context update]
    end

    L1 --> L2
    C1 --> C2
```

**Semantic Locks** prevent conflicts by tracking *intent*. Before editing `auth/handler.go`, an agent acquires a lock explaining what it plans to do. Other agents see this and work elsewhere.

**Context Sync** uses CRDTs to share knowledge across the cluster. When one agent learns something about the codebase, all agents benefit.

## Quick Example

```bash
# 1. Start the daemon
agent-collab daemon start

# 2. Create a cluster
agent-collab init -p my-project
# Outputs: Invite token: abc123...

# 3. Connect Claude Code
claude mcp add agent-collab -- agent-collab mcp serve

# Done! Your AI agents can now collaborate.
```

## Next Steps

<div class="grid cards" markdown>

- :material-download: **[Installation](getting-started/installation.md)**

    Install agent-collab on your system

- :material-rocket-launch: **[Quick Start](getting-started/quick-start.md)**

    Get up and running in 5 minutes

- :material-sitemap: **[Architecture](concepts/architecture.md)**

    Understand how agent-collab works

- :material-tools: **[MCP Tools](guide/mcp-tools.md)**

    Learn about available MCP tools

</div>
