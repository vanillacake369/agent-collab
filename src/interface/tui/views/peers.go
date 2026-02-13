package views

// PeersView는 피어 탭 뷰입니다.
type PeersView struct {
	width  int
	height int

	// 데이터
	peers         []PeerInfo
	selectedIndex int
}

// PeerInfo는 피어 정보입니다.
type PeerInfo struct {
	ID        string
	Name      string
	Status    string
	Latency   int
	Transport string
	SyncPct   float64
	Address   string
	Messages  struct {
		Sent     int64
		Received int64
	}
}

// NewPeersView는 새 피어 뷰를 생성합니다.
func NewPeersView() *PeersView {
	return &PeersView{}
}

// SetSize는 뷰 크기를 설정합니다.
func (v *PeersView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetPeers는 피어 목록을 설정합니다.
func (v *PeersView) SetPeers(peers interface{}) {
	if p, ok := peers.([]PeerInfo); ok {
		v.peers = p
	}
}

// SelectedPeer는 선택된 피어를 반환합니다.
func (v *PeersView) SelectedPeer() *PeerInfo {
	if len(v.peers) == 0 || v.selectedIndex >= len(v.peers) {
		return nil
	}
	return &v.peers[v.selectedIndex]
}

// OnlineCount는 온라인 피어 수를 반환합니다.
func (v *PeersView) OnlineCount() int {
	count := 0
	for _, p := range v.peers {
		if p.Status == "online" {
			count++
		}
	}
	return count
}

// SyncingCount는 동기화 중인 피어 수를 반환합니다.
func (v *PeersView) SyncingCount() int {
	count := 0
	for _, p := range v.peers {
		if p.Status == "syncing" {
			count++
		}
	}
	return count
}
