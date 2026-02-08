package views

// TokensView는 토큰 탭 뷰입니다.
type TokensView struct {
	width  int
	height int

	// 데이터
	todayUsed   int64
	dailyLimit  int64
	breakdown   []TokenBreakdown
	hourlyData  []float64
	costToday   float64
	costWeek    float64
	costMonth   float64
	tokensWeek  int64
	tokensMonth int64
}

// TokenBreakdown은 토큰 사용량 상세입니다.
type TokenBreakdown struct {
	Category string
	Tokens   int64
	Percent  float64
	Cost     float64
}

// NewTokensView는 새 토큰 뷰를 생성합니다.
func NewTokensView() *TokensView {
	return &TokensView{
		dailyLimit: 200000,
	}
}

// SetSize는 뷰 크기를 설정합니다.
func (v *TokensView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetData는 데이터를 설정합니다.
func (v *TokensView) SetData(data interface{}) {
	// TODO: 타입 어설션
}

// UsagePercent는 사용률을 반환합니다.
func (v *TokensView) UsagePercent() float64 {
	if v.dailyLimit == 0 {
		return 0
	}
	return float64(v.todayUsed) / float64(v.dailyLimit) * 100
}
