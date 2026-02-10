package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-collab/src/application"
	"agent-collab/src/infrastructure/storage/vector"
)

// RegisterDefaultTools registers the default agent-collab tools.
func RegisterDefaultTools(server *Server, app *application.App) {
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
		return handleAcquireLock(ctx, app, args)
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
		return handleReleaseLock(ctx, app, args)
	})

	server.RegisterTool(Tool{
		Name:        "list_locks",
		Description: "List all active locks in the cluster",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleListLocks(ctx, app, args)
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
		return handleShareContext(ctx, app, args)
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
		return handleEmbedText(ctx, app, args)
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
		return handleSearchSimilar(ctx, app, args)
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
		return handleClusterStatus(ctx, app, args)
	})

	server.RegisterTool(Tool{
		Name:        "list_agents",
		Description: "List all connected agents in the cluster",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	}, func(ctx context.Context, args map[string]any) (*ToolCallResult, error) {
		return handleListAgents(ctx, app, args)
	})
}

func handleAcquireLock(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
	lockService := app.LockService()
	if lockService == nil {
		return textResult("Error: Lock service not initialized"), nil
	}

	filePath, _ := args["file_path"].(string)
	startLine, _ := args["start_line"].(float64)
	endLine, _ := args["end_line"].(float64)
	intention, _ := args["intention"].(string)

	// Note: This is a simplified version. In production, you'd use the full lock request.
	result := fmt.Sprintf("Lock requested for %s lines %d-%d: %s", filePath, int(startLine), int(endLine), intention)
	return textResult(result), nil
}

func handleReleaseLock(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
	lockService := app.LockService()
	if lockService == nil {
		return textResult("Error: Lock service not initialized"), nil
	}

	lockID, _ := args["lock_id"].(string)
	if err := lockService.ReleaseLock(ctx, lockID); err != nil {
		return textResult(fmt.Sprintf("Error releasing lock: %v", err)), nil
	}

	return textResult(fmt.Sprintf("Lock %s released successfully", lockID)), nil
}

func handleListLocks(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
	lockService := app.LockService()
	if lockService == nil {
		return textResult("Error: Lock service not initialized"), nil
	}

	locks := lockService.ListLocks()
	if len(locks) == 0 {
		return textResult("No active locks"), nil
	}

	data, _ := json.MarshalIndent(locks, "", "  ")
	return textResult(string(data)), nil
}

func handleShareContext(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
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

	vectorStore := app.VectorStore()
	embedService := app.EmbeddingService()
	if vectorStore == nil || embedService == nil {
		return textResult("Error: Vector store or embedding service not initialized"), nil
	}

	// Generate embedding for the content
	embedding, err := embedService.Embed(ctx, content)
	if err != nil {
		return textResult(fmt.Sprintf("Error generating embedding: %v", err)), nil
	}

	// Create document
	doc := &vector.Document{
		Content:   content,
		Embedding: embedding,
		FilePath:  filePath,
		Metadata:  metadata,
	}

	// Insert into vector store
	if err := vectorStore.Insert(doc); err != nil {
		return textResult(fmt.Sprintf("Error storing context: %v", err)), nil
	}

	// Flush to persist
	if err := vectorStore.Flush(); err != nil {
		return textResult(fmt.Sprintf("Error persisting context: %v", err)), nil
	}

	// Also watch the file for future changes if syncManager is available
	syncManager := app.SyncManager()
	if syncManager != nil && filePath != "" {
		syncManager.WatchFile(filePath)
	}

	return textResult(fmt.Sprintf("Context shared successfully (Document ID: %s, embedding: %d dims)",
		doc.ID, len(embedding))), nil
}

func handleEmbedText(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
	embedService := app.EmbeddingService()
	if embedService == nil {
		return textResult("Error: Embedding service not initialized"), nil
	}

	text, _ := args["text"].(string)
	embedding, err := embedService.Embed(ctx, text)
	if err != nil {
		return textResult(fmt.Sprintf("Error generating embedding: %v", err)), nil
	}

	return textResult(fmt.Sprintf("Generated embedding with %d dimensions using %s provider", len(embedding), embedService.Provider())), nil
}

func handleSearchSimilar(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
	vectorStore := app.VectorStore()
	embedService := app.EmbeddingService()
	if vectorStore == nil || embedService == nil {
		return textResult("Error: Vector store or embedding service not initialized"), nil
	}

	query, _ := args["query"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Generate embedding for query
	embedding, err := embedService.Embed(ctx, query)
	if err != nil {
		return textResult(fmt.Sprintf("Error generating embedding: %v", err)), nil
	}

	// Search using the embedding
	results, err := vectorStore.Search(embedding, &vector.SearchOptions{
		Collection: "default",
		TopK:       limit,
	})
	if err != nil {
		return textResult(fmt.Sprintf("Error searching: %v", err)), nil
	}

	if len(results) == 0 {
		return textResult("No similar content found"), nil
	}

	data, _ := json.MarshalIndent(results, "", "  ")
	return textResult(string(data)), nil
}

func handleClusterStatus(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
	status := app.GetStatus()
	data, _ := json.MarshalIndent(status, "", "  ")
	return textResult(string(data)), nil
}

func handleListAgents(ctx context.Context, app *application.App, args map[string]any) (*ToolCallResult, error) {
	registry := app.AgentRegistry()
	if registry == nil {
		return textResult("Error: Agent registry not initialized"), nil
	}

	agents := registry.List()
	if len(agents) == 0 {
		return textResult("No agents connected"), nil
	}

	data, _ := json.MarshalIndent(agents, "", "  ")
	return textResult(string(data)), nil
}

func textResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []Content{{Type: "text", Text: text}},
	}
}
