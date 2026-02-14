package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"agent-collab/src/application"
	"agent-collab/src/domain/token"

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
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// Load configuration to initialize the node
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if err := app.LoadFromConfig(ctx); err != nil {
		fmt.Println("❌ 클러스터가 초기화되지 않았습니다.")
		fmt.Println("먼저 'agent-collab init' 또는 'agent-collab join'을 실행하세요.")
		return nil
	}
	defer app.Stop()

	tokenStr, err := app.CreateInviteToken()
	if err != nil {
		return fmt.Errorf("토큰 생성 실패: %w", err)
	}

	fmt.Println(tokenStr)

	return nil
}

func runTokenRefresh(cmd *cobra.Command, args []string) error {
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// Load configuration to initialize the node
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if err := app.LoadFromConfig(ctx); err != nil {
		fmt.Println("❌ 클러스터가 초기화되지 않았습니다.")
		return nil
	}
	defer app.Stop()

	fmt.Println("🔄 토큰 갱신 중...")
	fmt.Println()

	tokenStr, err := app.CreateInviteToken()
	if err != nil {
		return fmt.Errorf("토큰 생성 실패: %w", err)
	}

	fmt.Println("✓ 새 토큰이 생성되었습니다.")
	fmt.Println()
	fmt.Printf("  %s\n", tokenStr)
	fmt.Println()
	fmt.Println("Note: 이전 토큰도 유효합니다. (토큰은 노드 주소 기반)")

	return nil
}

// TokenUsage는 토큰 사용량 정보입니다.
type TokenUsage struct {
	Period       string           `json:"period"`
	TotalTokens  int64            `json:"total_tokens"`
	Limit        int64            `json:"limit"`
	UsagePercent float64          `json:"usage_percent"`
	Breakdown    []UsageBreakdown `json:"breakdown"`
	EstCost      float64          `json:"estimated_cost_usd"`
}

// UsageBreakdown은 사용량 상세 정보입니다.
type UsageBreakdown struct {
	Category string  `json:"category"`
	Tokens   int64   `json:"tokens"`
	Percent  float64 `json:"percent"`
}

func runTokenUsage(cmd *cobra.Command, args []string) error {
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	tracker := app.TokenTracker()
	if tracker == nil {
		fmt.Println("❌ 토큰 추적기가 초기화되지 않았습니다.")
		return nil
	}

	metrics := tracker.GetMetrics()

	// Calculate total tokens based on period
	var totalTokens int64
	var cost float64
	switch usagePeriod {
	case "week":
		totalTokens = metrics.TokensWeek
		cost = metrics.CostWeek
	case "month":
		totalTokens = metrics.TokensMonth
		cost = metrics.CostMonth
	default:
		totalTokens = metrics.TokensToday
		cost = metrics.CostToday
	}

	limit := metrics.DailyLimit
	if limit == 0 {
		limit = 200000 // default limit
	}

	usagePercent := float64(totalTokens) / float64(limit) * 100
	if usagePercent > 100 {
		usagePercent = 100
	}

	// Build breakdown from metrics
	var breakdown []UsageBreakdown
	categoryNames := map[token.UsageCategory]string{
		token.CategoryEmbedding:   "Embedding Generation",
		token.CategorySync:        "Context Synchronization",
		token.CategoryNegotiation: "Lock Negotiation",
		token.CategoryQuery:       "Query Processing",
		token.CategoryOther:       "Other",
	}

	for cat, tokens := range metrics.ByCategory {
		if tokens > 0 {
			pct := float64(tokens) / float64(totalTokens) * 100
			if totalTokens == 0 {
				pct = 0
			}
			breakdown = append(breakdown, UsageBreakdown{
				Category: categoryNames[cat],
				Tokens:   tokens,
				Percent:  pct,
			})
		}
	}

	usage := &TokenUsage{
		Period:       usagePeriod,
		TotalTokens:  totalTokens,
		Limit:        limit,
		UsagePercent: usagePercent,
		EstCost:      cost,
		Breakdown:    breakdown,
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
