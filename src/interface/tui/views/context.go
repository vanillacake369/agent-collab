package views

import "time"

// ContextView는 컨텍스트 탭 뷰입니다.
type ContextView struct {
	width  int
	height int

	// 데이터
	totalEmbeddings int
	databaseSize    int64
	lastUpdated     time.Time
	syncProgress    map[string]float64
	recentDeltas    []DeltaInfo
}

// DeltaInfo는 Delta 정보입니다.
type DeltaInfo struct {
	Time   time.Time
	From   string
	Files  int
	Size   int64
	Status string
}

// NewContextView는 새 컨텍스트 뷰를 생성합니다.
func NewContextView() *ContextView {
	return &ContextView{
		syncProgress: make(map[string]float64),
	}
}

// SetSize는 뷰 크기를 설정합니다.
func (v *ContextView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetData는 데이터를 설정합니다.
type ContextData struct {
	TotalEmbeddings int
	DatabaseSize    int64
	SyncProgress    map[string]float64
	RecentDeltas    []DeltaInfo
}

func (v *ContextView) SetData(data interface{}) {
	if d, ok := data.(ContextData); ok {
		v.totalEmbeddings = d.TotalEmbeddings
		v.databaseSize = d.DatabaseSize
		v.syncProgress = d.SyncProgress
		v.recentDeltas = d.RecentDeltas
		v.lastUpdated = time.Now()
	}
}
