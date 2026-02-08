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
  - 로컬 Vector DB 초기화
  - 현재 코드베이스의 첫 인덱싱
  - 팀원 초대용 토큰 생성`,
	RunE: runInit,
}

var (
	projectName string
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&projectName, "project", "p", "", "프로젝트 이름 (필수)")
	initCmd.MarkFlagRequired("project")
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 클러스터 초기화 중...")
	fmt.Println()

	// 애플리케이션 생성
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// 타임아웃 컨텍스트
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 초기화
	result, err := app.Initialize(ctx, projectName)
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

	fmt.Println("📋 초대 토큰 (팀원에게 공유하세요):")
	fmt.Println()
	fmt.Printf("  %s\n", result.InviteToken)
	fmt.Println()
	fmt.Println("팀원은 다음 명령어로 클러스터에 참여할 수 있습니다:")
	fmt.Printf("  agent-collab join %s\n", result.InviteToken)

	return nil
}
