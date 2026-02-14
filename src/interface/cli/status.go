package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"agent-collab/src/application"
	"agent-collab/src/interface/daemon"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "클러스터 상태 확인",
	Long: `현재 클러스터의 상태를 확인합니다.

peers, wireguard, token 명령어가 status에 통합되었습니다.

사용 예시:
  agent-collab status              클러스터 상태 확인
  agent-collab status --json       JSON 형식으로 출력
  agent-collab status --watch      실시간 갱신`,
	RunE: runStatus,
}

var (
	statusJSON  bool
	statusWatch bool
)

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "JSON 형식으로 출력")
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "실시간 갱신")
}

// EnhancedStatus contains extended status information
type EnhancedStatus struct {
	*application.Status

	// Extended info from daemon
	Peers      []daemon.PeerInfo `json:"peers,omitempty"`
	Events     []daemon.Event    `json:"events,omitempty"`
	TokenUsage *daemon.TokenUsageResponse `json:"token_usage,omitempty"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Check if daemon is running first
	client := daemon.NewClient()
	if client.IsRunning() {
		return runStatusFromDaemon(client)
	}

	// Fallback: create app and get status directly
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	status := app.GetStatus()

	if statusWatch {
		return runStatusWatch(app)
	}

	enhanced := &EnhancedStatus{Status: status}
	return printEnhancedStatus(enhanced)
}

func runStatusFromDaemon(client *daemon.Client) error {
	daemonStatus, err := client.Status()
	if err != nil {
		return fmt.Errorf("daemon 상태 조회 실패: %w", err)
	}

	// Convert daemon status to app status format
	status := &application.Status{
		Running:     true,
		ProjectName: daemonStatus.ProjectName,
		NodeID:      daemonStatus.NodeID,
		PeerCount:   daemonStatus.PeerCount,
		LockCount:   daemonStatus.LockCount,
	}

	enhanced := &EnhancedStatus{Status: status}

	// Fetch token usage
	if tokenUsage, err := client.TokenUsage(); err == nil {
		enhanced.TokenUsage = tokenUsage
		status.TokensToday = tokenUsage.TokensToday
		status.TokensPerHour = tokenUsage.TokensPerHour
		status.CostToday = tokenUsage.CostToday
	}

	// Fetch peers (always)
	if peersResp, err := client.ListPeers(); err == nil {
		enhanced.Peers = peersResp.Peers
	}

	// Fetch events (always, limit to 5 recent)
	if eventsResp, err := client.ListEvents(5, "", false); err == nil {
		enhanced.Events = eventsResp.Events
	}

	if statusWatch {
		return runStatusWatchDaemon(client)
	}

	return printEnhancedStatus(enhanced)
}

func printEnhancedStatus(enhanced *EnhancedStatus) error {
	if statusJSON {
		data, err := json.MarshalIndent(enhanced, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	status := enhanced.Status

	fmt.Println("╔═══════════════════════════════════════════╗")
	fmt.Println("║           클러스터 상태                    ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()

	if status.ProjectName == "" {
		fmt.Println("❌ 초기화되지 않음")
		fmt.Println()
		fmt.Println("클러스터를 시작하려면:")
		fmt.Println("  agent-collab init -p <project-name>  # 새 클러스터 생성")
		fmt.Println("  agent-collab join <token>            # 기존 클러스터 참여")
		return nil
	}

	// 클러스터 정보
	fmt.Println("📦 클러스터")
	fmt.Printf("   프로젝트: %s\n", status.ProjectName)
	if status.NodeID != "" {
		nodeIDShort := status.NodeID
		if len(nodeIDShort) > 20 {
			nodeIDShort = status.NodeID[:12] + "..." + status.NodeID[len(status.NodeID)-8:]
		}
		fmt.Printf("   노드 ID: %s\n", nodeIDShort)
	}
	if status.Running {
		fmt.Println("   상태: 🟢 실행 중")
	} else {
		fmt.Println("   상태: 🔴 중지됨")
	}
	fmt.Println()

	// 네트워크 정보
	fmt.Println("🌐 네트워크")
	fmt.Printf("   연결된 피어: %d\n", status.PeerCount)
	if len(status.Addresses) > 0 {
		fmt.Println("   주소:")
		for _, addr := range status.Addresses {
			fmt.Printf("     - %s\n", addr)
		}
	}
	fmt.Println()

	// 피어 목록 (--peers 플래그 또는 피어 수가 적을 때)
	if len(enhanced.Peers) > 0 {
		fmt.Println("👥 피어 목록")
		for _, peer := range enhanced.Peers {
			peerIDShort := peer.ID
			if len(peerIDShort) > 20 {
				peerIDShort = peer.ID[:12] + "..." + peer.ID[len(peer.ID)-8:]
			}
			latencyStr := ""
			if peer.Latency > 0 {
				latencyStr = fmt.Sprintf(" [%dms]", peer.Latency)
			}
			statusIcon := "●"
			if !peer.Connected {
				statusIcon = "○"
			}
			fmt.Printf("   %s %s%s\n", statusIcon, peerIDShort, latencyStr)
		}
		fmt.Println()
	}

	// 락 정보
	fmt.Println("🔒 락")
	fmt.Printf("   전체: %d | 내 락: %d\n", status.LockCount, status.MyLockCount)
	fmt.Println()

	// 토큰 사용량
	if enhanced.TokenUsage != nil && enhanced.TokenUsage.DailyLimit > 0 {
		fmt.Println("💰 토큰 사용량")
		fmt.Printf("   오늘: %s tokens (%.1f%% of limit)\n",
			formatTokenCount(enhanced.TokenUsage.TokensToday),
			enhanced.TokenUsage.UsagePercent)
		fmt.Printf("   비용: $%.4f\n", enhanced.TokenUsage.CostToday)
		fmt.Println()
	} else if status.TokensToday > 0 {
		fmt.Println("💰 토큰 사용량")
		fmt.Printf("   오늘: %s tokens\n", formatTokenCount(status.TokensToday))
		if status.CostToday > 0 {
			fmt.Printf("   비용: $%.4f\n", status.CostToday)
		}
		fmt.Println()
	}

	// WireGuard VPN 정보
	if status.WireGuardEnabled {
		fmt.Println("🔐 WireGuard VPN")
		fmt.Printf("   VPN IP: %s\n", status.WireGuardIP)
		fmt.Printf("   Endpoint: %s\n", status.WireGuardEndpoint)
		fmt.Printf("   VPN 피어: %d\n", status.WireGuardPeerCount)
		fmt.Println()
	}

	// 이벤트 (--events 플래그)
	if len(enhanced.Events) > 0 {
		fmt.Println("📋 최근 이벤트")
		for _, event := range enhanced.Events {
			timeStr := event.Timestamp.Format("15:04:05")
			fmt.Printf("   %s %s\n", timeStr, event.Type)
		}
		fmt.Println()
	}

	return nil
}


func runStatusWatch(app *application.App) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 초기 출력
	fmt.Print("\033[2J\033[H") // 화면 클리어
	enhanced := &EnhancedStatus{Status: app.GetStatus()}
	printEnhancedStatus(enhanced)

	for range ticker.C {
		fmt.Print("\033[2J\033[H") // 화면 클리어
		enhanced := &EnhancedStatus{Status: app.GetStatus()}
		printEnhancedStatus(enhanced)
	}

	return nil
}

func runStatusWatchDaemon(client *daemon.Client) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 초기 출력
	fmt.Print("\033[2J\033[H") // 화면 클리어
	if err := printDaemonStatus(client); err != nil {
		return err
	}

	for range ticker.C {
		fmt.Print("\033[2J\033[H") // 화면 클리어
		if err := printDaemonStatus(client); err != nil {
			return err
		}
	}

	return nil
}

func printDaemonStatus(client *daemon.Client) error {
	daemonStatus, err := client.Status()
	if err != nil {
		return fmt.Errorf("daemon 상태 조회 실패: %w", err)
	}

	status := &application.Status{
		Running:     true,
		ProjectName: daemonStatus.ProjectName,
		NodeID:      daemonStatus.NodeID,
		PeerCount:   daemonStatus.PeerCount,
		LockCount:   daemonStatus.LockCount,
	}

	enhanced := &EnhancedStatus{Status: status}

	if tokenUsage, err := client.TokenUsage(); err == nil {
		enhanced.TokenUsage = tokenUsage
		status.TokensToday = tokenUsage.TokensToday
		status.TokensPerHour = tokenUsage.TokensPerHour
		status.CostToday = tokenUsage.CostToday
	}

	// Fetch peers (always)
	if peersResp, err := client.ListPeers(); err == nil {
		enhanced.Peers = peersResp.Peers
	}

	// Fetch events (always, limit to 5 recent)
	if eventsResp, err := client.ListEvents(5, "", false); err == nil {
		enhanced.Events = eventsResp.Events
	}

	return printEnhancedStatus(enhanced)
}
