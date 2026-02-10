package cli

import (
	"context"
	"fmt"
	"time"

	"agent-collab/src/application"

	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "락 관리",
	Long:  `Semantic Lock을 관리합니다.`,
}

var lockListCmd = &cobra.Command{
	Use:   "list",
	Short: "현재 락 목록",
	RunE:  runLockList,
}

var lockReleaseCmd = &cobra.Command{
	Use:   "release <lock-id>",
	Short: "락 강제 해제",
	Args:  cobra.ExactArgs(1),
	RunE:  runLockRelease,
}

var lockHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "락 히스토리",
	RunE:  runLockHistory,
}

func init() {
	rootCmd.AddCommand(lockCmd)

	lockCmd.AddCommand(lockListCmd)
	lockCmd.AddCommand(lockReleaseCmd)
	lockCmd.AddCommand(lockHistoryCmd)
}

func runLockList(cmd *cobra.Command, args []string) error {
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	lockService := app.LockService()
	if lockService == nil {
		fmt.Println("❌ 클러스터가 초기화되지 않았습니다.")
		fmt.Println("먼저 'agent-collab init' 또는 'agent-collab join'을 실행하세요.")
		return nil
	}

	locks := lockService.ListLocks()

	fmt.Println("=== Active Locks ===")
	fmt.Println()

	if len(locks) == 0 {
		fmt.Println("활성 락이 없습니다.")
		return nil
	}

	fmt.Printf("%-12s %-12s %-35s %-15s %s\n",
		"ID", "HOLDER", "TARGET", "INTENTION", "TTL")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")

	for _, l := range locks {
		target := fmt.Sprintf("%s:%d-%d", l.Target.FilePath, l.Target.StartLine, l.Target.EndLine)
		if len(target) > 35 {
			target = target[:32] + "..."
		}
		ttl := l.TTLRemaining().Truncate(time.Second)
		fmt.Printf("%-12s %-12s %-35s %-15s %v\n",
			l.ID, l.HolderName, target, l.Intention, ttl)
	}

	return nil
}

func runLockRelease(cmd *cobra.Command, args []string) error {
	lockID := args[0]

	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	lockService := app.LockService()
	if lockService == nil {
		return fmt.Errorf("클러스터가 초기화되지 않았습니다")
	}

	fmt.Printf("🔓 락 해제 중: %s\n", lockID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := lockService.ReleaseLock(ctx, lockID); err != nil {
		return fmt.Errorf("락 해제 실패: %w", err)
	}

	fmt.Println("✓ 락이 해제되었습니다.")
	return nil
}

func runLockHistory(cmd *cobra.Command, args []string) error {
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	lockService := app.LockService()
	if lockService == nil {
		fmt.Println("❌ 클러스터가 초기화되지 않았습니다.")
		return nil
	}

	history := lockService.GetHistory(10)

	fmt.Println("=== Lock History (Last 10) ===")
	fmt.Println()

	if len(history) == 0 {
		fmt.Println("락 히스토리가 없습니다.")
		return nil
	}

	for _, h := range history {
		icon := "●"
		switch h.Action {
		case "released":
			icon = "○"
		case "conflict":
			icon = "⚠"
		case "expired":
			icon = "⏱"
		}
		fmt.Printf("  %s %s %-10s %-15s %s\n",
			h.Timestamp.Format("15:04:05"), icon, h.Action, h.HolderName, h.Target)
	}

	return nil
}
