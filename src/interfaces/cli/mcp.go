package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"agent-collab/src/application"
	"agent-collab/src/interfaces/daemon"
	"agent-collab/src/interfaces/mcp"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP 서버 관리",
	Long:  `Model Context Protocol (MCP) 서버를 관리합니다.`,
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "MCP 서버 시작 (stdio)",
	Long: `stdio를 통해 MCP 서버를 시작합니다.

이 명령은 Claude Desktop, Claude Code 등의 MCP 클라이언트가
agent-collab의 기능에 접근할 수 있게 합니다.

데몬이 실행 중이면 데몬에 연결하여 클러스터를 공유합니다.
데몬이 없으면 --standalone 플래그를 사용하여 독립 모드로 실행할 수 있습니다.

Claude Desktop 설정 예시 (claude_desktop_config.json):
{
  "mcpServers": {
    "agent-collab": {
      "command": "agent-collab",
      "args": ["mcp", "serve"]
    }
  }
}`,
	RunE: runMCPServe,
}

var mcpInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "MCP 서버 정보",
	RunE:  runMCPInfo,
}

var mcpCallCmd = &cobra.Command{
	Use:   "call <tool-name> [json-args]",
	Short: "MCP 도구 직접 호출",
	Long: `데몬에 연결하여 MCP 도구를 직접 호출합니다.

사용 예시:
  agent-collab mcp call list_locks '{}'
  agent-collab mcp call share_context '{"file_path":"main.go","content":"Added error handling"}'
  agent-collab mcp call search_similar '{"query":"authentication","limit":5}'
  agent-collab mcp call get_events '{"limit":10}'
  agent-collab mcp call get_warnings '{}'
  agent-collab mcp call cluster_status '{}'

사용 가능한 도구:
  acquire_lock   - 코드 영역에 락 획득
  release_lock   - 락 해제
  list_locks     - 활성 락 목록
  share_context  - 컨텍스트 공유
  embed_text     - 텍스트 임베딩 생성
  search_similar - 유사 콘텐츠 검색
  cluster_status - 클러스터 상태
  list_agents    - 연결된 에이전트 목록
  get_events     - 최근 이벤트 조회
  get_warnings   - 경고 및 충돌 확인
  check_cohesion - 작업 정합성 확인`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runMCPCall,
}

var mcpStandalone bool

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpInfoCmd)
	mcpCmd.AddCommand(mcpCallCmd)

	mcpServeCmd.Flags().BoolVar(&mcpStandalone, "standalone", false, "데몬 없이 독립 모드로 실행")
}

func runMCPServe(cmd *cobra.Command, args []string) error {
	// Handle shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	// Check if daemon is running
	daemonClient := daemon.NewClient()

	if !mcpStandalone && daemonClient.IsRunning() {
		// Connect to daemon
		fmt.Fprintf(os.Stderr, "Connecting to daemon...\n")
		return runMCPWithDaemon(ctx, daemonClient)
	}

	if !mcpStandalone {
		fmt.Fprintf(os.Stderr, "Warning: Daemon not running. Starting in standalone mode.\n")
		fmt.Fprintf(os.Stderr, "Run 'agent-collab daemon start' for shared cluster access.\n")
	}

	// Standalone mode
	return runMCPStandalone(ctx)
}

func runMCPWithDaemon(ctx context.Context, client *daemon.Client) error {
	// Create MCP server without a registry (daemon manages agents)
	server := mcp.NewServer("agent-collab", "1.0.0", nil)

	// Register daemon-connected tools (includes event tools that query daemon's event history)
	mcp.RegisterDaemonTools(server, client)

	// Note: We don't use RegisterEventTools here because MCP runs in stdio mode
	// where each request is a new process, so EventHandler can't accumulate events.
	// Instead, daemon_tools.go's get_events queries the daemon's persisted event history.

	// Serve on stdio
	return server.ServeStdio(ctx)
}

func runMCPStandalone(ctx context.Context) error {
	// Create application
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// Create MCP server
	server := mcp.NewServer("agent-collab", "1.0.0", app.AgentRegistry())

	// Register tools
	mcp.RegisterDefaultTools(server, app)

	// Serve on stdio
	return server.ServeStdio(ctx)
}

func runMCPCall(cmd *cobra.Command, args []string) error {
	toolName := args[0]
	jsonArgs := "{}"
	if len(args) > 1 {
		jsonArgs = args[1]
	}

	// Parse JSON arguments
	var toolArgs map[string]any
	if err := json.Unmarshal([]byte(jsonArgs), &toolArgs); err != nil {
		return fmt.Errorf("invalid JSON arguments: %w", err)
	}

	// Check if daemon is running
	client := daemon.NewClient()
	if !client.IsRunning() {
		return fmt.Errorf("데몬이 실행 중이 아닙니다. 'agent-collab daemon start'를 실행하세요")
	}

	// Call the appropriate daemon API based on tool name
	var result any
	var err error

	switch toolName {
	case "acquire_lock":
		filePath, _ := toolArgs["file_path"].(string)
		startLine, _ := toolArgs["start_line"].(float64)
		endLine, _ := toolArgs["end_line"].(float64)
		intention, _ := toolArgs["intention"].(string)
		result, err = client.AcquireLock(filePath, int(startLine), int(endLine), intention)

	case "release_lock":
		lockID, _ := toolArgs["lock_id"].(string)
		err = client.ReleaseLock(lockID)
		if err == nil {
			result = map[string]any{"success": true, "message": "Lock released"}
		}

	case "list_locks":
		result, err = client.ListLocks()

	case "share_context":
		filePath, _ := toolArgs["file_path"].(string)
		content, _ := toolArgs["content"].(string)
		metadata, _ := toolArgs["metadata"].(map[string]any)
		result, err = client.ShareContext(filePath, content, metadata)

	case "embed_text":
		text, _ := toolArgs["text"].(string)
		result, err = client.Embed(text)

	case "search_similar":
		query, _ := toolArgs["query"].(string)
		limit := 10
		if l, ok := toolArgs["limit"].(float64); ok {
			limit = int(l)
		}
		result, err = client.Search(query, limit)

	case "cluster_status":
		result, err = client.Status()

	case "list_agents":
		result, err = client.ListAgents()

	case "get_events":
		limit := 10
		if l, ok := toolArgs["limit"].(float64); ok {
			limit = int(l)
		}
		eventType, _ := toolArgs["type"].(string)
		includeAll, _ := toolArgs["include_all"].(bool)
		result, err = client.ListEvents(limit, eventType, includeAll)

	case "get_warnings":
		// Get recent events that might be warnings (includeAll=true to see all cluster events)
		events, listErr := client.ListEvents(20, "", true)
		if listErr != nil {
			err = listErr
		} else {
			// Filter for warning-type events
			warnings := []daemon.Event{}
			for _, e := range events.Events {
				if e.Type == "lock.conflict" || e.Type == "context.updated" || e.Type == "agent.joined" {
					warnings = append(warnings, e)
				}
			}
			result = map[string]any{
				"warnings": warnings,
				"count":    len(warnings),
			}
		}

	case "check_cohesion":
		checkType, _ := toolArgs["type"].(string)
		intention, _ := toolArgs["intention"].(string)
		resultStr, _ := toolArgs["result"].(string)
		filesChanged := []string{}
		if fc, ok := toolArgs["files_changed"].([]any); ok {
			for _, f := range fc {
				if s, ok := f.(string); ok {
					filesChanged = append(filesChanged, s)
				}
			}
		}
		result, err = client.CheckCohesion(checkType, intention, resultStr, filesChanged)

	default:
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	if err != nil {
		return fmt.Errorf("tool call failed: %w", err)
	}

	// Output result as JSON
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func runMCPInfo(cmd *cobra.Command, args []string) error {
	fmt.Println("=== MCP Server Info ===")
	fmt.Println()
	fmt.Println("Protocol Version:", mcp.ProtocolVersion)
	fmt.Println("Server Name:     agent-collab")
	fmt.Println("Server Version:  1.0.0")
	fmt.Println()

	// Check daemon status
	client := daemon.NewClient()
	if client.IsRunning() {
		status, err := client.Status()
		if err == nil {
			fmt.Println("Daemon Status:   Running")
			fmt.Printf("  Project:       %s\n", status.ProjectName)
			fmt.Printf("  Peers:         %d\n", status.PeerCount)
			fmt.Printf("  Agents:        %d\n", status.AgentCount)
			fmt.Printf("  Embedding:     %s\n", status.EmbeddingProvider)
		}
	} else {
		fmt.Println("Daemon Status:   Not running")
		fmt.Println("  Run 'agent-collab daemon start' for shared cluster access")
	}

	fmt.Println()
	fmt.Println("Available Tools:")
	fmt.Println("  - acquire_lock    : Acquire a semantic lock on a code region")
	fmt.Println("  - release_lock    : Release a previously acquired lock")
	fmt.Println("  - list_locks      : List all active locks in the cluster")
	fmt.Println("  - share_context   : Share context with other agents")
	fmt.Println("  - embed_text      : Generate embeddings for text")
	fmt.Println("  - search_similar  : Search for similar content")
	fmt.Println("  - cluster_status  : Get cluster status")
	fmt.Println("  - list_agents     : List connected agents")
	fmt.Println()
	fmt.Println("Usage with Claude Desktop:")
	fmt.Println(`  Add to claude_desktop_config.json:
  {
    "mcpServers": {
      "agent-collab": {
        "command": "agent-collab",
        "args": ["mcp", "serve"]
      }
    }
  }`)
	fmt.Println()
	fmt.Println("Usage with Claude Code:")
	fmt.Println("  claude mcp add agent-collab -- agent-collab mcp serve")

	return nil
}
