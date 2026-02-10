package views

// ClusterView는 클러스터 탭 뷰입니다.
type ClusterView struct {
	width  int
	height int

	// 데이터
	healthScore    float64
	totalPeers     int
	activeLocks    int
	pendingSyncs   int
	avgLatency     int
	messagesPerSec float64
}

// NewClusterView는 새 클러스터 뷰를 생성합니다.
func NewClusterView() *ClusterView {
	return &ClusterView{
		healthScore:    98.5,
		totalPeers:     4,
		activeLocks:    2,
		avgLatency:     42,
		messagesPerSec: 12.4,
	}
}

// SetSize는 뷰 크기를 설정합니다.
func (v *ClusterView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetData는 데이터를 설정합니다.
func (v *ClusterView) SetData(healthScore float64, totalPeers, activeLocks int) {
	v.healthScore = healthScore
	v.totalPeers = totalPeers
	v.activeLocks = activeLocks
}
