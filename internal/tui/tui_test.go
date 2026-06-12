package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/YangTaeyoung/claude-switch/internal/limit"
)

// fakeBackend는 테스트용 인메모리 백엔드. 모든 값은 가짜다.
type fakeBackend struct {
	profiles []Profile
	active   string
	switched []string
	deleted  []string
}

func (f *fakeBackend) Profiles() ([]Profile, string, error) { return f.profiles, f.active, nil }

func (f *fakeBackend) Switch(name string) error {
	f.switched = append(f.switched, name)
	f.active = name
	return nil
}

func (f *fakeBackend) Delete(name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

func (f *fakeBackend) Usage(ctx context.Context, name string) limit.Result {
	return limit.Result{Status: "allowed"}
}

func newTestModel(t *testing.T) (*Model, *fakeBackend) {
	t.Helper()
	b := &fakeBackend{
		profiles: []Profile{{Name: "work", Email: "a@example.com"}, {Name: "personal", Email: "b@example.com"}},
		active:   "work",
	}
	m, err := NewModel(b)
	if err != nil {
		t.Fatal(err)
	}
	return m, b
}

func TestCursorMovement(t *testing.T) {
	m, _ := newTestModel(t)
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
	// cursor=0 → work(활성)
	m.handleKey("d")
	if m.confirmDelete {
		t.Fatal("활성 프로필은 확인 모드에 진입하면 안 됨")
	}
	if !strings.Contains(m.status, "active") {
		t.Fatalf("status = %q", m.status)
	}
}
