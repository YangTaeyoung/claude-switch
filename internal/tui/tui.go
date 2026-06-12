// Package tui는 claude-switch의 인터랙티브 대시보드다 (Bubble Tea v2).
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/YangTaeyoung/claude-switch/internal/app"
	"github.com/YangTaeyoung/claude-switch/internal/limit"
)

// Profile은 TUI에 표시되는 프로필 한 줄이다.
type Profile struct {
	Name  string
	Email string
}

// Backend는 TUI가 사용하는 동작 집합. 실제 구현은 appBackend, 데모는 demoBackend.
type Backend interface {
	Profiles() ([]Profile, string, error) // (목록, 활성 프로필명, 에러)
	Switch(name string) error
	Delete(name string) error
	Usage(ctx context.Context, name string) limit.Result
}

type row struct {
	profile Profile
	usage   limit.Result
	loaded  bool
}

type Model struct {
	backend       Backend
	rows          []row
	active        string
	cursor        int
	confirmDelete bool
	status        string
	statusErr     bool
	spin          int
	width         int
}

type usageMsg struct {
	name   string
	result limit.Result
}

type actionMsg struct {
	status string
	isErr  bool
	reload bool
}

type tickMsg time.Time

func NewModel(b Backend) (*Model, error) {
	m := &Model{backend: b}
	if err := m.reload(); err != nil {
		return nil, err
	}
	return m, nil
}

// reload는 프로필 목록·활성 프로필을 다시 읽는다. 로드된 사용량은 유지한다.
func (m *Model) reload() error {
	profiles, active, err := m.backend.Profiles()
	if err != nil {
		return err
	}
	prev := map[string]row{}
	for _, r := range m.rows {
		prev[r.profile.Name] = r
	}
	rows := make([]row, 0, len(profiles))
	for _, p := range profiles {
		r := row{profile: p}
		if old, ok := prev[p.Name]; ok {
			r.usage, r.loaded = old.usage, old.loaded
		}
		rows = append(rows, r)
	}
	m.rows = rows
	m.active = active
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return nil
}

// fetchAll은 모든 프로필의 사용량을 병렬로 다시 가져온다.
func (m *Model) fetchAll() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.rows)+1)
	for i := range m.rows {
		name := m.rows[i].profile.Name
		m.rows[i].loaded = false
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			return usageMsg{name: name, result: m.backend.Usage(ctx, name)}
		})
	}
	cmds = append(cmds, tick())
	return tea.Batch(cmds...)
}

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Init() tea.Cmd { return m.fetchAll() }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" || (key == "q" && !m.confirmDelete) {
			return m, tea.Quit
		}
		return m, m.handleKey(key)
	case usageMsg:
		for i := range m.rows {
			if m.rows[i].profile.Name == msg.name {
				m.rows[i].usage = msg.result
				m.rows[i].loaded = true
			}
		}
		return m, nil
	case actionMsg:
		m.status, m.statusErr = msg.status, msg.isErr
		if msg.reload {
			if err := m.reload(); err != nil {
				m.status, m.statusErr = err.Error(), true
			}
		}
		return m, nil
	case tickMsg:
		m.spin++
		for _, r := range m.rows {
			if !r.loaded {
				return m, tick()
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	}
	return m, nil
}

// handleKey는 키 문자열 하나를 처리한다. 테스트에서 직접 호출한다.
func (m *Model) handleKey(key string) tea.Cmd {
	if m.confirmDelete {
		switch key {
		case "y":
			m.confirmDelete = false
			name := m.rows[m.cursor].profile.Name
			return func() tea.Msg {
				if err := m.backend.Delete(name); err != nil {
					return actionMsg{status: err.Error(), isErr: true}
				}
				return actionMsg{status: "✓ Deleted " + name, reload: true}
			}
		case "n", "esc":
			m.confirmDelete = false
		}
		return nil
	}
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "enter":
		name := m.rows[m.cursor].profile.Name
		return func() tea.Msg {
			if err := m.backend.Switch(name); err != nil {
				return actionMsg{status: err.Error(), isErr: true}
			}
			return actionMsg{status: "✓ Switched to " + name, reload: true}
		}
	case "r":
		m.status, m.statusErr = "", false
		return m.fetchAll()
	case "d":
		if m.rows[m.cursor].profile.Name == m.active {
			m.status, m.statusErr = "cannot delete the active profile", true
			return nil
		}
		m.confirmDelete = true
	}
	return nil
}

// appBackend는 실제 app.App을 Backend로 감싼다.
type appBackend struct{ a *app.App }

func (b appBackend) Profiles() ([]Profile, string, error) {
	cfg, err := b.a.Config()
	if err != nil {
		return nil, "", err
	}
	out := make([]Profile, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		out = append(out, Profile{Name: p.Name, Email: p.Email})
	}
	return out, cfg.Active, nil
}

func (b appBackend) Switch(name string) error { return b.a.Use(name) }
func (b appBackend) Delete(name string) error { return b.a.Delete(name) }

func (b appBackend) Usage(ctx context.Context, name string) limit.Result {
	cfg, err := b.a.Config()
	if err != nil {
		return limit.Result{Err: err}
	}
	return b.a.Usage(ctx, cfg, name, limit.Check)
}

// Run은 TUI를 시작한다. 데모 모드(CLAUDE_SWITCH_DEMO=1)면 가짜 데이터를 쓴다.
func Run(a *app.App) error {
	a.Out, a.Errw = io.Discard, io.Discard
	var b Backend = appBackend{a: a}
	if os.Getenv("CLAUDE_SWITCH_DEMO") == "1" {
		b = newDemoBackend()
	}
	m, err := NewModel(b)
	if err != nil {
		return err
	}
	if len(m.rows) == 0 {
		return fmt.Errorf("no profiles registered. Log in with claude /login, then run: claude-switch save <name>")
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
