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
	Long:  `현재 클러스터의 상태를 확인합니다.`,
	RunE:  runStatus,
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

func runStatus(cmd *cobra.Command, args []string) error {
	// Check if daemon is running first
	client := daemon.NewClient()
	if client.IsRunning() {
		daemonStatus, err := client.Status()
		if err == nil {
			// Convert daemon status to app status format
			status := &application.Status{
				Running:           true,
				ProjectName:       daemonStatus.ProjectName,
				NodeID:            daemonStatus.NodeID,
				PeerCount:         daemonStatus.PeerCount,
				LockCount:         daemonStatus.LockCount,
				EmbeddingCount:    0, // Not available from daemon status
			}
			if statusWatch {
				// For watch mode, we need the app
				app, err := application.New(nil)
				if err != nil {
					return fmt.Errorf("앱 생성 실패: %w", err)
				}
				return runStatusWatch(app)
			}
			return printAppStatus(status)
		}
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

	return printAppStatus(status)
}

func printAppStatus(status *application.Status) error {
	if statusJSON {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println("📊 클러스터 상태")
	fmt.Println()

	if status.ProjectName == "" {
		fmt.Println("❌ 초기화되지 않음")
		fmt.Println()
		fmt.Println("클러스터를 시작하려면:")
		fmt.Println("  agent-collab init -p <project-name>  # 새 클러스터 생성")
		fmt.Println("  agent-collab join <token>            # 기존 클러스터 참여")
		return nil
	}

	// 프로젝트 정보
	fmt.Printf("프로젝트: %s\n", status.ProjectName)
	fmt.Printf("상태: ")
	if status.Running {
		fmt.Println("🟢 실행 중")
	} else {
		fmt.Println("🔴 중지됨")
	}
	fmt.Println()

	// 노드 정보
	if status.NodeID != "" {
		fmt.Println("🔗 네트워크")
		fmt.Printf("  노드 ID: %s\n", status.NodeID)
		fmt.Printf("  연결된 피어: %d\n", status.PeerCount)
		if len(status.Addresses) > 0 {
			fmt.Println("  주소:")
			for _, addr := range status.Addresses {
				fmt.Printf("    - %s\n", addr)
			}
		}
		fmt.Println()
	}

	// 락 정보
	fmt.Println("🔒 락")
	fmt.Printf("  전체 락: %d\n", status.LockCount)
	fmt.Printf("  내 락: %d\n", status.MyLockCount)
	fmt.Println()

	// 동기화 정보
	fmt.Println("🔄 동기화")
	fmt.Printf("  델타 수: %d\n", status.DeltaCount)
	fmt.Printf("  감시 파일: %d\n", status.WatchedFiles)
	fmt.Println()

	// WireGuard VPN 정보
	if status.WireGuardEnabled {
		fmt.Println("🔐 WireGuard VPN")
		fmt.Printf("  VPN IP: %s\n", status.WireGuardIP)
		fmt.Printf("  Endpoint: %s\n", status.WireGuardEndpoint)
		fmt.Printf("  VPN 피어: %d\n", status.WireGuardPeerCount)
	}

	return nil
}

func runStatusWatch(app *application.App) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 초기 출력
	fmt.Print("\033[2J\033[H") // 화면 클리어
	printAppStatus(app.GetStatus())

	for range ticker.C {
		fmt.Print("\033[2J\033[H") // 화면 클리어
		printAppStatus(app.GetStatus())
	}

	return nil
}
