// Package tui는 claude-switch의 인터랙티브 대시보드다 (Bubble Tea v2).
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/YangTaeyoung/claude-switch/internal/app"
	"github.com/YangTaeyoung/claude-switch/internal/i18n"
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
	Save(name string) error
	Switch(name string) error
	Rename(oldName, newName string) error
	Delete(name string) error
	Usage(ctx context.Context, name string) limit.Result
	Language() i18n.Lang
	SetLanguage(l i18n.Lang) error
}

// screen은 TUI 화면 식별자다.
type screen int

const (
	screenHome screen = iota
	screenProfiles
	screenSave
	screenRename
	screenSettings
)

const (
	menuManage = iota
	menuSave
	menuSettings
	menuQuit
	menuCount
)

var languages = []i18n.Lang{i18n.EN, i18n.KO}

type row struct {
	profile Profile
	usage   limit.Result
	loaded  bool
}

type Model struct {
	backend Backend
	version string

	screen         screen
	rows           []row
	active         string
	cursor         int
	menuCursor     int
	settingsCursor int
	confirmDelete  bool
	input          textinput.Model
	renameTarget   string
	status         string
	statusErr      bool
	spin           int
	width          int
	fetched        bool
}

type usageMsg struct {
	name   string
	result limit.Result
}

type actionMsg struct {
	status  string
	isErr   bool
	reload  bool
	fetch   bool
	goTo    screen
	hasGoTo bool
}

type tickMsg time.Time

func NewModel(b Backend, version string) (*Model, error) {
	i18n.SetLang(b.Language())
	in := textinput.New()
	in.CharLimit = 40
	m := &Model{backend: b, version: version, input: in}
	if err := m.reload(); err != nil {
		return nil, err
	}
	if len(m.rows) == 0 {
		m.menuCursor = menuSave // 프로필이 없으면 저장이 기본 선택
	}
	for i, l := range languages {
		if l == b.Language() {
			m.settingsCursor = i
		}
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
	m.fetched = true
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

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.screen == screenSave || m.screen == screenRename {
			switch key {
			case "enter":
				return m, m.submitInput()
			case "esc":
				m.status, m.statusErr = "", false
				if m.screen == screenRename {
					m.screen = screenProfiles
				} else {
					m.screen = screenHome
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		}
		if key == "q" && !m.confirmDelete {
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
		if msg.hasGoTo {
			m.screen = msg.goTo
		}
		if msg.fetch {
			return m, m.fetchAll()
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

// handleKey는 입력 화면 외 키 입력을 처리한다. 테스트에서 직접 호출한다.
func (m *Model) handleKey(key string) tea.Cmd {
	switch m.screen {
	case screenHome:
		return m.handleHomeKey(key)
	case screenProfiles:
		return m.handleProfilesKey(key)
	case screenSettings:
		return m.handleSettingsKey(key)
	}
	return nil
}

func (m *Model) handleHomeKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < menuCount-1 {
			m.menuCursor++
		}
	case "enter":
		switch m.menuCursor {
		case menuManage:
			if len(m.rows) == 0 {
				m.status, m.statusErr = i18n.T("home.noProfile"), false
				return nil
			}
			m.status, m.statusErr = "", false
			m.screen = screenProfiles
			if !m.fetched {
				return m.fetchAll()
			}
		case menuSave:
			m.status, m.statusErr = "", false
			m.openInput(i18n.T("save.prompt"))
			m.screen = screenSave
			return m.input.Focus()
		case menuSettings:
			m.status, m.statusErr = "", false
			m.screen = screenSettings
		case menuQuit:
			return tea.Quit
		}
	}
	return nil
}

func (m *Model) handleProfilesKey(key string) tea.Cmd {
	if m.confirmDelete {
		switch key {
		case "y":
			m.confirmDelete = false
			name := m.rows[m.cursor].profile.Name
			return func() tea.Msg {
				if err := m.backend.Delete(name); err != nil {
					return actionMsg{status: err.Error(), isErr: true}
				}
				return actionMsg{status: fmt.Sprintf(i18n.T("status.deleted"), name), reload: true}
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
			return actionMsg{status: fmt.Sprintf(i18n.T("status.switched"), name), reload: true}
		}
	case "r":
		m.status, m.statusErr = "", false
		return m.fetchAll()
	case "n":
		m.renameTarget = m.rows[m.cursor].profile.Name
		m.openInput(i18n.T("rename.prompt"))
		m.screen = screenRename
		return m.input.Focus()
	case "d":
		if m.rows[m.cursor].profile.Name == m.active {
			m.status, m.statusErr = i18n.T("status.activeDelete"), true
			return nil
		}
		m.confirmDelete = true
	case "esc":
		m.status, m.statusErr = "", false
		m.screen = screenHome
	}
	return nil
}

func (m *Model) handleSettingsKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if m.settingsCursor < len(languages)-1 {
			m.settingsCursor++
		}
	case "enter":
		lang := languages[m.settingsCursor]
		if err := m.backend.SetLanguage(lang); err != nil {
			m.status, m.statusErr = err.Error(), true
			return nil
		}
		i18n.SetLang(lang)
		m.status, m.statusErr = i18n.T("status.langSet"), false
	case "esc":
		m.status, m.statusErr = "", false
		m.screen = screenHome
	}
	return nil
}

func (m *Model) openInput(placeholder string) {
	m.input.Reset()
	m.input.Placeholder = placeholder
}

// submitInput은 save/rename 입력을 확정한다.
func (m *Model) submitInput() tea.Cmd {
	name := strings.TrimSpace(m.input.Value())
	if name == "" {
		m.status, m.statusErr = i18n.T("save.empty"), true
		return nil
	}
	switch m.screen {
	case screenSave:
		return func() tea.Msg {
			if err := m.backend.Save(name); err != nil {
				return actionMsg{status: err.Error(), isErr: true}
			}
			return actionMsg{
				status: fmt.Sprintf(i18n.T("status.saved"), name),
				reload: true, fetch: true, goTo: screenProfiles, hasGoTo: true,
			}
		}
	case screenRename:
		oldName := m.renameTarget
		return func() tea.Msg {
			if err := m.backend.Rename(oldName, name); err != nil {
				return actionMsg{status: err.Error(), isErr: true}
			}
			return actionMsg{
				status: fmt.Sprintf(i18n.T("status.renamed"), oldName, name),
				reload: true, goTo: screenProfiles, hasGoTo: true,
			}
		}
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

func (b appBackend) Save(name string) error               { return b.a.Save(name) }
func (b appBackend) Switch(name string) error             { return b.a.Use(name) }
func (b appBackend) Rename(oldName, newName string) error { return b.a.Rename(oldName, newName) }
func (b appBackend) Delete(name string) error             { return b.a.Delete(name) }

func (b appBackend) Usage(ctx context.Context, name string) limit.Result {
	cfg, err := b.a.Config()
	if err != nil {
		return limit.Result{Err: err}
	}
	return b.a.Usage(ctx, cfg, name, limit.Check)
}

func (b appBackend) Language() i18n.Lang {
	cfg, err := b.a.Config()
	if err != nil {
		return i18n.EN
	}
	return i18n.Lang(cfg.Language)
}

func (b appBackend) SetLanguage(l i18n.Lang) error {
	cfg, err := b.a.Config()
	if err != nil {
		return err
	}
	cfg.Language = string(l)
	return cfg.Save(b.a.ConfigPath)
}

// Run은 TUI를 시작한다. 데모 모드(CLAUDE_SWITCH_DEMO=1)면 가짜 데이터를 쓴다.
func Run(a *app.App, version string) error {
	a.Out, a.Errw = io.Discard, io.Discard
	var b Backend = appBackend{a: a}
	if os.Getenv("CLAUDE_SWITCH_DEMO") == "1" {
		b = newDemoBackend()
	}
	m, err := NewModel(b, version)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
