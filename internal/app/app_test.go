package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YangTaeyoung/claude-switch/internal/config"
	"github.com/YangTaeyoung/claude-switch/internal/keychain"
)

// fakeKeychain은 인메모리 키체인이다. 키는 "service\x00account".
type fakeKeychain struct {
	items map[string]string
}

func newFakeKeychain() *fakeKeychain { return &fakeKeychain{items: map[string]string{}} }

func key(service, account string) string { return service + "\x00" + account }

func (f *fakeKeychain) Get(service, account string) (string, error) {
	v, ok := f.items[key(service, account)]
	if !ok {
		return "", keychain.ErrNotFound
	}
	return v, nil
}

func (f *fakeKeychain) GetByService(service string) (string, string, error) {
	for k, v := range f.items {
		svc, acct, _ := strings.Cut(k, "\x00")
		if svc == service {
			return v, acct, nil
		}
	}
	return "", "", keychain.ErrNotFound
}

func (f *fakeKeychain) Set(service, account, value string) error {
	f.items[key(service, account)] = value
	return nil
}

func (f *fakeKeychain) Delete(service, account string) error {
	k := key(service, account)
	if _, ok := f.items[k]; !ok {
		return keychain.ErrNotFound
	}
	delete(f.items, k)
	return nil
}

// newTestApp은 임시 디렉토리 기반 App과 fake 키체인을 만든다.
// 모든 자격증명 값은 명백한 가짜다.
func newTestApp(t *testing.T) (*App, *fakeKeychain) {
	t.Helper()
	dir := t.TempDir()
	claudeJSON := filepath.Join(dir, ".claude.json")
	if err := os.WriteFile(claudeJSON, []byte(`{"oauthAccount":{"emailAddress":"acct-a@example.com"},"keep":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	kc := newFakeKeychain()
	a := &App{
		KC:             kc,
		ConfigPath:     filepath.Join(dir, "config.json"),
		ClaudeJSONPath: claudeJSON,
		Out:            &bytes.Buffer{},
		Errw:           &bytes.Buffer{},
	}
	return a, kc
}

func loginAs(t *testing.T, a *App, kc *fakeKeychain, email, cred string) {
	t.Helper()
	if err := kc.Set(ClaudeService, "fakeuser", cred); err != nil {
		t.Fatal(err)
	}
	doc := fmt.Sprintf(`{"oauthAccount":{"emailAddress":%q},"keep":true}`, email)
	if err := os.WriteFile(a.ClaudeJSONPath, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRequiresLogin(t *testing.T) {
	a, _ := newTestApp(t)
	err := a.Save("work")
	if err == nil || !strings.Contains(err.Error(), "/login") {
		t.Fatalf("로그인 안내 에러를 기대: %v", err)
	}
}

func TestSaveThenUseSwapsCredentials(t *testing.T) {
	a, kc := newTestApp(t)

	loginAs(t, a, kc, "acct-a@example.com", "fake-cred-A")
	if err := a.Save("work"); err != nil {
		t.Fatal(err)
	}
	loginAs(t, a, kc, "acct-b@example.com", "fake-cred-B")
	if err := a.Save("personal"); err != nil {
		t.Fatal(err)
	}

	if err := a.Use("work"); err != nil {
		t.Fatal(err)
	}

	// Claude Code 키체인 항목이 work 자격증명으로 교체됨
	got, _, err := kc.GetByService(ClaudeService)
	if err != nil {
		t.Fatal(err)
	}
	if got != "fake-cred-A" {
		t.Fatalf("키체인 = %q, want fake-cred-A", got)
	}

	// ~/.claude.json oauthAccount도 work 계정으로 교체, 다른 필드 보존
	data, err := os.ReadFile(a.ClaudeJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc["oauthAccount"]), "acct-a@example.com") {
		t.Fatalf("oauthAccount = %s", doc["oauthAccount"])
	}
	if _, ok := doc["keep"]; !ok {
		t.Fatal("keep 필드 유실")
	}
}

func TestUseSyncsBackRotatedToken(t *testing.T) {
	a, kc := newTestApp(t)

	loginAs(t, a, kc, "acct-a@example.com", "fake-cred-A-v1")
	if err := a.Save("work"); err != nil {
		t.Fatal(err)
	}
	loginAs(t, a, kc, "acct-b@example.com", "fake-cred-B")
	if err := a.Save("personal"); err != nil {
		t.Fatal(err)
	}

	// personal 활성 상태에서 Claude Code가 토큰을 회전시킴
	if err := kc.Set(ClaudeService, "fakeuser", "fake-cred-B-rotated"); err != nil {
		t.Fatal(err)
	}

	if err := a.Use("work"); err != nil {
		t.Fatal(err)
	}

	// sync-back: personal 프로필에 회전된 토큰이 반영되어야 함
	got, err := kc.Get(ProfileService, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if got != "fake-cred-B-rotated" {
		t.Fatalf("personal 프로필 = %q, want fake-cred-B-rotated", got)
	}
}

func TestNextCycles(t *testing.T) {
	a, kc := newTestApp(t)

	loginAs(t, a, kc, "acct-a@example.com", "fake-cred-A")
	if err := a.Save("work"); err != nil {
		t.Fatal(err)
	}
	loginAs(t, a, kc, "acct-b@example.com", "fake-cred-B")
	if err := a.Save("personal"); err != nil {
		t.Fatal(err)
	}

	// 현재 active=personal → next는 work
	if err := a.Next(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Active != "work" {
		t.Fatalf("Active = %q, want work", cfg.Active)
	}
}

func TestDeleteActiveProfileRefused(t *testing.T) {
	a, kc := newTestApp(t)
	loginAs(t, a, kc, "acct-a@example.com", "fake-cred-A")
	if err := a.Save("work"); err != nil {
		t.Fatal(err)
	}
	if err := a.Delete("work"); err == nil {
		t.Fatal("활성 프로필 삭제는 거부되어야 함")
	}
}

func TestDeleteRemovesProfile(t *testing.T) {
	a, kc := newTestApp(t)
	loginAs(t, a, kc, "acct-a@example.com", "fake-cred-A")
	if err := a.Save("work"); err != nil {
		t.Fatal(err)
	}
	loginAs(t, a, kc, "acct-b@example.com", "fake-cred-B")
	if err := a.Save("personal"); err != nil {
		t.Fatal(err)
	}

	if err := a.Delete("work"); err != nil {
		t.Fatal(err)
	}
	if _, err := kc.Get(ProfileService, "work"); err == nil {
		t.Fatal("키체인 프로필 항목이 남아 있음")
	}
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Find("work") != nil {
		t.Fatal("config에 프로필이 남아 있음")
	}
}
