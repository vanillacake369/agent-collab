# agent-collab Plugin for Claude Code

Multi-agent collaboration tools for Claude Code. Enables context sharing, lock management, cohesion checking, and event notifications across multiple Claude instances via P2P network.

## Features

- **Context Sharing**: Share your work with other Claude agents via P2P network
- **Cohesion Checking**: Verify your work aligns with team context before and after changes
- **Lock Management**: Prevent conflicts by acquiring locks before editing files
- **Event Notifications**: See what other agents are doing in real-time
- **Semantic Search**: Find related context shared by other agents
- **Automatic Hooks**: Auto-detect collaboration needs and inject protocol

## Installation

### Prerequisites

1. Install agent-collab daemon:
```bash
# macOS
brew install agent-collab

# or from source
go install github.com/vanillacake369/agent-collab/cmd/agent-collab@latest
```

2. Start the daemon and join/create a cluster:
```bash
# Create a new cluster
agent-collab daemon start
agent-collab init --project my-project

# Or join existing cluster
agent-collab daemon start
agent-collab join <invite-token>
```

### Install Plugin

```bash
# Install from marketplace
/plugin install agent-collab

# Or install directly from git
/plugin install https://github.com/vanillacake369/agent-collab --subdir plugin
```

## Commands

| Command | Description |
|---------|-------------|
| `/collab:start [work]` | Start session - check cluster, warnings, related context |
| `/collab:share <file> <summary>` | Share completed work with team |
| `/collab:check [query]` | Check cluster status and search context |
| `/collab:lock <action> <file>` | Manage file locks (acquire/release/list) |
| `/collab:status` | Show detailed cluster and daemon status |

## Skills (Auto-invoked)

Skills are automatically invoked by Claude when relevant keywords are detected:

| Skill | Triggers |
|-------|----------|
| `collab-start` | "start collaboration", "협업 시작", "check team status" |
| `collab-share` | "share my work", "공유", "finished working" |
| `collab-check` | "check cluster", "누가 작업중", "search for" |
| `collab-cohesion` | "check alignment", "정합성 확인", "does this conflict" |

## Hooks

The plugin automatically injects collaboration context:

| Hook | Trigger | Action |
|------|---------|--------|
| `UserPromptSubmit` | Work-related prompts | Inject collaboration protocol |
| `SessionStart` | Session begins | Check cluster, show status |
| `SessionEnd` | Session ends | Release locks, notify cluster |
| `PreToolUse` | Edit/Write tools | Acquire file lock |
| `PostToolUse` | Edit/Write complete | Remind to share context |
| `SubagentStart` | Agent spawned | Track in cluster |
| `SubagentStop` | Agent finished | Share results |

## MCP Tools

| Tool | Description |
|------|-------------|
| `cluster_status` | Check cluster connection and peer count |
| `acquire_lock` | Lock a file region before editing |
| `release_lock` | Release a held lock |
| `list_locks` | See what files are locked |
| `share_context` | Share work summary with cluster |
| `search_similar` | Find related context via semantic search |
| `check_cohesion` | Verify work aligns with team direction |
| `get_warnings` | Get alerts about conflicts or changes |
| `get_events` | View recent cluster activity |

## Workflow

### Recommended Flow

```
1. /collab:start <what you plan to do>
   └── Checks cluster, finds related context, verifies cohesion

2. Work on the code
   └── Locks are auto-acquired when editing (via hooks)

3. /collab:share <file> <summary>
   └── Checks cohesion, shares context, releases lock
```

### Example Session

```
You: /collab:start implement JWT authentication

Claude: ## Cluster Status
Connected with 2 other agents

## Recent Activity
- 30m ago: Agent-A shared context about user model (models/user.go)

## Related Context Found
- "User model with password hashing" (models/user.go) - Agent-A

## Cohesion Check: OK
Your work aligns with existing context. Ready to proceed.

---

[... work on code ...]

---

You: /collab:share auth/jwt.go Added JWT token validation with RS256

Claude: ## Context Shared
File: auth/jwt.go
Summary: Added JWT token validation with RS256

## Cohesion: OK
Lock released. Other agents will see your work.
```

## Keyword Detection

The plugin automatically detects collaboration-related prompts:

**Work Keywords** (trigger collaboration protocol):
- English: implement, create, add, modify, fix, refactor
- Korean: 구현, 작성, 추가, 수정, 변경, 리팩터

**Collaboration Keywords** (trigger explicit checks):
- English: collaborate, share, team, conflict
- Korean: 협업, 공유, 팀, 충돌

**Before-Work Keywords** (trigger pre-checks):
- "before I start", "check first", "작업 전", "먼저 확인"

**Completion Keywords** (trigger sharing):
- "done", "finished", "완료", "공유해"

## Configuration

### MCP Server

The plugin registers the MCP server automatically. Manual config:

```json
{
  "mcpServers": {
    "agent-collab": {
      "command": "agent-collab",
      "args": ["mcp", "serve"]
    }
  }
}
```

## Troubleshooting

### "Not connected to cluster"
```bash
agent-collab daemon status  # Check daemon
agent-collab daemon start   # Start if not running
```

### "Lock acquisition failed"
Another agent is editing that file. Use `/collab:check` to see who.

### "Cohesion conflict detected"
Your work may conflict with existing team decisions. Options:
1. Review the conflicting context
2. Proceed if this is an intentional change
3. Discuss with team first

## Plugin Structure

```
plugin/
├── .claude-plugin/
│   └── plugin.json          # Plugin manifest
├── .mcp.json                 # MCP server config
├── commands/                 # User-invocable commands
│   ├── start.md
│   ├── share.md
│   ├── check.md
│   ├── lock.md
│   └── status.md
├── skills/                   # Auto-invoked skills
│   ├── collab-start/
│   ├── collab-share/
│   ├── collab-check/
│   └── collab-cohesion/
├── hooks/                    # Event hooks
│   ├── hooks.json
│   ├── collab-detector.mjs
│   ├── session-start.mjs
│   ├── session-end.mjs
│   ├── pre-tool-enforcer.mjs
│   ├── post-tool-reminder.mjs
│   └── subagent-tracker.mjs
└── README.md
```

## License

MIT
