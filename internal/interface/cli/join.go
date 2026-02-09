package cli

import (
	"context"
	"fmt"
	"time"

	"agent-collab/internal/application"

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
  - 기존 컨텍스트 동기화`,
	Args: cobra.ExactArgs(1),
	RunE: runJoin,
}

var (
	displayName string
)

func init() {
	rootCmd.AddCommand(joinCmd)

	joinCmd.Flags().StringVarP(&displayName, "name", "n", "", "표시 이름 (선택)")
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

	// 결과 출력
	fmt.Printf("✓ 프로젝트 '%s' 참여 중...\n", result.ProjectName)
	fmt.Printf("✓ 노드 ID: %s\n", result.NodeID)
	fmt.Printf("✓ Bootstrap peer: %s\n", result.BootstrapPeer)
	fmt.Printf("✓ 연결된 peer: %d명\n", result.ConnectedPeers)

	// WireGuard 정보 출력
	if result.WireGuardEnabled {
		fmt.Println()
		fmt.Println("✓ WireGuard VPN 연결 완료")
		fmt.Printf("  VPN IP: %s\n", result.WireGuardIP)
	}
	fmt.Println()

	// 앱 시작
	if err := app.Start(); err != nil {
		return fmt.Errorf("앱 시작 실패: %w", err)
	}
	defer app.Stop()

	fmt.Println("클러스터 참여 완료!")
	fmt.Println()
	fmt.Println("대시보드를 실행하려면:")
	fmt.Println("  agent-collab dashboard")

	return nil
}
