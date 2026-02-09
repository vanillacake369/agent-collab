package cli

import (
	"fmt"
	"os"

	"agent-collab/internal/infrastructure/embedding"

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

var configResetForce bool

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)

	configResetCmd.Flags().BoolVarP(&configResetForce, "force", "f", false, "확인 없이 리셋")
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Println("=== Current Configuration ===")
	fmt.Println()

	// Network settings
	fmt.Println("--- Network ---")
	fmt.Printf("  %-28s: %v\n", "network.listen_port", getOrDefault("network.listen_port", 4001))
	fmt.Printf("  %-28s: %v\n", "network.bootstrap", viper.GetStringSlice("network.bootstrap"))
	fmt.Println()

	// Lock settings
	fmt.Println("--- Lock ---")
	fmt.Printf("  %-28s: %v\n", "lock.default_ttl", getOrDefault("lock.default_ttl", "30s"))
	fmt.Printf("  %-28s: %v\n", "lock.heartbeat_interval", getOrDefault("lock.heartbeat_interval", "10s"))
	fmt.Println()

	// Context sync settings
	fmt.Println("--- Context Sync ---")
	fmt.Printf("  %-28s: %v\n", "context.sync_interval", getOrDefault("context.sync_interval", "5s"))
	fmt.Println()

	// Embedding/AI settings
	fmt.Println("--- AI/Embedding ---")
	detectedProvider := embedding.DetectAvailableProvider()
	configuredProvider := viper.GetString("embedding.provider")
	if configuredProvider == "" {
		configuredProvider = string(detectedProvider) + " (auto-detected)"
	}
	fmt.Printf("  %-28s: %s\n", "embedding.provider", configuredProvider)
	fmt.Printf("  %-28s: %s\n", "embedding.model", getOrDefault("embedding.model", "(provider default)"))

	// Show which API keys are set
	fmt.Println()
	fmt.Println("  API Keys Status:")
	apiKeys := []struct {
		name     string
		envVar   string
		provider embedding.Provider
	}{
		{"OpenAI", "OPENAI_API_KEY", embedding.ProviderOpenAI},
		{"Anthropic", "ANTHROPIC_API_KEY", embedding.ProviderAnthropic},
		{"Google", "GOOGLE_API_KEY", embedding.ProviderGoogle},
	}
	for _, ak := range apiKeys {
		status := "✗ not set"
		if os.Getenv(ak.envVar) != "" {
			status = "✓ set"
			if ak.provider == detectedProvider {
				status += " (active)"
			}
		}
		fmt.Printf("    %-20s: %s\n", ak.name, status)
	}
	fmt.Println()

	// Token settings
	fmt.Println("--- Token Usage ---")
	fmt.Printf("  %-28s: %v\n", "token.daily_limit", getOrDefault("token.daily_limit", 200000))
	fmt.Println()

	// UI settings
	fmt.Println("--- UI ---")
	fmt.Printf("  %-28s: %v\n", "ui.theme", getOrDefault("ui.theme", "dark"))
	fmt.Println()

	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		configFile = "(none)"
	}
	fmt.Printf("설정 파일: %s\n", configFile)

	return nil
}

func getOrDefault(key string, defaultVal any) any {
	val := viper.Get(key)
	if val == nil || val == "" || val == 0 {
		return defaultVal
	}
	return val
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Validate known keys
	validKeys := map[string]bool{
		"network.listen_port":     true,
		"lock.default_ttl":        true,
		"lock.heartbeat_interval": true,
		"context.sync_interval":   true,
		"token.daily_limit":       true,
		"ui.theme":                true,
		// Embedding/AI settings
		"embedding.provider": true,
		"embedding.model":    true,
		"embedding.base_url": true,
	}

	if !validKeys[key] {
		fmt.Printf("⚠️  알 수 없는 설정 키: %s\n", key)
		fmt.Println("사용 가능한 키:")
		fmt.Println("  network.listen_port, lock.default_ttl, lock.heartbeat_interval,")
		fmt.Println("  context.sync_interval, token.daily_limit, ui.theme,")
		fmt.Println("  embedding.provider, embedding.model, embedding.base_url")
		return nil
	}

	// Validate embedding provider
	if key == "embedding.provider" {
		validProviders := map[string]bool{
			"openai": true, "anthropic": true, "google": true, "ollama": true, "mock": true,
		}
		if !validProviders[value] {
			fmt.Printf("⚠️  잘못된 provider: %s\n", value)
			fmt.Println("사용 가능한 provider: openai, anthropic, google, ollama, mock")
			return nil
		}
	}

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
	if !configResetForce {
		fmt.Println("⚠️  모든 설정을 기본값으로 복원합니다.")
		fmt.Println()
		fmt.Println("계속하려면 --force 플래그를 사용하세요.")
		return nil
	}

	// 기본값으로 리셋
	viper.Reset()

	fmt.Println("✓ 설정이 기본값으로 복원되었습니다.")
	return nil
}
