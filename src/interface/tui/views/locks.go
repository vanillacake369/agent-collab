package views

// LocksView는 락 탭 뷰입니다.
type LocksView struct {
	width  int
	height int

	// 데이터
	locks         []LockInfo
	pendingLocks  []LockInfo
	selectedIndex int
}

// LockInfo는 락 정보입니다.
type LockInfo struct {
	ID        string
	Holder    string
	Target    string
	Intention string
	TTL       int
}

// NewLocksView는 새 락 뷰를 생성합니다.
func NewLocksView() *LocksView {
	return &LocksView{}
}

// SetSize는 뷰 크기를 설정합니다.
func (v *LocksView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetLocks는 락 목록을 설정합니다.
func (v *LocksView) SetLocks(locks interface{}) {
	if l, ok := locks.([]LockInfo); ok {
		v.locks = l
	}
}
