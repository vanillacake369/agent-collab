package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap은 키 바인딩 맵입니다.
type KeyMap struct {
	// 일반
	Quit    key.Binding
	Help    key.Binding
	Refresh key.Binding

	// 탭 전환 (숫자키만 사용)
	Tab1    key.Binding
	Tab2    key.Binding
	Tab3    key.Binding
	Tab4    key.Binding
	Tab5    key.Binding
	NextTab key.Binding
	PrevTab key.Binding

	// 네비게이션
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Enter  key.Binding
	Escape key.Binding

	// 모드 전환
	CommandMode key.Binding

	// 명령어 단축키 (init, join, leave만)
	ActionInit  key.Binding
	ActionJoin  key.Binding
	ActionLeave key.Binding

	// 컨텍스트 액션
	Delete key.Binding

	// 확인 대화상자
	Yes key.Binding
	No  key.Binding
}

// DefaultKeyMap은 기본 키 바인딩을 반환합니다.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// 일반
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "종료"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "도움말"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "새로고침"),
		),

		// 탭 전환 (숫자키만)
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "Cluster"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "Context"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "Locks"),
		),
		Tab4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "Tokens"),
		),
		Tab5: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "Peers"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "다음 탭"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("Shift+Tab", "이전 탭"),
		),

		// 네비게이션
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "위"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "아래"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "왼쪽"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "오른쪽"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "선택"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "취소"),
		),

		// 모드 전환
		CommandMode: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "명령"),
		),

		// 명령어 단축키
		ActionInit: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "Init"),
		),
		ActionJoin: key.NewBinding(
			key.WithKeys("j"),
			key.WithHelp("j", "Join"),
		),
		ActionLeave: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "Leave"),
		),

		// 컨텍스트 액션
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "삭제"),
		),

		// 확인 대화상자
		Yes: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "Yes"),
		),
		No: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n", "No"),
		),
	}
}

// ShortHelp는 간단한 도움말을 반환합니다.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Refresh, k.CommandMode, k.Help}
}

// FullHelp는 전체 도움말을 반환합니다.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit, k.Refresh, k.CommandMode, k.Help},
		{k.Tab1, k.Tab2, k.Tab3, k.Tab4, k.Tab5},
		{k.ActionInit, k.ActionJoin, k.ActionLeave},
		{k.Up, k.Down, k.Enter, k.Escape},
	}
}

// NormalModeHelp는 Normal 모드 도움말을 반환합니다.
func (k KeyMap) NormalModeHelp() []key.Binding {
	return []key.Binding{
		k.Quit, k.Refresh, k.CommandMode,
		k.ActionInit, k.ActionJoin, k.ActionLeave,
		k.Help,
	}
}

// CommandModeHelp는 Command 모드 도움말을 반환합니다.
func (k KeyMap) CommandModeHelp() []key.Binding {
	return []key.Binding{
		k.Enter, k.Escape,
	}
}

// ConfirmModeHelp는 Confirm 모드 도움말을 반환합니다.
func (k KeyMap) ConfirmModeHelp() []key.Binding {
	return []key.Binding{
		k.Yes, k.No, k.Escape,
	}
}
