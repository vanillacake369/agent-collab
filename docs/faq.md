# FAQ

Frequently asked questions about agent-collab.

## General

### What is agent-collab?

agent-collab is a P2P distributed collaboration system for AI agents. It helps multiple AI agents (like Claude Code, Gemini CLI) work on the same codebase without conflicts.

### Do I need a server?

No. agent-collab uses peer-to-peer networking via libp2p. All agents communicate directly with each other.

### What AI agents are supported?

Any MCP-compatible agent works with agent-collab:

- Claude Code
- Gemini CLI
- Custom MCP clients

## Setup

### The daemon won't start

1. Check if another instance is running:
   ```bash
   agent-collab daemon status
   ```

2. Check for port conflicts (default: 4001):
   ```bash
   lsof -i :4001
   ```

3. Try a different port:
   ```bash
   agent-collab config set network.listen_port 4002
   agent-collab daemon start
   ```

### Can't connect to cluster

1. Verify the invite token is correct
2. Check network connectivity to other peers
3. Ensure firewall allows port 4001 (or your configured port)
4. Try regenerating the token:
   ```bash
   agent-collab token show
   ```

### MCP tools not showing in Claude Code

1. Ensure daemon is running
2. Re-add the MCP server:
   ```bash
   claude mcp remove agent-collab
   claude mcp add agent-collab -- agent-collab mcp serve
   ```

## Locks

### What happens if I forget to release a lock?

Locks have a TTL (default: 30 seconds) and require heartbeats. If an agent disconnects without releasing, the lock expires automatically.

### Can I force-release someone else's lock?

Yes, but use with caution:

```bash
agent-collab lock release <lock-id>
```

### How do I prevent lock conflicts?

- Keep locks small (only the lines you're modifying)
- Be specific with intentions
- Release locks as soon as you're done

## Context

### How is context stored?

Context is stored locally in `~/.agent-collab/vectors/` and synchronized across peers using CRDTs.

### Is my code sent to external servers?

Only if you configure an external embedding provider (OpenAI, etc.). With Ollama, everything stays local.

### How do I clear old context?

```bash
rm -rf ~/.agent-collab/vectors/
agent-collab daemon restart
```

## Performance

### How many agents can collaborate?

The P2P network scales well with dozens of agents. Performance depends on network conditions between peers.

### Does it work across different networks?

Yes, if peers can reach each other. For NAT traversal, ensure at least one peer has a public IP or use a relay.

## Troubleshooting

### Reset everything

```bash
agent-collab daemon stop
rm -rf ~/.agent-collab/
agent-collab daemon start
agent-collab init my-project
```

### View logs

Check daemon logs for debugging:

```bash
agent-collab daemon status --verbose
```

### Report issues

Found a bug? Report it at [GitHub Issues](https://github.com/vanillacake369/agent-collab/issues).
