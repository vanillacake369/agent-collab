package mode

// Mode는 TUI 모드를 나타냅니다.
type Mode int

const (
	// Normal은 기본 탭 네비게이션 모드입니다.
	Normal Mode = iota
	// Command는 명령 팔레트 모드입니다 (: 키로 진입).
	Command
	// Input은 텍스트 입력 모드입니다.
	Input
	// Confirm은 확인 대화상자 모드입니다.
	Confirm
)

// String은 모드 이름을 반환합니다.
func (m Mode) String() string {
	switch m {
	case Normal:
		return "NORMAL"
	case Command:
		return "COMMAND"
	case Input:
		return "INPUT"
	case Confirm:
		return "CONFIRM"
	default:
		return "UNKNOWN"
	}
}
