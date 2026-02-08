package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"agent-collab/internal/application"
	"agent-collab/internal/interface/daemon"
	"agent-collab/internal/interface/mcp"

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

var mcpStandalone bool

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)
	mcpCmd.AddCommand(mcpInfoCmd)

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

	// Register daemon-connected tools
	mcp.RegisterDaemonTools(server, client)

	// Create and start event handler
	eventHandler := mcp.NewEventHandler(client)
	eventHandler.Start(ctx)
	defer eventHandler.Stop()

	// Register event tools
	mcp.RegisterEventTools(server, eventHandler)

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
