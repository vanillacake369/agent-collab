# Quick Start

Get agent-collab running in 5 minutes.

## 1. Start the Daemon

```bash
agent-collab daemon start
```

The daemon runs in the background and handles P2P networking.

## 2. Create a Cluster

```bash
agent-collab init my-project
```

This outputs an invite token. Share it with teammates who want to join.

## 3. Connect Claude Code

```bash
claude mcp add agent-collab -- agent-collab mcp serve
```

That's it! Your AI agents can now share context and coordinate locks.

## Join an Existing Cluster

If someone already created a cluster, use their invite token:

```bash
agent-collab join <invite-token>
```

## Verify Setup

Check that everything is working:

```bash
# Check daemon status
agent-collab daemon status

# Check cluster status
agent-collab status
```

## What's Next?

- Learn about [MCP Tools](../guide/mcp-tools.md) available to your AI agent
- Configure [Settings](../guide/configuration.md) for your workflow
- Set up [Claude Code Integration](../integrations/claude-code.md) in detail
