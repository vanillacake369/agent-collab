---
description: Manually acquire or release a lock on a file
argument-hint: <acquire|release> <file_path> [intention]
allowed-tools: [Bash]
---

# /collab:lock

Manually manage locks on files for collaborative editing.

## Usage

```
/collab:lock acquire main.go implementing auth handler
/collab:lock release main.go
/collab:lock list
```

## Arguments

Parse from: $ARGUMENTS
- Action: acquire, release, or list
- file_path: path to file (for acquire/release)
- intention: description of work (for acquire)

## Instructions

### For "list"
```
Call MCP tool: list_locks
Arguments: {}
```

### For "acquire"
```
Call MCP tool: acquire_lock
Arguments: {
  "file_path": "<file_path>",
  "start_line": 1,
  "end_line": 9999,
  "intention": "<intention>"
}
```

### For "release"
First get lock_id:
```
Call MCP tool: list_locks
Arguments: {}
```

Then release:
```
Call MCP tool: release_lock
Arguments: {"lock_id": "<lock_id>"}
```

## Output Format

### List
```
## Active Locks
| File | Agent | Intention | Duration |
|------|-------|-----------|----------|
```

### Acquire Success
```
## Lock Acquired
- File: <file_path>
- Lock ID: <lock_id>
- Intention: <intention>

Remember to release when done: /collab:lock release <file_path>
```

### Acquire Failed
```
## Lock Failed
- File: <file_path>
- Held by: <agent_id>
- Since: <time>
- Their intention: <their_intention>

Consider working on a different file or waiting.
```
