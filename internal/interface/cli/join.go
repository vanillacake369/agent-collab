package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"agent-collab/internal/application"
	"agent-collab/internal/interface/daemon"

	"github.com/spf13/cobra"
)

var joinCmd = &cobra.Command{
	Use:   "join <invite-token>",
	Short: "기존 클러스터에 참여",
	Long: `초대 토큰을 사용하여 기존 클러스터에 참여합니다.

이 명령은 다음을 수행합니다:
  - 토큰에서 프로젝트 정보 추출
  - Bootstrap peer에 연결
  - NAT 통과 및 P2P 연결 수립
  - 기존 컨텍스트 동기화
  - 백그라운드 데몬 시작`,
	Args: cobra.ExactArgs(1),
	RunE: runJoin,
}

var (
	displayName    string
	joinForeground bool
)

func init() {
	rootCmd.AddCommand(joinCmd)

	joinCmd.Flags().StringVarP(&displayName, "name", "n", "", "표시 이름 (선택)")
	joinCmd.Flags().BoolVarP(&joinForeground, "foreground", "f", false, "포그라운드에서 실행 (데몬 없이)")
}

func runJoin(cmd *cobra.Command, args []string) error {
	token := args[0]

	fmt.Println("🔗 클러스터 참여 중...")
	fmt.Println()

	// 애플리케이션 생성
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// 타임아웃 컨텍스트
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 클러스터 참여
	result, err := app.Join(ctx, token)
	if err != nil {
		return fmt.Errorf("클러스터 참여 실패: %w", err)
	}

	// 앱 정지 (데몬이 다시 로드할 것임)
	app.Stop()

	// 결과 출력
	fmt.Printf("✓ 프로젝트 '%s' 참여 설정 완료\n", result.ProjectName)
	fmt.Printf("✓ 노드 ID: %s\n", result.NodeID)
	fmt.Printf("✓ Bootstrap peer: %s\n", result.BootstrapPeer)

	// WireGuard 정보 출력
	if result.WireGuardEnabled {
		fmt.Println()
		fmt.Println("✓ WireGuard VPN 연결 완료")
		fmt.Printf("  VPN IP: %s\n", result.WireGuardIP)
	}
	fmt.Println()

	// 포그라운드 모드면 데몬 시작하지 않고 직접 실행
	if joinForeground {
		return runDaemonRun(cmd, args)
	}

	// 백그라운드 데몬 시작
	return startDaemonAfterJoin()
}

// startDaemonAfterJoin starts the daemon in background after joining.
func startDaemonAfterJoin() error {
	client := daemon.NewClient()

	// Check if already running
	if client.IsRunning() {
		fmt.Println("✓ 데몬이 이미 실행 중입니다.")
		return nil
	}

	// Start daemon in background
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("실행 파일 경로를 찾을 수 없습니다: %w", err)
	}

	// #nosec G204 - executable is from os.Executable(), not user input
	daemonProcess := exec.Command(executable, "daemon", "run")
	daemonProcess.Stdout = nil
	daemonProcess.Stderr = nil
	daemonProcess.Stdin = nil

	// Detach from parent process (platform-specific)
	setSysProcAttr(daemonProcess)

	if err := daemonProcess.Start(); err != nil {
		return fmt.Errorf("데몬 시작 실패: %w", err)
	}

	fmt.Printf("🚀 백그라운드 데몬 시작 중... (PID: %d)\n", daemonProcess.Process.Pid)

	// Wait for daemon to be ready
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if client.IsRunning() {
			fmt.Println("✓ 데몬이 시작되었습니다.")

			// Wait a bit more for bootstrap connection
			fmt.Print("✓ Bootstrap peer 연결 중")
			var peerCount int
			for j := 0; j < 20; j++ {
				time.Sleep(250 * time.Millisecond)
				fmt.Print(".")
				if status, err := client.Status(); err == nil && status.PeerCount > 0 {
					peerCount = status.PeerCount
					break
				}
			}
			fmt.Println()

			fmt.Println("✓ 클러스터 참여 완료!")
			fmt.Printf("✓ 연결된 peer: %d명\n", peerCount)
			fmt.Println()
			fmt.Println("상태 확인: agent-collab daemon status")
			fmt.Println("데몬 중지: agent-collab daemon stop")
			return nil
		}
	}

	return fmt.Errorf("데몬 시작 시간 초과")
}
