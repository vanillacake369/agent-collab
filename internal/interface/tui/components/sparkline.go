package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var sparkChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

var sparklineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))

// RenderSparkline은 스파크라인을 렌더링합니다.
func RenderSparkline(data []float64, width int) string {
	if len(data) == 0 {
		return strings.Repeat(sparkChars[0], width)
	}

	// 데이터를 width에 맞게 샘플링
	sampled := sampleData(data, width)

	// 최대값 찾기
	max := 0.0
	for _, v := range sampled {
		if v > max {
			max = v
		}
	}

	if max == 0 {
		return strings.Repeat(sparkChars[0], width)
	}

	// 스파크라인 생성
	var result strings.Builder
	for _, v := range sampled {
		idx := int((v / max) * float64(len(sparkChars)-1))
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		if idx < 0 {
			idx = 0
		}
		result.WriteString(sparklineStyle.Render(sparkChars[idx]))
	}

	return result.String()
}

// sampleData는 데이터를 주어진 너비에 맞게 샘플링합니다.
func sampleData(data []float64, width int) []float64 {
	if len(data) <= width {
		// 데이터가 width보다 작으면 그대로 반환
		result := make([]float64, width)
		copy(result, data)
		return result
	}

	result := make([]float64, width)
	step := float64(len(data)) / float64(width)

	for i := 0; i < width; i++ {
		idx := int(float64(i) * step)
		if idx >= len(data) {
			idx = len(data) - 1
		}
		result[i] = data[idx]
	}

	return result
}

// RenderSparklineWithScale은 스케일이 있는 스파크라인을 렌더링합니다.
func RenderSparklineWithScale(data []float64, width int, min, max float64) string {
	if len(data) == 0 || max <= min {
		return strings.Repeat(sparkChars[0], width)
	}

	sampled := sampleData(data, width)

	var result strings.Builder
	for _, v := range sampled {
		normalized := (v - min) / (max - min)
		if normalized > 1 {
			normalized = 1
		}
		if normalized < 0 {
			normalized = 0
		}

		idx := int(normalized * float64(len(sparkChars)-1))
		result.WriteString(sparklineStyle.Render(sparkChars[idx]))
	}

	return result.String()
}
