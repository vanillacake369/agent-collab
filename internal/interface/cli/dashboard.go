package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"agent-collab/internal/interface/tui"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "TUI 대시보드 실행",
	Long: `인터랙티브 TUI 대시보드를 실행하여 클러스터 상태를 모니터링합니다.

탭:
  [1] Cluster  - 클러스터 전체 상태
  [2] Context  - 컨텍스트 동기화 상태
  [3] Locks    - Semantic Lock 상태
  [4] Tokens   - 토큰 사용량
  [5] Peers    - 연결된 Peer 목록`,
	RunE: runDashboard,
}

var (
	startTab string
)

func init() {
	rootCmd.AddCommand(dashboardCmd)

	dashboardCmd.Flags().StringVarP(&startTab, "tab", "t", "cluster",
		"시작 탭 (cluster|context|locks|tokens|peers)")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	// TUI 앱 생성
	app := tui.NewApp(tui.WithStartTab(startTab))

	// Bubbletea 프로그램 실행
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),       // 대체 화면 사용
		tea.WithMouseCellMotion(), // 마우스 지원
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("대시보드 실행 실패: %w", err)
	}

	return nil
}
