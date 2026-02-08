package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("62")).
				Padding(0, 1)

	tableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	tableSelectedStyle = tableRowStyle.Copy().
				Background(lipgloss.Color("236"))

	tableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

// TableColumn은 테이블 열을 정의합니다.
type TableColumn struct {
	Name  string
	Width int
}

// Table은 테이블 컴포넌트입니다.
type Table struct {
	columns  []TableColumn
	rows     [][]string
	selected int
	width    int
	height   int
}

// NewTable은 새 테이블을 생성합니다.
func NewTable(columns []TableColumn) *Table {
	return &Table{
		columns:  columns,
		selected: -1,
	}
}

// SetRows는 행을 설정합니다.
func (t *Table) SetRows(rows [][]string) {
	t.rows = rows
}

// SetSelected는 선택된 행을 설정합니다.
func (t *Table) SetSelected(idx int) {
	t.selected = idx
}

// SetSize는 크기를 설정합니다.
func (t *Table) SetSize(width, height int) {
	t.width = width
	t.height = height
}

// View는 테이블을 렌더링합니다.
func (t *Table) View() string {
	var lines []string

	// 헤더
	var headerCells []string
	for _, col := range t.columns {
		cell := fmt.Sprintf("%-*s", col.Width, col.Name)
		headerCells = append(headerCells, cell)
	}
	header := tableHeaderStyle.Render(strings.Join(headerCells, " "))
	lines = append(lines, header)

	// 구분선
	totalWidth := 0
	for _, col := range t.columns {
		totalWidth += col.Width + 1
	}
	separator := tableBorderStyle.Render(strings.Repeat("─", totalWidth))
	lines = append(lines, separator)

	// 행
	for i, row := range t.rows {
		var cells []string
		for j, cell := range row {
			if j < len(t.columns) {
				formatted := fmt.Sprintf("%-*s", t.columns[j].Width, cell)
				cells = append(cells, formatted)
			}
		}
		rowStr := strings.Join(cells, " ")

		var style lipgloss.Style
		if i == t.selected {
			style = tableSelectedStyle
		} else {
			style = tableRowStyle
		}

		lines = append(lines, style.Render(rowStr))
	}

	return strings.Join(lines, "\n")
}
