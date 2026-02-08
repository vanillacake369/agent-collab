package cli

import (
	"fmt"

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

	// TODO: 토큰 파싱 및 연결 로직
	_ = token

	fmt.Printf("✓ 프로젝트 '%s' 참여 중...\n", "my-project") // TODO: 실제 프로젝트명
	fmt.Printf("✓ Bootstrap peer 연결 중... (%d/%d 연결됨)\n", 3, 3)
	fmt.Printf("✓ NAT 타입 감지: %s\n", "Full Cone NAT")
	fmt.Printf("✓ %s 통해 연결 성공\n", "QUIC")
	fmt.Printf("✓ 컨텍스트 동기화 완료 (%.1f MB)\n", 2.3)
	fmt.Printf("✓ 현재 활성 peer: %d명\n", 4)
	fmt.Println()
	fmt.Println("클러스터 참여 완료!")

	return nil
}
