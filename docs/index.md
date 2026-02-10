# agent-collab

**P2P distributed collaboration for AI agents**

Share context and coordinate work across your team without a central server.

---

## Why agent-collab?

When multiple AI agents work on the same codebase, conflicts happen. `agent-collab` solves this with:

- **No Server Required** — Direct P2P communication via libp2p
- **Semantic Locks** — Prevent conflicts before they happen with intention-based locking
- **Context Sharing** — Keep all agents in sync with CRDT-based synchronization
- **MCP Integration** — Connect Claude Code, Gemini CLI, or any MCP-compatible agent

## How It Works

```
┌─────────────────────────────────────────────────────────┐
│                  Developer Machine A                     │
│  ┌─────────────┐    ┌──────────────┐    ┌───────────┐  │
│  │ Claude Code │───▶│ agent-collab │◀──▶│ Vector DB │  │
│  │   (MCP)     │    └──────┬───────┘    └───────────┘  │
│  └─────────────┘           │                            │
└────────────────────────────┼────────────────────────────┘
                             │ P2P (libp2p + GossipSub)
┌────────────────────────────┼────────────────────────────┐
│                  Developer Machine B                     │
│  ┌─────────────┐    ┌──────┴───────┐    ┌───────────┐  │
│  │ Gemini CLI  │───▶│ agent-collab │◀──▶│ Vector DB │  │
│  │   (MCP)     │    └──────────────┘    └───────────┘  │
│  └─────────────┘                                        │
└─────────────────────────────────────────────────────────┘
```

**Semantic Locks** prevent conflicts by tracking *intent*. Before editing `auth/handler.go`, an agent acquires a lock explaining what it plans to do. Other agents see this and work elsewhere.

**Context Sync** uses CRDTs to share knowledge across the cluster. When one agent learns something about the codebase, all agents benefit.

## Next Steps

<div class="grid cards" markdown>

- :material-download: **[Installation](getting-started/installation.md)**
  Install agent-collab on your system

- :material-rocket-launch: **[Quick Start](getting-started/quick-start.md)**
  Get up and running in 5 minutes

- :material-cog: **[Configuration](guide/configuration.md)**
  Customize agent-collab for your workflow

- :material-tools: **[MCP Tools](guide/mcp-tools.md)**
  Learn about available MCP tools

</div>
