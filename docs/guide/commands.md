# CLI Commands

Complete reference of agent-collab CLI commands.

## Cluster Commands

### Initialize a Cluster

```bash
agent-collab init <project>
```

Creates a new cluster and outputs an invite token.

### Join a Cluster

```bash
agent-collab join <token>
```

Join an existing cluster using an invite token.

### Leave a Cluster

```bash
agent-collab leave
```

Leave the current cluster.

### Cluster Status

```bash
agent-collab status
```

Show cluster status and connected peers.

## Lock Commands

### List Locks

```bash
agent-collab lock list
```

Show all active locks in the cluster.

### Release a Lock

```bash
agent-collab lock release <id>
```

Manually release a lock by ID.

### Lock History

```bash
agent-collab lock history
```

Show recent lock activity.

## Daemon Commands

### Start Daemon

```bash
agent-collab daemon start
```

Start the background daemon.

### Stop Daemon

```bash
agent-collab daemon stop
```

Stop the running daemon.

### Daemon Status

```bash
agent-collab daemon status
```

Check if the daemon is running.

## Token Commands

### Show Token

```bash
agent-collab token show
```

Display the current cluster invite token.

### Token Usage

```bash
agent-collab token usage
```

Show API token usage statistics.

## Config Commands

### Show Config

```bash
agent-collab config show
```

Display current configuration.

### Set Config

```bash
agent-collab config set <key> <value>
```

Set a configuration value.

## MCP Server

### Serve MCP

```bash
agent-collab mcp serve
```

Start the MCP server for AI agent integration.
