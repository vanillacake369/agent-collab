package cli

import (
	"fmt"
	"time"

	"agent-collab/src/application"

	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "에이전트 관리",
	Long:  `연결된 AI 에이전트를 관리합니다.`,
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "연결된 에이전트 목록",
	RunE:  runAgentsList,
}

var agentsInfoCmd = &cobra.Command{
	Use:   "info <agent-id>",
	Short: "에이전트 상세 정보",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentsInfo,
}

var agentsProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "지원 프로바이더 목록",
	RunE:  runAgentsProviders,
}

func init() {
	rootCmd.AddCommand(agentsCmd)

	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsInfoCmd)
	agentsCmd.AddCommand(agentsProvidersCmd)
}

func runAgentsList(cmd *cobra.Command, args []string) error {
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	registry := app.AgentRegistry()
	if registry == nil {
		fmt.Println("❌ 에이전트 레지스트리가 초기화되지 않았습니다.")
		return nil
	}

	agents := registry.List()

	fmt.Println("=== Connected Agents ===")
	fmt.Printf("Total: %d agents\n", len(agents))
	fmt.Println()

	if len(agents) == 0 {
		fmt.Println("연결된 에이전트가 없습니다.")
		fmt.Println()
		fmt.Println("에이전트 연결 방법:")
		fmt.Println("  - MCP: agent-collab mcp serve")
		fmt.Println("  - P2P: agent-collab join <token>")
		return nil
	}

	fmt.Printf("%-8s %-20s %-12s %-15s %s\n",
		"STATUS", "NAME", "PROVIDER", "MODEL", "CONNECTED")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")

	for _, agent := range agents {
		statusIcon := "●"
		switch agent.Status {
		case "offline":
			statusIcon = "○"
		case "busy":
			statusIcon = "◐"
		case "error":
			statusIcon = "✗"
		}

		connectedAgo := time.Since(agent.ConnectedAt).Truncate(time.Second)
		fmt.Printf("%-8s %-20s %-12s %-15s %v ago\n",
			statusIcon, agent.Info.Name, agent.Info.Provider, agent.Info.Model, connectedAgo)
	}

	return nil
}

func runAgentsInfo(cmd *cobra.Command, args []string) error {
	agentID := args[0]

	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	registry := app.AgentRegistry()
	if registry == nil {
		return fmt.Errorf("에이전트 레지스트리가 초기화되지 않았습니다")
	}

	agent, found := registry.Get(agentID)
	if !found {
		return fmt.Errorf("에이전트를 찾을 수 없습니다: %s", agentID)
	}

	fmt.Printf("=== Agent Info: %s ===\n", agent.Info.Name)
	fmt.Println()

	fmt.Printf("  %-16s: %s\n", "ID", agent.Info.ID)
	fmt.Printf("  %-16s: %s\n", "Name", agent.Info.Name)
	fmt.Printf("  %-16s: %s\n", "Provider", agent.Info.Provider)
	fmt.Printf("  %-16s: %s\n", "Model", agent.Info.Model)
	fmt.Printf("  %-16s: %s\n", "Version", agent.Info.Version)
	fmt.Printf("  %-16s: %s\n", "Status", agent.Status)
	fmt.Printf("  %-16s: %s\n", "Peer ID", agent.PeerID)
	fmt.Printf("  %-16s: %s\n", "Connected At", agent.ConnectedAt.Format(time.RFC3339))
	fmt.Printf("  %-16s: %s\n", "Last Seen", agent.LastSeenAt.Format(time.RFC3339))
	fmt.Printf("  %-16s: %d\n", "Tokens Used", agent.TokensUsed)
	fmt.Printf("  %-16s: %d\n", "Request Count", agent.RequestCount)
	fmt.Println()

	if len(agent.Info.Capabilities) > 0 {
		fmt.Println("  Capabilities:")
		for _, cap := range agent.Info.Capabilities {
			fmt.Printf("    - %s\n", cap)
		}
	}

	return nil
}

func runAgentsProviders(cmd *cobra.Command, args []string) error {
	fmt.Println("=== Supported AI Providers ===")
	fmt.Println()

	providers := []struct {
		name        string
		envVar      string
		models      string
		description string
	}{
		{
			name:        "OpenAI",
			envVar:      "OPENAI_API_KEY",
			models:      "text-embedding-3-small, text-embedding-3-large",
			description: "OpenAI의 임베딩 모델 사용",
		},
		{
			name:        "Anthropic",
			envVar:      "ANTHROPIC_API_KEY",
			models:      "voyage-2, voyage-large-2, voyage-code-2",
			description: "Anthropic/Voyage AI 임베딩 모델 사용",
		},
		{
			name:        "Google",
			envVar:      "GOOGLE_API_KEY",
			models:      "text-embedding-004, embedding-001",
			description: "Google AI 임베딩 모델 사용",
		},
		{
			name:        "Ollama",
			envVar:      "OLLAMA_HOST (optional)",
			models:      "nomic-embed-text, mxbai-embed-large",
			description: "로컬 Ollama 서버 사용 (localhost:11434)",
		},
		{
			name:        "Mock",
			envVar:      "(none)",
			models:      "mock-embedding",
			description: "테스트용 가짜 임베딩 (API 키 불필요)",
		},
	}

	for _, p := range providers {
		fmt.Printf("● %s\n", p.name)
		fmt.Printf("  환경변수: %s\n", p.envVar)
		fmt.Printf("  모델: %s\n", p.models)
		fmt.Printf("  설명: %s\n", p.description)
		fmt.Println()
	}

	fmt.Println("사용 예시:")
	fmt.Println("  export OPENAI_API_KEY=sk-...")
	fmt.Println("  agent-collab config set embedding.provider openai")
	fmt.Println("  agent-collab config set embedding.model text-embedding-3-small")

	return nil
}
