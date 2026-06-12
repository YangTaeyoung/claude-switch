package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/YangTaeyoung/claude-switch/internal/i18n"
	"github.com/YangTaeyoung/claude-switch/internal/limit"
)

// fakeBackend는 테스트용 인메모리 백엔드. 모든 값은 가짜다.
type fakeBackend struct {
	profiles []Profile
	active   string
	lang     i18n.Lang
	switched []string
	saved    []string
	renamed  [][2]string
	deleted  []string
}

func (f *fakeBackend) Profiles() ([]Profile, string, error) { return f.profiles, f.active, nil }

func (f *fakeBackend) Save(name string) error {
	f.saved = append(f.saved, name)
	f.profiles = append(f.profiles, Profile{Name: name, Email: name + "@example.com"})
	f.active = name
	return nil
}

func (f *fakeBackend) Switch(name string) error {
	f.switched = append(f.switched, name)
	f.active = name
	return nil
}

func (f *fakeBackend) Rename(oldName, newName string) error {
	f.renamed = append(f.renamed, [2]string{oldName, newName})
	for i := range f.profiles {
		if f.profiles[i].Name == oldName {
			f.profiles[i].Name = newName
		}
	}
	if f.active == oldName {
		f.active = newName
	}
	return nil
}

func (f *fakeBackend) Delete(name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

func (f *fakeBackend) Usage(ctx context.Context, name string) limit.Result {
	return limit.Result{Status: "allowed"}
}

func (f *fakeBackend) Language() i18n.Lang { return f.lang }

func (f *fakeBackend) SetLanguage(l i18n.Lang) error {
	f.lang = l
	return nil
}

func newTestModel(t *testing.T) (*Model, *fakeBackend) {
	t.Helper()
	t.Cleanup(func() { i18n.SetLang(i18n.EN) })
	b := &fakeBackend{
		profiles: []Profile{{Name: "work", Email: "a@example.com"}, {Name: "personal", Email: "b@example.com"}},
		active:   "work",
		lang:     i18n.EN,
	}
	m, err := NewModel(b, "test")
	if err != nil {
		t.Fatal(err)
	}
	return m, b
}

// drain은 cmd를 실행해 나온 actionMsg를 모델에 반영한다 (fetch로 인한 후속 cmd는 무시).
func drain(t *testing.T, m *Model, cmd func() interface{ String() string }) {
	t.Helper()
}

func applyAction(t *testing.T, m *Model, msg any) {
	t.Helper()
	if _, ok := msg.(actionMsg); !ok {
		t.Fatalf("actionMsg가 아님: %#v", msg)
	}
	m.Update(msg)
}

// ── 홈 화면 ──

func TestHomeMenuNavigationAndManage(t *testing.T) {
	m, _ := newTestModel(t)
	if m.screen != screenHome {
		t.Fatal("초기 화면은 home")
	}
	if m.menuCursor != menuManage {
		t.Fatalf("프로필 있으면 기본 커서 = Manage, got %d", m.menuCursor)
	}
	m.handleKey("down")
	m.handleKey("up")
	cmd := m.handleKey("enter") // Manage 선택
	if m.screen != screenProfiles {
		t.Fatalf("Manage 후 화면 = %d", m.screen)
	}
	if cmd == nil {
		t.Fatal("첫 진입은 사용량 fetch cmd를 기대")
	}
}

func TestHomeDefaultCursorWhenNoProfiles(t *testing.T) {
	t.Cleanup(func() { i18n.SetLang(i18n.EN) })
	b := &fakeBackend{lang: i18n.EN}
	m, err := NewModel(b, "test")
	if err != nil {
		t.Fatal(err)
	}
	if m.menuCursor != menuSave {
		t.Fatalf("프로필 0개면 기본 커서 = Save, got %d", m.menuCursor)
	}
	// Manage 선택해도 진입하지 않고 안내만
	m.menuCursor = menuManage
	m.handleKey("enter")
	if m.screen != screenHome {
		t.Fatal("프로필 0개에서 Manage는 home 유지")
	}
	if m.status == "" {
		t.Fatal("안내 문구를 기대")
	}
}

func TestHomeQuitItem(t *testing.T) {
	m, _ := newTestModel(t)
	m.menuCursor = menuQuit
	if cmd := m.handleKey("enter"); cmd == nil {
		t.Fatal("Quit은 cmd(tea.Quit)를 반환해야 함")
	}
}

// ── 저장 플로우 ──

func TestSaveFlow(t *testing.T) {
	m, b := newTestModel(t)
	m.menuCursor = menuSave
	m.handleKey("enter")
	if m.screen != screenSave {
		t.Fatalf("화면 = %d, want save", m.screen)
	}

	m.input.SetValue("  newbie  ")
	cmd := m.submitInput()
	if cmd == nil {
		t.Fatal("submit은 cmd를 기대")
	}
	msg := cmd()
	applyAction(t, m, msg)

	if len(b.saved) != 1 || b.saved[0] != "newbie" {
		t.Fatalf("saved = %v", b.saved)
	}
	if m.screen != screenProfiles {
		t.Fatalf("저장 후 화면 = %d, want profiles", m.screen)
	}
}

func TestSaveEmptyNameRejected(t *testing.T) {
	m, b := newTestModel(t)
	m.screen = screenSave
	m.input.SetValue("   ")
	if cmd := m.submitInput(); cmd != nil {
		t.Fatal("빈 이름은 cmd 없이 거부")
	}
	if len(b.saved) != 0 || !m.statusErr {
		t.Fatalf("saved=%v statusErr=%v", b.saved, m.statusErr)
	}
}

// ── 이름 변경 플로우 ──

func TestRenameFlow(t *testing.T) {
	m, b := newTestModel(t)
	m.screen = screenProfiles
	m.handleKey("down") // personal
	m.handleKey("n")
	if m.screen != screenRename || m.renameTarget != "personal" {
		t.Fatalf("screen=%d target=%q", m.screen, m.renameTarget)
	}

	m.input.SetValue("personal-renamed")
	cmd := m.submitInput()
	if cmd == nil {
		t.Fatal("submit은 cmd를 기대")
	}
	applyAction(t, m, cmd())

	if len(b.renamed) != 1 || b.renamed[0] != [2]string{"personal", "personal-renamed"} {
		t.Fatalf("renamed = %v", b.renamed)
	}
	if m.screen != screenProfiles {
		t.Fatalf("이름 변경 후 화면 = %d", m.screen)
	}
}

// ── 설정 (언어) ──

func TestSettingsLanguageSwitch(t *testing.T) {
	m, b := newTestModel(t)
	m.menuCursor = menuSettings
	m.handleKey("enter")
	if m.screen != screenSettings {
		t.Fatalf("화면 = %d", m.screen)
	}

	m.handleKey("down") // 한국어
	m.handleKey("enter")
	if b.lang != i18n.KO {
		t.Fatalf("backend lang = %q", b.lang)
	}
	if i18n.Current() != i18n.KO {
		t.Fatal("i18n 즉시 반영 실패")
	}

	m.handleKey("esc")
	if m.screen != screenHome {
		t.Fatal("esc로 home 복귀")
	}
}

// ── 프로필 화면 (기존 동작 회귀) ──

func TestCursorMovement(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = screenProfiles
	if m.cursor != 0 {
		t.Fatalf("초기 cursor = %d", m.cursor)
	}
	m.handleKey("down")
	if m.cursor != 1 {
		t.Fatalf("down 후 cursor = %d", m.cursor)
	}
	m.handleKey("down") // 끝에서 더 내려가지 않음
	if m.cursor != 1 {
		t.Fatalf("경계 초과 cursor = %d", m.cursor)
	}
	m.handleKey("k")
	if m.cursor != 0 {
		t.Fatalf("k 후 cursor = %d", m.cursor)
	}
}

func TestSwitchOnEnter(t *testing.T) {
	m, b := newTestModel(t)
	m.screen = screenProfiles
	m.handleKey("down")
	cmd := m.handleKey("enter")
	if cmd == nil {
		t.Fatal("enter는 cmd를 반환해야 함")
	}
	msg := cmd()
	am, ok := msg.(actionMsg)
	if !ok || am.isErr {
		t.Fatalf("msg = %#v", msg)
	}
	if len(b.switched) != 1 || b.switched[0] != "personal" {
		t.Fatalf("switched = %v", b.switched)
	}
}

func TestDeleteConfirmFlow(t *testing.T) {
	m, b := newTestModel(t)
	m.screen = screenProfiles
	m.handleKey("down") // personal (비활성)
	m.handleKey("d")
	if !m.confirmDelete {
		t.Fatal("d 후 confirmDelete = false")
	}
	m.handleKey("n")
	if m.confirmDelete {
		t.Fatal("n 후에도 confirmDelete = true")
	}
	if len(b.deleted) != 0 {
		t.Fatal("취소했는데 삭제됨")
	}

	m.handleKey("d")
	cmd := m.handleKey("y")
	if cmd == nil {
		t.Fatal("y는 cmd를 반환해야 함")
	}
	cmd()
	if len(b.deleted) != 1 || b.deleted[0] != "personal" {
		t.Fatalf("deleted = %v", b.deleted)
	}
}

func TestDeleteActiveProfileBlocked(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = screenProfiles
	// cursor=0 → work(활성)
	m.handleKey("d")
	if m.confirmDelete {
		t.Fatal("활성 프로필은 확인 모드에 진입하면 안 됨")
	}
	if !strings.Contains(m.status, "active") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestProfilesEscGoesHome(t *testing.T) {
	m, _ := newTestModel(t)
	m.screen = screenProfiles
	m.handleKey("esc")
	if m.screen != screenHome {
		t.Fatalf("esc 후 화면 = %d", m.screen)
	}
}
