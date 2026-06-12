package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/YangTaeyoung/claude-switch/internal/i18n"
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

// logoLines는 홈 화면의 ASCII 워드마크다 (figlet smslant).
var logoLines = []string{
	"      __             __                  _ __      __",
	" ____/ /__ ___ _____/ /__ ________    __(_) /_____/ /",
	"/ __/ / _ `/ // / _  / -_)___(_-< |/|/ / / __/ __/ _ \\",
	"\\__/_/\\_,_/\\_,_/\\_,_/\\__/   /___/__,__/_/\\__/\\__/_//_/",
}

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
	var content string
	switch m.screen {
	case screenHome:
		content = m.viewHome()
	case screenProfiles:
		content = m.viewProfiles()
	case screenSave:
		content = m.viewInput(i18n.T("save.title"))
	case screenRename:
		content = m.viewInput(fmt.Sprintf(i18n.T("rename.title"), m.renameTarget))
	case screenSettings:
		content = m.viewSettings()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *Model) viewHome() string {
	var b strings.Builder
	b.WriteString("\n")
	for _, line := range logoLines {
		b.WriteString("  " + titleStyle.Render(line) + "\n")
	}
	b.WriteString("  " + dimStyle.Render("v"+m.version) + "\n\n")

	if len(m.rows) == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(colorWarn).Render(i18n.T("home.noProfile")) + "\n\n")
	}

	items := []string{
		fmt.Sprintf(i18n.T("menu.manage"), len(m.rows)),
		i18n.T("menu.save"),
		i18n.T("menu.settings"),
		i18n.T("menu.quit"),
	}
	for i, label := range items {
		cursor := "  "
		styled := label
		if i == m.menuCursor {
			cursor = activeStyle.Render("▸ ")
			styled = nameStyle.Render(label)
		}
		b.WriteString("  " + cursor + styled + "\n")
	}

	b.WriteString("\n" + m.statusLine())
	b.WriteString("  " + dimStyle.Render(i18n.T("home.help")) + "\n")
	return b.String()
}

func (m *Model) viewProfiles() string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render(i18n.T("profiles.title")) + "\n\n")

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
		b.WriteString("  " + errStyle.Render(fmt.Sprintf(i18n.T("confirm.delete"), m.rows[m.cursor].profile.Name)) + "\n")
	} else {
		b.WriteString(m.statusLine())
	}
	b.WriteString("  " + dimStyle.Render(i18n.T("profiles.help")) + "\n")
	return b.String()
}

func (m *Model) viewInput(title string) string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render(title) + "\n\n")
	b.WriteString("  " + m.input.View() + "\n\n")
	b.WriteString(m.statusLine())
	b.WriteString("  " + dimStyle.Render(i18n.T("input.help")) + "\n")
	return b.String()
}

func (m *Model) viewSettings() string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render(i18n.T("settings.title")) + "\n\n")
	b.WriteString("  " + dimStyle.Render(i18n.T("settings.language")) + "\n")

	labels := map[i18n.Lang]string{i18n.EN: "English", i18n.KO: "한국어"}
	for i, l := range languages {
		cursor := "  "
		label := labels[l]
		if i == m.settingsCursor {
			cursor = activeStyle.Render("▸ ")
			label = nameStyle.Render(label)
		}
		check := "  "
		if l == i18n.Current() {
			check = okStyle.Render("✓ ")
		}
		b.WriteString("  " + cursor + check + label + "\n")
	}

	b.WriteString("\n" + m.statusLine())
	b.WriteString("  " + dimStyle.Render(i18n.T("settings.help")) + "\n")
	return b.String()
}

// statusLine은 상태 메시지 한 줄(없으면 빈 줄)을 만든다.
func (m *Model) statusLine() string {
	if m.status == "" {
		return "\n"
	}
	style := okStyle
	if m.statusErr {
		style = errStyle
	}
	return "  " + style.Render(m.status) + "\n"
}

func (m *Model) usageLines(r row) string {
	const indent = "        "
	if !r.loaded {
		return indent + dimStyle.Render(spinFrames[m.spin%len(spinFrames)]+" "+i18n.T("profiles.loading")) + "\n"
	}
	if r.usage.Err != nil {
		return indent + dimStyle.Render(i18n.T("profiles.noLimit")+" ("+r.usage.Err.Error()+")") + "\n"
	}
	return indent + windowLine("5h", r.usage.FiveHour) + "\n" + indent + windowLine("7d", r.usage.SevenDay) + "\n"
}

func windowLine(label string, w limit.Window) string {
	if w.Utilization < 0 {
		return dimStyle.Render(label + "  n/a")
	}
	line := fmt.Sprintf("%s  %s %3.0f%%", dimStyle.Render(label), gauge(w.Utilization, barWidth), w.Utilization*100)
	if !w.ResetsAt.IsZero() {
		line += dimStyle.Render("  " + i18n.T("profiles.resets") + " " + w.ResetsAt.Local().Format("01-02 15:04"))
	}
	return line
}
