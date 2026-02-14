package cli

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"agent-collab/src/interface/tui"
)

var (
	cfgFile  string
	verbose  bool
	startTab string
	version  = "dev"
	commit   = "unknown"
	date     = "unknown"
	builtBy  = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "agent-collab",
	Short: "분산 에이전트 협업 시스템",
	Long: `agent-collab은 서로 다른 네트워크의 개발자 에이전트들을
P2P로 연결하여 컨텍스트를 공유하고 충돌을 사전 예방합니다.

인자 없이 실행하면 TUI 대시보드가 시작됩니다.

서브커맨드:
  agent-collab init -p my-project    새 클러스터 생성
  agent-collab join <token>          기존 클러스터 참여
  agent-collab status                클러스터 상태 확인
  agent-collab leave                 클러스터 탈퇴
  agent-collab mcp serve             MCP 서버 실행

TUI 단축키:
  :           명령 팔레트
  i           Init (새 클러스터)
  J           Join (클러스터 참여)
  L           Leave (클러스터 탈퇴)
  1-5         탭 전환
  ↑↓/jk       항목 선택
  q           종료`,
	RunE: runRoot,
}

// runRoot는 인자 없이 실행 시 TUI를 시작합니다.
func runRoot(cmd *cobra.Command, args []string) error {
	// TUI 앱 생성
	app := tui.NewApp(tui.WithStartTab(startTab))

	// Bubbletea 프로그램 실행
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),       // 대체 화면 사용
		tea.WithMouseCellMotion(), // 마우스 지원
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI 실행 실패: %w", err)
	}

	return nil
}

// Execute는 루트 명령을 실행합니다.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion은 버전을 설정합니다.
func SetVersion(v string) {
	version = v
}

// SetVersionInfo sets full version information.
func SetVersionInfo(v, c, d, b string) {
	version = v
	commit = c
	date = d
	builtBy = b
}

// GetVersion returns the current version.
func GetVersion() string {
	return version
}

// GetVersionInfo returns full version information.
func GetVersionInfo() (ver, com, dat, built string) {
	return version, commit, date, builtBy
}

func init() {
	cobra.OnInitialize(initConfig)

	// 전역 플래그
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "설정 파일 경로")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "상세 출력")

	// TUI 옵션 (루트 명령에도 추가)
	rootCmd.Flags().StringVarP(&startTab, "tab", "t", "cluster",
		"시작 탭 (cluster|context|locks|tokens|peers)")

	// viper 바인딩
	viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "홈 디렉토리를 찾을 수 없습니다:", err)
			os.Exit(1)
		}

		// 설정 파일 경로
		viper.AddConfigPath(home + "/.agent-collab")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// 환경변수 자동 바인딩
	viper.SetEnvPrefix("AGENT_COLLAB")
	viper.AutomaticEnv()

	// 설정 파일 읽기 (없어도 에러 아님)
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "설정 파일 사용:", viper.ConfigFileUsed())
		}
	}
}
