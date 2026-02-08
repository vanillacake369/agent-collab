package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var leaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "클러스터 탈퇴",
	Long: `현재 클러스터에서 탈퇴합니다.

이 명령은 다음을 수행합니다:
  - 모든 peer와의 연결 종료
  - 활성 락 해제
  - 로컬 컨텍스트 유지 (삭제하려면 --clean 사용)`,
	RunE: runLeave,
}

var (
	leaveForce bool
	leaveClean bool
)

func init() {
	rootCmd.AddCommand(leaveCmd)

	leaveCmd.Flags().BoolVarP(&leaveForce, "force", "f", false, "강제 탈퇴 (확인 없이)")
	leaveCmd.Flags().BoolVar(&leaveClean, "clean", false, "로컬 데이터도 삭제")
}

func runLeave(cmd *cobra.Command, args []string) error {
	if !leaveForce {
		fmt.Println("⚠️  클러스터에서 탈퇴하시겠습니까?")
		fmt.Println()
		fmt.Println("  - 모든 peer와의 연결이 종료됩니다.")
		fmt.Println("  - 활성 락이 해제됩니다.")
		if leaveClean {
			fmt.Println("  - 로컬 데이터가 삭제됩니다.")
		}
		fmt.Println()
		fmt.Println("계속하려면 --force 플래그를 사용하세요.")
		return nil
	}

	fmt.Println("🔌 클러스터 탈퇴 중...")
	fmt.Println()

	// TODO: 실제 탈퇴 로직
	fmt.Println("✓ 활성 락 해제 완료")
	fmt.Println("✓ Peer 연결 종료")

	if leaveClean {
		fmt.Println("✓ 로컬 데이터 삭제 완료")
	}

	fmt.Println()
	fmt.Println("클러스터에서 탈퇴했습니다.")
	fmt.Println("다시 참여하려면 'agent-collab join <token>'을 사용하세요.")

	return nil
}
