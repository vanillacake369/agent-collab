package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-collab/src/interface/daemon"
)

// RegisterDaemonTools registers tools that connect to the daemon.
func RegisterDaemonTools(server *Server, client *daemon.Client) {
	// Lock management tools
	server.RegisterTool(Tool{
		Name:        "acquire_lock",
		Description: "IMPORTANT: Call this BEFORE modifying any file to prevent conflicts with other agents. If lock acquisition fails, another agent is working on that area - wait or work on something else.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"file_path": {
					Type:        "string",
					Description: "Path to the file you want to modify",
				},
				"start_line": {
					Type:        "integer",
					Description: "Start line of the region (use 1 for entire file)",
				},
				"end_line": {
					Type:        "integer",
					Description: "End line of the region (use -1 for entire file)",
				},
				"intention": {
					Type:        "string",
					Description: "Brief description of what you plan to do (e.g., 'Add error handling to login function')",
				},
			},
			Required: []string{"file_path", "start_line", "end_line", "intention"},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonAcquireLock(ctx, client, args)
	})

	server.RegisterTool(Tool{
		Name:        "release_lock",
		Description: "Release a previously acquired lock",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"lock_id": {
					Type:        "string",
					Description: "ID of the lock to release",
				},
			},
			Required: []string{"lock_id"},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonReleaseLock(ctx, client, args)
	})

	server.RegisterTool(Tool{
		Name:        "list_locks",
		Description: "List all active locks in the cluster",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonListLocks(ctx, client, args)
	})

	// Context synchronization tools
	server.RegisterTool(Tool{
		Name:        "share_context",
		Description: "IMPORTANT: Call this after completing any code changes to share your work with other agents. This broadcasts what you changed and why, enabling other agents to avoid conflicts and build on your work. Always share context after modifying files.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"file_path": {
					Type:        "string",
					Description: "Path to the file you modified",
				},
				"content": {
					Type:        "string",
					Description: "Summary of changes: what you changed, why, and any impact on other parts of the codebase",
				},
				"metadata": {
					Type:        "object",
					Description: "Additional metadata (e.g., related_files, breaking_changes)",
				},
			},
			Required: []string{"file_path", "content"},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonShareContext(ctx, client, args)
	})

	// Embedding tools
	server.RegisterTool(Tool{
		Name:        "embed_text",
		Description: "Generate embeddings for text using the configured provider",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"text": {
					Type:        "string",
					Description: "Text to embed",
				},
			},
			Required: []string{"text"},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonEmbedText(ctx, client, args)
	})

	server.RegisterTool(Tool{
		Name:        "search_similar",
		Description: "IMPORTANT: Call this before starting work to find relevant context shared by other agents. Search for code patterns, file names, or concepts to discover what others have done and avoid duplicating work or creating conflicts.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "Search query - use keywords like file names, function names, or concepts (e.g., 'authentication handler', 'database connection')",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results (default 10)",
				},
			},
			Required: []string{"query"},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonSearchSimilar(ctx, client, args)
	})

	// Cluster status tools
	server.RegisterTool(Tool{
		Name:        "cluster_status",
		Description: "Get the current status of the agent cluster",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonClusterStatus(ctx, client, args)
	})

	server.RegisterTool(Tool{
		Name:        "list_agents",
		Description: "List all connected agents in the cluster",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonListAgents(ctx, client, args)
	})

	// Event notification tools
	server.RegisterTool(Tool{
		Name:        "get_events",
		Description: "View recent cluster activity - see what other agents have been doing (file changes, lock acquisitions, context shares). Call this periodically during long tasks to stay aware of changes.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"type": {
					Type:        "string",
					Description: "Filter by event type (optional): context.updated (file changes), lock.acquired, lock.conflict",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of events to return (default 10)",
				},
			},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonGetEvents(ctx, client, args)
	})

	server.RegisterTool(Tool{
		Name:        "get_warnings",
		Description: "IMPORTANT: Call this at the START of every task to check for conflicts or relevant updates from other agents. Shows lock conflicts, new context shares, and agent activity that may affect your work.",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleDaemonGetWarnings(ctx, client, args)
	})
}

func handleDaemonAcquireLock(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	filePath, _ := args["file_path"].(string)
	startLine, _ := args["start_line"].(float64)
	endLine, _ := args["end_line"].(float64)
	intention, _ := args["intention"].(string)

	result, err := client.AcquireLock(filePath, int(startLine), int(endLine), intention)
	if err != nil {
		return textResult(fmt.Sprintf("Error acquiring lock: %v", err)), nil
	}

	if !result.Success {
		return textResult(fmt.Sprintf("Lock denied: %s", result.Error)), nil
	}

	return textResult(fmt.Sprintf("Lock acquired successfully. Lock ID: %s", result.LockID)), nil
}

func handleDaemonReleaseLock(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	lockID, _ := args["lock_id"].(string)

	if err := client.ReleaseLock(lockID); err != nil {
		return textResult(fmt.Sprintf("Error releasing lock: %v", err)), nil
	}

	return textResult(fmt.Sprintf("Lock %s released successfully", lockID)), nil
}

func handleDaemonListLocks(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	result, err := client.ListLocks()
	if err != nil {
		return textResult(fmt.Sprintf("Error listing locks: %v", err)), nil
	}

	if len(result.Locks) == 0 {
		return textResult("No active locks"), nil
	}

	data, _ := json.MarshalIndent(result.Locks, "", "  ")
	return textResult(string(data)), nil
}

func handleDaemonShareContext(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	filePath, _ := args["file_path"].(string)
	content, _ := args["content"].(string)

	// Extract metadata if provided
	var metadata map[string]any
	if m, ok := args["metadata"].(map[string]any); ok {
		metadata = m
	}

	if content == "" {
		return textResult("Error: content is required for sharing context"), nil
	}

	// Share context via daemon (stores in VectorDB and broadcasts to peers)
	result, err := client.ShareContext(filePath, content, metadata)
	if err != nil {
		return textResult(fmt.Sprintf("Error sharing context: %v", err)), nil
	}

	return textResult(fmt.Sprintf("Context shared successfully. %s (Document ID: %s)",
		result.Message, result.DocumentID)), nil
}

func handleDaemonEmbedText(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	text, _ := args["text"].(string)

	result, err := client.Embed(text)
	if err != nil {
		return textResult(fmt.Sprintf("Error generating embedding: %v", err)), nil
	}

	return textResult(fmt.Sprintf("Generated embedding with %d dimensions using %s provider (%s model)",
		result.Dimension, result.Provider, result.Model)), nil
}

func handleDaemonSearchSimilar(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	query, _ := args["query"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	result, err := client.Search(query, limit)
	if err != nil {
		return textResult(fmt.Sprintf("Error searching: %v", err)), nil
	}

	if len(result.Results) == 0 {
		return textResult("No similar content found"), nil
	}

	data, _ := json.MarshalIndent(result.Results, "", "  ")
	return textResult(string(data)), nil
}

func handleDaemonClusterStatus(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	status, err := client.Status()
	if err != nil {
		return textResult(fmt.Sprintf("Error getting cluster status: %v", err)), nil
	}

	data, _ := json.MarshalIndent(status, "", "  ")
	return textResult(string(data)), nil
}

func handleDaemonListAgents(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	result, err := client.ListAgents()
	if err != nil {
		return textResult(fmt.Sprintf("Error listing agents: %v", err)), nil
	}

	if len(result.Agents) == 0 {
		return textResult("No agents connected"), nil
	}

	data, _ := json.MarshalIndent(result.Agents, "", "  ")
	return textResult(string(data)), nil
}

func handleDaemonGetEvents(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	eventType := ""
	if t, ok := args["type"].(string); ok {
		eventType = t
	}

	result, err := client.ListEvents(limit, eventType)
	if err != nil {
		return textResult(fmt.Sprintf("Error getting events: %v", err)), nil
	}

	if len(result.Events) == 0 {
		return textResult("No recent events"), nil
	}

	// Format events for better readability
	var output string
	output = fmt.Sprintf("Recent cluster events (%d):\n\n", len(result.Events))

	for _, event := range result.Events {
		output += fmt.Sprintf("- [%s] %s\n", event.Type, event.Timestamp.Format("15:04:05"))

		// Add event-specific details
		switch event.Type {
		case daemon.EventLockAcquired:
			var data daemon.LockEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				output += fmt.Sprintf("    File: %s, Agent: %s, Intention: %s\n", data.FilePath, data.AgentID, data.Intention)
			}
		case daemon.EventLockReleased:
			var data daemon.LockEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				output += fmt.Sprintf("    File: %s, Agent: %s\n", data.FilePath, data.AgentID)
			}
		case daemon.EventLockConflict:
			var data daemon.LockConflictData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				output += fmt.Sprintf("    File: %s, Requested by: %s, Held by: %s\n", data.FilePath, data.RequesterID, data.HolderID)
			}
		case daemon.EventContextUpdated:
			var data daemon.ContextEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				msg := fmt.Sprintf("    File: %s", data.FilePath)
				if data.AgentID != "" {
					msg += fmt.Sprintf(", From: %s", data.AgentID)
				}
				output += msg + "\n"
			}
		case daemon.EventAgentJoined:
			var data daemon.AgentEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				output += fmt.Sprintf("    Agent: %s (%s)\n", data.Name, data.Provider)
			}
		case daemon.EventPeerConnected:
			var data daemon.PeerEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				output += fmt.Sprintf("    Peer: %s\n", data.PeerID)
			}
		}
	}

	return textResult(output), nil
}

func handleDaemonGetWarnings(ctx context.Context, client *daemon.Client, args map[string]any) (*ToolCallResult, error) {
	// Get recent important events that might affect the current agent's work
	result, err := client.ListEvents(20, "")
	if err != nil {
		return textResult(fmt.Sprintf("Error getting warnings: %v", err)), nil
	}

	if len(result.Events) == 0 {
		return textResult("No pending warnings"), nil
	}

	// Filter for important warning-worthy events
	var warnings []string
	for _, event := range result.Events {
		switch event.Type {
		case daemon.EventLockConflict:
			var data daemon.LockConflictData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				warnings = append(warnings, fmt.Sprintf("⚠️ Lock conflict on %s: held by %s", data.FilePath, data.HolderID))
			}
		case daemon.EventLockAcquired:
			var data daemon.LockEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				warnings = append(warnings, fmt.Sprintf("🔒 Lock acquired on %s by %s: %s", data.FilePath, data.AgentID, data.Intention))
			}
		case daemon.EventAgentJoined:
			var data daemon.AgentEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				warnings = append(warnings, fmt.Sprintf("👋 New agent joined: %s (%s)", data.Name, data.Provider))
			}
		case daemon.EventContextUpdated:
			var data daemon.ContextEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				msg := "📄 Context shared"
				if data.FilePath != "" {
					msg += ": " + data.FilePath
				}
				if data.AgentID != "" {
					msg += " from " + data.AgentID
				}
				warnings = append(warnings, msg)
			}
		case daemon.EventPeerConnected:
			var data daemon.PeerEventData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				warnings = append(warnings, fmt.Sprintf("🔗 Peer connected: %s", data.PeerID))
			}
		case daemon.EventDaemonShutdown:
			warnings = append(warnings, "⛔ Daemon is shutting down")
		}
	}

	if len(warnings) == 0 {
		return textResult("No pending warnings"), nil
	}

	output := "Cluster warnings:\n"
	for _, w := range warnings {
		output += "- " + w + "\n"
	}
	return textResult(output), nil
}
