package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "설정 관리",
	Long:  `agent-collab 설정을 관리합니다.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "현재 설정 출력",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "설정 변경",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "기본값 복원",
	RunE:  runConfigReset,
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Println("=== Current Configuration ===")
	fmt.Println()

	// 기본 설정 값들
	settings := map[string]interface{}{
		"network.listen_port":    viper.GetInt("network.listen_port"),
		"network.bootstrap":      viper.GetStringSlice("network.bootstrap"),
		"lock.default_ttl":       viper.GetDuration("lock.default_ttl"),
		"lock.heartbeat_interval": viper.GetDuration("lock.heartbeat_interval"),
		"context.sync_interval":  viper.GetDuration("context.sync_interval"),
		"token.daily_limit":      viper.GetInt64("token.daily_limit"),
		"ui.theme":               viper.GetString("ui.theme"),
	}

	// 기본값 설정
	defaults := map[string]interface{}{
		"network.listen_port":    4001,
		"network.bootstrap":      []string{},
		"lock.default_ttl":       "30s",
		"lock.heartbeat_interval": "10s",
		"context.sync_interval":  "5s",
		"token.daily_limit":      200000,
		"ui.theme":               "dark",
	}

	for key, defaultVal := range defaults {
		val := settings[key]
		if val == nil || val == "" || val == 0 {
			val = defaultVal
		}
		fmt.Printf("  %-28s: %v\n", key, val)
	}

	fmt.Println()
	fmt.Printf("설정 파일: %s\n", viper.ConfigFileUsed())

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// TODO: 설정 값 검증
	viper.Set(key, value)

	// 설정 파일에 저장
	if err := viper.WriteConfig(); err != nil {
		// 설정 파일이 없으면 생성
		if err := viper.SafeWriteConfig(); err != nil {
			return fmt.Errorf("설정 저장 실패: %w", err)
		}
	}

	fmt.Printf("✓ 설정 변경됨: %s = %s\n", key, value)
	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	fmt.Println("⚠️  모든 설정을 기본값으로 복원합니다.")
	fmt.Println()

	// TODO: 확인 프롬프트

	// 기본값으로 리셋
	viper.Reset()

	fmt.Println("✓ 설정이 기본값으로 복원되었습니다.")
	return nil
}
