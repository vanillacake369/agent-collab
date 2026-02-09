package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-collab/internal/interface/daemon"
)

// RegisterDaemonTools registers tools that connect to the daemon.
func RegisterDaemonTools(server *Server, client *daemon.Client) {
	// Lock management tools
	server.RegisterTool(Tool{
		Name:        "acquire_lock",
		Description: "Acquire a semantic lock on a code region to prevent conflicts",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"file_path": {
					Type:        "string",
					Description: "Path to the file",
				},
				"start_line": {
					Type:        "integer",
					Description: "Start line of the region",
				},
				"end_line": {
					Type:        "integer",
					Description: "End line of the region",
				},
				"intention": {
					Type:        "string",
					Description: "What you intend to do with this region",
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
		Description: "Share context information with other agents in the cluster",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"file_path": {
					Type:        "string",
					Description: "Path to the file being worked on",
				},
				"content": {
					Type:        "string",
					Description: "Content or summary to share",
				},
				"metadata": {
					Type:        "object",
					Description: "Additional metadata",
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
		Description: "Search for similar content in the vector store",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "Search query",
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
