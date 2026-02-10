# CLI Reference

Complete command reference for agent-collab.

## Global Flags

| Flag | Description |
|------|-------------|
| `--help`, `-h` | Show help |
| `--version`, `-v` | Show version |

## Commands

### agent-collab init

Create a new cluster.

```bash
agent-collab init <project-name>
```

**Arguments:**

- `project-name` — Name for the cluster

**Output:** Invite token for others to join

---

### agent-collab join

Join an existing cluster.

```bash
agent-collab join <token>
```

**Arguments:**

- `token` — Invite token from cluster creator

---

### agent-collab leave

Leave the current cluster.

```bash
agent-collab leave
```

---

### agent-collab status

Show cluster status.

```bash
agent-collab status
```

**Output:**

- Cluster name
- Connected peers
- Your node ID
- Uptime

---

### agent-collab lock list

List active locks.

```bash
agent-collab lock list
```

**Output:** Table of active locks with:

- Lock ID
- File path
- Line range
- Owner
- Intention
- Created time

---

### agent-collab lock release

Release a lock.

```bash
agent-collab lock release <lock-id>
```

**Arguments:**

- `lock-id` — ID of the lock to release

---

### agent-collab lock history

Show recent lock activity.

```bash
agent-collab lock history
```

---

### agent-collab daemon start

Start the background daemon.

```bash
agent-collab daemon start
```

---

### agent-collab daemon stop

Stop the daemon.

```bash
agent-collab daemon stop
```

---

### agent-collab daemon status

Check daemon status.

```bash
agent-collab daemon status
```

---

### agent-collab token show

Display invite token.

```bash
agent-collab token show
```

---

### agent-collab token usage

Show API token usage.

```bash
agent-collab token usage
```

**Output:**

- Daily usage
- Daily limit
- Reset time

---

### agent-collab config show

Display configuration.

```bash
agent-collab config show
```

---

### agent-collab config set

Set a configuration value.

```bash
agent-collab config set <key> <value>
```

**Arguments:**

- `key` — Configuration key
- `value` — New value

---

### agent-collab mcp serve

Start MCP server.

```bash
agent-collab mcp serve
```

Used for AI agent integration. Typically run as:

```bash
claude mcp add agent-collab -- agent-collab mcp serve
```
