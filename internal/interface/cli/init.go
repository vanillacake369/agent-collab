package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"agent-collab/internal/application"
	"agent-collab/internal/infrastructure/network/wireguard/platform"
	"agent-collab/internal/interface/daemon"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "새 클러스터 초기화",
	Long: `프로젝트의 Control Plane을 초기화합니다.

이 명령은 다음을 수행합니다:
  - 프로젝트 전용 libp2p 네트워크 ID 및 암호화 키 생성
  - WireGuard VPN 인터페이스 설정 (선택적)
  - 로컬 Vector DB 초기화
  - 현재 코드베이스의 첫 인덱싱
  - 팀원 초대용 토큰 생성
  - 백그라운드 데몬 시작

WireGuard VPN을 사용하려면 --wireguard 플래그를 사용하세요 (관리자 권한 필요).`,
	RunE: runInit,
}

var (
	projectName     string
	enableWireGuard bool
	wgPort          int
	wgSubnet        string
	initForeground  bool
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&projectName, "project", "p", "", "프로젝트 이름 (필수)")
	initCmd.MarkFlagRequired("project")

	// WireGuard flags (disabled by default)
	initCmd.Flags().BoolVarP(&enableWireGuard, "wireguard", "w", false, "WireGuard VPN 활성화 (관리자 권한 필요)")
	initCmd.Flags().IntVar(&wgPort, "wg-port", 51820, "WireGuard 포트")
	initCmd.Flags().StringVar(&wgSubnet, "wg-subnet", "10.100.0.0/24", "VPN 서브넷")

	// Foreground flag
	initCmd.Flags().BoolVarP(&initForeground, "foreground", "f", false, "포그라운드에서 실행 (데몬 없이)")
}

func runInit(cmd *cobra.Command, args []string) error {
	// WireGuard 지원 여부 확인
	if enableWireGuard {
		supported, suggestion := platform.CheckAndSuggestInstall()
		if !supported {
			fmt.Println(suggestion)
			fmt.Println()
			fmt.Println("WireGuard 없이 계속하려면 --wireguard 플래그 없이 실행하세요:")
			fmt.Printf("  agent-collab init -p %s\n", projectName)
			fmt.Println()
			return fmt.Errorf("WireGuard가 설치되어 있지 않습니다")
		}

		// 루트 권한 확인
		p := platform.GetPlatform()
		if p.RequiresRoot() {
			fmt.Println("⚠ WireGuard는 관리자 권한이 필요합니다.")
			fmt.Println()
			if strings.Contains(p.Name(), "windows") {
				fmt.Println("관리자 권한으로 다시 실행하세요.")
			} else {
				fmt.Printf("  sudo agent-collab init -p %s --wireguard\n", projectName)
			}
			fmt.Println()
			fmt.Println("WireGuard 없이 계속하려면 --wireguard 플래그 없이 실행하세요:")
			fmt.Printf("  agent-collab init -p %s\n", projectName)
			fmt.Println()
			return fmt.Errorf("관리자 권한이 필요합니다")
		}
	}

	fmt.Println("🚀 클러스터 초기화 중...")
	if enableWireGuard {
		fmt.Println("  (WireGuard VPN 활성화)")
	} else {
		fmt.Println("  (WireGuard VPN 비활성화)")
	}
	fmt.Println()

	// 애플리케이션 생성
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// 타임아웃 컨텍스트
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 초기화 옵션 설정
	opts := &application.InitializeOptions{
		ProjectName:     projectName,
		EnableWireGuard: enableWireGuard,
		WireGuardPort:   wgPort,
		Subnet:          wgSubnet,
	}

	// 초기화
	result, err := app.InitializeWithOptions(ctx, opts)
	if err != nil {
		return fmt.Errorf("초기화 실패: %w", err)
	}

	// 앱 정지 (데몬이 다시 로드할 것임)
	app.Stop()

	// 결과 출력
	fmt.Println("✓ 프로젝트 키 생성 완료")
	fmt.Printf("  키 경로: %s\n", result.KeyPath)
	fmt.Println()

	fmt.Println("✓ P2P 노드 설정 완료")
	fmt.Printf("  노드 ID: %s\n", result.NodeID)
	fmt.Println()

	// WireGuard 정보 출력
	if result.WireGuardEnabled {
		fmt.Println("✓ WireGuard VPN 활성화 완료")
		fmt.Printf("  VPN IP: %s\n", result.WireGuardIP)
		fmt.Printf("  Endpoint: %s\n", result.WireGuardEndpoint)
		fmt.Println()
	}

	fmt.Println("📋 초대 토큰 (팀원에게 공유하세요):")
	fmt.Println()
	fmt.Printf("  %s\n", result.InviteToken)
	fmt.Println()
	fmt.Println("팀원은 다음 명령어로 클러스터에 참여할 수 있습니다:")
	fmt.Printf("  agent-collab join %s\n", result.InviteToken)
	fmt.Println()

	// 포그라운드 모드면 데몬 시작하지 않고 직접 실행
	if initForeground {
		return runDaemonRun(cmd, args)
	}

	// 백그라운드 데몬 시작
	return startDaemonAfterInit()
}

// startDaemonAfterInit starts the daemon in background after initialization.
func startDaemonAfterInit() error {
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
			fmt.Println()
			fmt.Println("상태 확인: agent-collab daemon status")
			fmt.Println("데몬 중지: agent-collab daemon stop")
			return nil
		}
	}

	return fmt.Errorf("데몬 시작 시간 초과")
}
