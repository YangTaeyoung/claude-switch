package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/YangTaeyoung/claude-switch/internal/limit"
)

var (
	colorOK     = lipgloss.Color("42")  // green
	colorWarn   = lipgloss.Color("214") // yellow/orange
	colorDanger = lipgloss.Color("196") // red
	colorAccent = lipgloss.Color("99")  // purple
	colorDim    = lipgloss.Color("241") // gray

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	activeStyle = lipgloss.NewStyle().Foreground(colorAccent)
	dimStyle    = lipgloss.NewStyle().Foreground(colorDim)
	nameStyle   = lipgloss.NewStyle().Bold(true)
	errStyle    = lipgloss.NewStyle().Foreground(colorDanger)
	okStyle     = lipgloss.NewStyle().Foreground(colorOK)
)

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const barWidth = 20

// filledCells는 사용률(0~1)을 채워진 칸 수로 변환한다. 범위 밖은 클램프.
func filledCells(utilization float64, width int) int {
	if utilization < 0 {
		utilization = 0
	}
	if utilization > 1 {
		utilization = 1
	}
	return int(utilization*float64(width) + 0.5)
}

// bar는 사용률(0~1)을 스타일 없는 게이지 문자열로 렌더링한다.
func bar(utilization float64, width int) string {
	filled := filledCells(utilization, width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// gauge는 채워진 부분은 사용률 색, 빈 부분은 회색으로 스타일링한 게이지를 만든다.
func gauge(utilization float64, width int) string {
	filled := filledCells(utilization, width)
	fill := lipgloss.NewStyle().Foreground(utilColor(utilization)).Render(strings.Repeat("█", filled))
	rest := dimStyle.Render(strings.Repeat("░", width-filled))
	return fill + rest
}

// utilColor는 사용률 구간별 게이지 색을 고른다.
func utilColor(utilization float64) color.Color {
	switch {
	case utilization < 0.5:
		return colorOK
	case utilization < 0.8:
		return colorWarn
	default:
		return colorDanger
	}
}

func (m *Model) View() tea.View {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render("claude-switch") + "\n\n")

	for i, r := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("▸ ")
		}
		dot := dimStyle.Render("○")
		name := fmt.Sprintf("%-12s", r.profile.Name)
		if r.profile.Name == m.active {
			dot = activeStyle.Render("●")
			name = nameStyle.Render(name)
		}
		b.WriteString(fmt.Sprintf("  %s%s %s %s\n", cursor, dot, name, dimStyle.Render(r.profile.Email)))
		b.WriteString(m.usageLines(r))
		b.WriteString("\n")
	}

	if m.confirmDelete {
		b.WriteString("  " + errStyle.Render(fmt.Sprintf("Delete %q? (y/n)", m.rows[m.cursor].profile.Name)) + "\n")
	} else if m.status != "" {
		style := okStyle
		if m.statusErr {
			style = errStyle
		}
		b.WriteString("  " + style.Render(m.status) + "\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("  " + dimStyle.Render("↑/↓ move · enter switch · r refresh · d delete · q quit") + "\n")

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func (m *Model) usageLines(r row) string {
	const indent = "        "
	if !r.loaded {
		return indent + dimStyle.Render(spinFrames[m.spin%len(spinFrames)]+" loading...") + "\n"
	}
	if r.usage.Err != nil {
		return indent + dimStyle.Render("limit unavailable ("+r.usage.Err.Error()+")") + "\n"
	}
	return indent + windowLine("5h", r.usage.FiveHour) + "\n" + indent + windowLine("7d", r.usage.SevenDay) + "\n"
}

func windowLine(label string, w limit.Window) string {
	if w.Utilization < 0 {
		return dimStyle.Render(label + "  n/a")
	}
	line := fmt.Sprintf("%s  %s %3.0f%%", dimStyle.Render(label), gauge(w.Utilization, barWidth), w.Utilization*100)
	if !w.ResetsAt.IsZero() {
		line += dimStyle.Render("  resets " + w.ResetsAt.Local().Format("01-02 15:04"))
	}
	return line
}
