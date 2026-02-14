package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-collab/src/application"
	"agent-collab/src/infrastructure/network/wireguard/platform"
	"agent-collab/src/interface/daemon"

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
	initForce       bool
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

	// Force flag
	initCmd.Flags().BoolVar(&initForce, "force", false, "기존 클러스터가 있어도 강제로 재초기화")
}

func runInit(cmd *cobra.Command, args []string) error {
	// init은 데몬 없이 직접 초기화를 수행합니다.
	// 초기화 후 config.json이 생성되면 데몬을 시작합니다.

	// 기존 클러스터 존재 여부 확인
	if err := checkExistingClusterWithForce(projectName, initForce); err != nil {
		return err
	}

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

	// 초기화 완료 후 데몬 시작/재시작
	client := daemon.NewClient()
	if client.IsRunning() {
		// Daemon is running but doesn't have the new config - restart it
		fmt.Println("🔄 데몬 재시작 중... (새 설정 로드)")
		if err := client.Shutdown(); err != nil {
			// Try to terminate the process
			if pid, err := client.GetPID(); err == nil {
				signalTerm(pid)
			}
		}
		// Wait for daemon to stop
		for i := 0; i < 30; i++ {
			time.Sleep(100 * time.Millisecond)
			if !client.IsRunning() {
				break
			}
		}
	}

	fmt.Println("🚀 데몬 시작 중...")
	if err := startDaemonBackground(); err != nil {
		fmt.Printf("⚠ 데몬 시작 실패: %v\n", err)
		fmt.Println("  수동으로 시작하려면: agent-collab daemon start")
	}

	fmt.Println()
	fmt.Println("상태 확인: agent-collab daemon status")
	fmt.Println("데몬 중지: agent-collab daemon stop")

	return nil
}

// existingConfig는 기존 config.json의 최소 정보를 담는 구조체입니다.
type existingConfig struct {
	ProjectName string `json:"project_name"`
}

// checkExistingCluster는 기존 클러스터가 존재하는지 확인합니다.
func checkExistingCluster(projectName string) error {
	return checkExistingClusterWithForce(projectName, false)
}

// checkExistingClusterWithForce는 기존 클러스터 존재 여부를 확인하고,
// force가 true면 기존 클러스터가 있어도 에러를 반환하지 않습니다.
func checkExistingClusterWithForce(projectName string, force bool) error {
	dataDir := getInitDataDir()
	configPath := filepath.Join(dataDir, "config.json")

	// config.json이 없으면 신규 초기화 가능
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	// config.json 읽기
	data, err := os.ReadFile(configPath)
	if err != nil {
		// 읽기 실패 = 손상된 파일, 덮어쓰기 허용
		return nil
	}

	// JSON 파싱
	var existing existingConfig
	if err := json.Unmarshal(data, &existing); err != nil {
		// 파싱 실패 = 손상된 JSON, 덮어쓰기 허용
		return nil
	}

	// force 플래그가 있으면 무조건 허용
	if force {
		return nil
	}

	// 동일 프로젝트명
	if existing.ProjectName == projectName {
		return fmt.Errorf("클러스터 '%s'가 이미 존재합니다. 재초기화하려면 --force 플래그를 사용하세요", projectName)
	}

	// 다른 프로젝트명
	return fmt.Errorf("다른 클러스터 '%s'가 존재합니다. 덮어쓰려면 --force 플래그를 사용하세요", existing.ProjectName)
}

// getInitDataDir는 데이터 디렉토리 경로를 반환합니다 (init용).
func getInitDataDir() string {
	if dir := os.Getenv("AGENT_COLLAB_DATA_DIR"); dir != "" {
		return dir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".agent-collab")
}
