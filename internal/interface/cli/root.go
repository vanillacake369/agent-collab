package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	version = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "agent-collab",
	Short: "분산 에이전트 협업 시스템",
	Long: `agent-collab은 서로 다른 네트워크의 개발자 에이전트들을
P2P로 연결하여 컨텍스트를 공유하고 충돌을 사전 예방합니다.

시작하기:
  agent-collab init -p my-project    새 클러스터 생성
  agent-collab join <token>          기존 클러스터 참여
  agent-collab dashboard             TUI 대시보드 실행
  agent-collab status                클러스터 상태 확인`,
}

// Execute는 루트 명령을 실행합니다.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion은 버전을 설정합니다.
func SetVersion(v string) {
	version = v
}

func init() {
	cobra.OnInitialize(initConfig)

	// 전역 플래그
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "설정 파일 경로")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "상세 출력")

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
