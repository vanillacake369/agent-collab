package token

// Model pricing (cost per 1M tokens in USD)
// Based on common LLM pricing as of 2024
var modelPricing = map[string]float64{
	// OpenAI models
	"gpt-4":           30.0,
	"gpt-4-turbo":     10.0,
	"gpt-4o":          5.0,
	"gpt-4o-mini":     0.15,
	"gpt-3.5-turbo":   0.5,

	// Anthropic models
	"claude-3-opus":   15.0,
	"claude-3-sonnet": 3.0,
	"claude-3-haiku":  0.25,
	"claude-3.5-sonnet": 3.0,

	// Embedding models
	"text-embedding-3-small": 0.02,
	"text-embedding-3-large": 0.13,
	"text-embedding-ada-002": 0.10,

	// Default fallback
	"default": 1.0,
}

// EstimateCost estimates the cost for a given number of tokens and model.
func EstimateCost(tokens int64, model string) float64 {
	price, ok := modelPricing[model]
	if !ok {
		price = modelPricing["default"]
	}

	// Cost = tokens * (price per 1M tokens) / 1,000,000
	return float64(tokens) * price / 1_000_000
}

// GetModelPrice returns the price per 1M tokens for a model.
func GetModelPrice(model string) float64 {
	if price, ok := modelPricing[model]; ok {
		return price
	}
	return modelPricing["default"]
}

// SetModelPrice sets a custom price for a model.
func SetModelPrice(model string, pricePerMillion float64) {
	modelPricing[model] = pricePerMillion
}

// EstimateDailyCost estimates daily cost based on current rate.
func EstimateDailyCost(tokensPerHour float64, model string) float64 {
	dailyTokens := tokensPerHour * 24
	return EstimateCost(int64(dailyTokens), model)
}

// EstimateMonthlyCost estimates monthly cost based on current rate.
func EstimateMonthlyCost(tokensPerHour float64, model string) float64 {
	monthlyTokens := tokensPerHour * 24 * 30
	return EstimateCost(int64(monthlyTokens), model)
}

// FormatCost formats a cost value as a string with currency.
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return "$0.00"
	}
	if cost < 1.0 {
		return "$" + formatFloat(cost, 3)
	}
	return "$" + formatFloat(cost, 2)
}

// formatFloat formats a float with specified decimal places.
func formatFloat(f float64, decimals int) string {
	format := "%." + string(rune('0'+decimals)) + "f"
	return sprintf(format, f)
}

// sprintf is a simple sprintf implementation to avoid fmt import.
func sprintf(format string, args ...any) string {
	// Simple implementation for float formatting
	if len(args) == 1 {
		if f, ok := args[0].(float64); ok {
			// Extract decimal places from format
			decimals := 2
			for i := 0; i < len(format); i++ {
				if format[i] == '.' && i+1 < len(format) {
					decimals = int(format[i+1] - '0')
					break
				}
			}
			return formatFloatSimple(f, decimals)
		}
	}
	return ""
}

// formatFloatSimple formats a float with basic implementation.
func formatFloatSimple(f float64, decimals int) string {
	// Multiply to shift decimal places
	multiplier := 1.0
	for i := 0; i < decimals; i++ {
		multiplier *= 10
	}

	// Round
	rounded := int64(f*multiplier + 0.5)

	// Split into integer and decimal parts
	intPart := rounded / int64(multiplier)
	decPart := rounded % int64(multiplier)

	// Format
	result := formatInt(intPart) + "."

	// Pad decimal part with leading zeros if needed
	decStr := formatInt(decPart)
	for len(decStr) < decimals {
		decStr = "0" + decStr
	}

	return result + decStr
}

// formatInt formats an integer as a string.
func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}

	if n < 0 {
		return "-" + formatInt(-n)
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	return string(digits)
}
