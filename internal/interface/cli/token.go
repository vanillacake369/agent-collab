package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "토큰 관리",
	Long:  `초대 토큰 및 사용량을 관리합니다.`,
}

var tokenShowCmd = &cobra.Command{
	Use:   "show",
	Short: "현재 초대 토큰 표시",
	RunE:  runTokenShow,
}

var tokenRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "초대 토큰 갱신",
	RunE:  runTokenRefresh,
}

var tokenUsageCmd = &cobra.Command{
	Use:   "usage",
	Short: "토큰 사용량 통계",
	RunE:  runTokenUsage,
}

var (
	usagePeriod string
	usageJSON   bool
)

func init() {
	rootCmd.AddCommand(tokenCmd)

	tokenCmd.AddCommand(tokenShowCmd)
	tokenCmd.AddCommand(tokenRefreshCmd)
	tokenCmd.AddCommand(tokenUsageCmd)

	tokenUsageCmd.Flags().StringVar(&usagePeriod, "period", "day", "기간 (day|week|month)")
	tokenUsageCmd.Flags().BoolVar(&usageJSON, "json", false, "JSON 형식으로 출력")
}

func runTokenShow(cmd *cobra.Command, args []string) error {
	// TODO: 실제 토큰 가져오기
	fmt.Println("현재 초대 토큰:")
	fmt.Println()
	fmt.Println("  eyJ2IjoxLCJwaWQiOiJhYmMxMjMuLi4iLCJwbiI6Im15LXByb2plY3QiLC4uLn0=")
	fmt.Println()
	fmt.Println("생성일: 2024-01-15 10:30:00")
	fmt.Println("만료일: 없음")
	fmt.Println()
	fmt.Println("이 토큰을 팀원에게 공유하세요.")

	return nil
}

func runTokenRefresh(cmd *cobra.Command, args []string) error {
	fmt.Println("🔄 토큰 갱신 중...")
	fmt.Println()

	// TODO: 실제 토큰 갱신
	fmt.Println("✓ 새 토큰이 생성되었습니다.")
	fmt.Println()
	fmt.Println("  eyJ2IjoxLCJwaWQiOiJ4eXo3ODkuLi4iLCJwbiI6Im15LXByb2plY3QiLC4uLn0=")
	fmt.Println()
	fmt.Println("⚠️  이전 토큰은 더 이상 사용할 수 없습니다.")

	return nil
}

// TokenUsage는 토큰 사용량 정보입니다.
type TokenUsage struct {
	Period      string         `json:"period"`
	TotalTokens int64          `json:"total_tokens"`
	Limit       int64          `json:"limit"`
	UsagePercent float64       `json:"usage_percent"`
	Breakdown   []UsageBreakdown `json:"breakdown"`
	EstCost     float64        `json:"estimated_cost_usd"`
}

// UsageBreakdown은 사용량 상세 정보입니다.
type UsageBreakdown struct {
	Category string  `json:"category"`
	Tokens   int64   `json:"tokens"`
	Percent  float64 `json:"percent"`
}

func runTokenUsage(cmd *cobra.Command, args []string) error {
	// TODO: 실제 사용량 가져오기
	usage := &TokenUsage{
		Period:       usagePeriod,
		TotalTokens:  104521,
		Limit:        200000,
		UsagePercent: 52.3,
		EstCost:      0.10,
		Breakdown: []UsageBreakdown{
			{Category: "Embedding Generation", Tokens: 78234, Percent: 75},
			{Category: "Context Synchronization", Tokens: 21123, Percent: 20},
			{Category: "Lock Negotiation", Tokens: 5164, Percent: 5},
		},
	}

	if usageJSON {
		data, err := json.MarshalIndent(usage, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// 일반 출력
	periodLabel := map[string]string{
		"day":   "Today",
		"week":  "This Week",
		"month": "This Month",
	}[usagePeriod]

	fmt.Printf("=== Token Usage (%s) ===\n", periodLabel)
	fmt.Println()

	// 게이지
	gauge := renderTextGauge(usage.UsagePercent, 30)
	fmt.Printf("%s %.1f%% (%s / %s)\n",
		gauge, usage.UsagePercent,
		formatTokenCount(usage.TotalTokens),
		formatTokenCount(usage.Limit))
	fmt.Println()

	// 상세
	fmt.Println("--- Breakdown ---")
	for _, b := range usage.Breakdown {
		gauge := renderTextGauge(b.Percent, 20)
		fmt.Printf("  %-25s %s %s (%.0f%%)\n",
			b.Category, gauge, formatTokenCount(b.Tokens), b.Percent)
	}
	fmt.Println()

	fmt.Printf("Estimated Cost: $%.2f\n", usage.EstCost)

	return nil
}

func renderTextGauge(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}

	result := ""
	for i := 0; i < width; i++ {
		if i < filled {
			result += "█"
		} else {
			result += "░"
		}
	}
	return result
}

func formatTokenCount(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
