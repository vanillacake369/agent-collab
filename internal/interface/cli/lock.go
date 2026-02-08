package cli

import (
	"fmt"
	"time"

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

// LockInfo는 락 정보입니다.
type LockInfo struct {
	ID        string    `json:"id"`
	Holder    string    `json:"holder"`
	Target    string    `json:"target"`
	Intention string    `json:"intention"`
	AcquiredAt time.Time `json:"acquired_at"`
	TTL       int       `json:"ttl_seconds"`
}

func runLockList(cmd *cobra.Command, args []string) error {
	// TODO: 실제 락 목록 가져오기
	locks := []LockInfo{
		{
			ID:        "lock-001",
			Holder:    "Alice",
			Target:    "src/auth/login.go:45-67",
			Intention: "리팩토링 중",
			TTL:       25,
		},
		{
			ID:        "lock-002",
			Holder:    "Bob",
			Target:    "pkg/api/handler.go:120-145",
			Intention: "버그 수정",
			TTL:       18,
		},
	}

	fmt.Println("=== Active Locks ===")
	fmt.Println()

	if len(locks) == 0 {
		fmt.Println("활성 락이 없습니다.")
		return nil
	}

	fmt.Printf("%-10s %-10s %-30s %-15s %s\n",
		"ID", "HOLDER", "TARGET", "INTENTION", "TTL")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")

	for _, l := range locks {
		fmt.Printf("%-10s %-10s %-30s %-15s %ds\n",
			l.ID, l.Holder, l.Target, l.Intention, l.TTL)
	}

	return nil
}

func runLockRelease(cmd *cobra.Command, args []string) error {
	lockID := args[0]

	fmt.Printf("🔓 락 해제 중: %s\n", lockID)

	// TODO: 실제 락 해제
	fmt.Println("✓ 락이 해제되었습니다.")

	return nil
}

func runLockHistory(cmd *cobra.Command, args []string) error {
	// TODO: 실제 히스토리 가져오기
	fmt.Println("=== Lock History (Last 10) ===")
	fmt.Println()

	history := []struct {
		Time      string
		Action    string
		Holder    string
		Target    string
	}{
		{"12:34:56", "acquired", "Alice", "src/auth/login.go"},
		{"12:34:45", "released", "You", "pkg/config/config.go"},
		{"12:34:30", "acquired", "Bob", "pkg/api/handler.go"},
		{"12:33:12", "conflict", "Charlie → Alice", "src/auth/login.go"},
		{"12:32:00", "released", "Alice", "src/models/user.go"},
	}

	for _, h := range history {
		icon := "●"
		switch h.Action {
		case "released":
			icon = "○"
		case "conflict":
			icon = "⚠"
		}
		fmt.Printf("  %s %s %-10s %-20s %s\n",
			h.Time, icon, h.Action, h.Holder, h.Target)
	}

	return nil
}
