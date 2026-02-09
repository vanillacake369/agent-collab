package cli

import (
	"context"
	"fmt"
	"time"

	"agent-collab/internal/application"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "새 클러스터 초기화",
	Long: `프로젝트의 Control Plane을 초기화합니다.

이 명령은 다음을 수행합니다:
  - 프로젝트 전용 libp2p 네트워크 ID 및 암호화 키 생성
  - WireGuard VPN 인터페이스 설정 (기본 활성화)
  - 로컬 Vector DB 초기화
  - 현재 코드베이스의 첫 인덱싱
  - 팀원 초대용 토큰 생성

WireGuard VPN은 기본적으로 활성화됩니다. 비활성화하려면 --no-wireguard 플래그를 사용하세요.`,
	RunE: runInit,
}

var (
	projectName      string
	disableWireGuard bool
	wgPort           int
	wgSubnet         string
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&projectName, "project", "p", "", "프로젝트 이름 (필수)")
	initCmd.MarkFlagRequired("project")

	// WireGuard flags (enabled by default)
	initCmd.Flags().BoolVar(&disableWireGuard, "no-wireguard", false, "WireGuard VPN 비활성화")
	initCmd.Flags().IntVar(&wgPort, "wg-port", 51820, "WireGuard 포트")
	initCmd.Flags().StringVar(&wgSubnet, "wg-subnet", "10.100.0.0/24", "VPN 서브넷")
}

func runInit(cmd *cobra.Command, args []string) error {
	enableWireGuard := !disableWireGuard

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

	// 결과 출력
	fmt.Println("✓ 프로젝트 키 생성 완료")
	fmt.Printf("  키 경로: %s\n", result.KeyPath)
	fmt.Println()

	fmt.Println("✓ P2P 노드 시작 완료")
	fmt.Printf("  노드 ID: %s\n", result.NodeID)
	fmt.Println("  주소:")
	for _, addr := range result.Addresses {
		fmt.Printf("    - %s\n", addr)
	}
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

	return nil
}
