package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"agent-collab/src/application"

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
	leaveReset bool
)

func init() {
	rootCmd.AddCommand(leaveCmd)

	leaveCmd.Flags().BoolVarP(&leaveForce, "force", "f", false, "강제 탈퇴 (확인 없이)")
	leaveCmd.Flags().BoolVar(&leaveClean, "clean", false, "로컬 데이터도 삭제")
	leaveCmd.Flags().BoolVar(&leaveReset, "reset", false, "모든 클러스터 데이터 삭제 (config, keys 포함)")
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

	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// Release locks
	lockService := app.LockService()
	if lockService != nil {
		myLocks := lockService.ListMyLocks()
		for _, l := range myLocks {
			_ = lockService.ReleaseLock(cmd.Context(), l.ID)
		}
		if len(myLocks) > 0 {
			fmt.Printf("✓ 활성 락 해제 완료 (%d개)\n", len(myLocks))
		} else {
			fmt.Println("✓ 활성 락 없음")
		}
	}

	// Stop the application (disconnects from peers)
	if err := app.Stop(); err != nil {
		fmt.Printf("⚠️  앱 종료 중 경고: %v\n", err)
	}
	fmt.Println("✓ Peer 연결 종료")

	// 데이터 정리
	cfg := app.Config()
	if cfg != nil && cfg.DataDir != "" {
		if leaveReset {
			// --reset: 모든 데이터 삭제
			if err := cleanupClusterData(cfg.DataDir, true); err != nil {
				fmt.Printf("⚠️  데이터 정리 중 오류: %v\n", err)
			} else {
				fmt.Println("✓ 모든 클러스터 데이터 삭제 완료")
			}
		} else if leaveClean {
			// --clean: 벡터/메트릭만 삭제
			if err := cleanupClusterData(cfg.DataDir, false); err != nil {
				fmt.Printf("⚠️  데이터 정리 중 오류: %v\n", err)
			} else {
				fmt.Println("✓ 벡터/메트릭 데이터 삭제 완료")
			}
		}
	}

	fmt.Println()
	fmt.Println("클러스터에서 탈퇴했습니다.")
	if leaveReset {
		fmt.Println("새 클러스터를 시작하려면 'agent-collab init -p <project>'를 사용하세요.")
	} else {
		fmt.Println("다시 참여하려면 'agent-collab join <token>'을 사용하세요.")
	}

	return nil
}

// cleanupClusterData는 클러스터 데이터를 정리합니다.
// reset=true면 config, key 포함 모든 데이터를 삭제합니다.
// reset=false면 vectors, metrics만 삭제합니다.
func cleanupClusterData(dataDir string, reset bool) error {
	// 항상 삭제: vectors, metrics
	os.RemoveAll(filepath.Join(dataDir, "vectors"))
	os.RemoveAll(filepath.Join(dataDir, "metrics"))

	if reset {
		// reset=true: config, key, wireguard, daemon 파일도 삭제
		os.Remove(filepath.Join(dataDir, "config.json"))
		os.Remove(filepath.Join(dataDir, "key.json"))
		os.Remove(filepath.Join(dataDir, "wireguard.json"))
		os.Remove(filepath.Join(dataDir, "daemon.pid"))
		os.Remove(filepath.Join(dataDir, "daemon.sock"))
	}

	return nil
}
