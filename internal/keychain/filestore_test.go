package keychain

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const claudeSvc = "Claude Code-credentials"
const profileSvc = "claude-switch-profile"

func newFileStore(t *testing.T) FileStore {
	t.Helper()
	dir := t.TempDir()
	return FileStore{
		ClaudeService:  claudeSvc,
		ClaudeCredPath: filepath.Join(dir, "claude", ".credentials.json"),
		BaseDir:        filepath.Join(dir, "credentials"),
	}
}

func TestFileStoreClaudeRoundTrip(t *testing.T) {
	f := newFileStore(t)

	// 아직 없음
	if _, err := f.Get(claudeSvc, "anyacct"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get on missing claude cred = %v, want ErrNotFound", err)
	}

	// Set은 account를 무시하고 고정 경로에 쓴다.
	const cred = `{"claudeAiOauth":{"accessToken":"tok"}}`
	if err := f.Set(claudeSvc, "alice", cred); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// 값이 통째로 보존되어야 한다 (trim 없음).
	got, err := f.Get(claudeSvc, "ignored")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != cred {
		t.Fatalf("Get = %q, want %q", got, cred)
	}

	// GetByService는 값과 빈 account를 반환한다.
	v, acct, err := f.GetByService(claudeSvc)
	if err != nil {
		t.Fatalf("GetByService: %v", err)
	}
	if v != cred || acct != "" {
		t.Fatalf("GetByService = (%q, %q), want (%q, \"\")", v, acct, cred)
	}

	// 파일 권한은 0600이어야 한다.
	info, err := os.Stat(f.ClaudeCredPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
}

func TestFileStoreProfileRoundTrip(t *testing.T) {
	f := newFileStore(t)

	if err := f.Set(profileSvc, "work", "secret-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := f.Set(profileSvc, "personal", "secret-2"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := f.Get(profileSvc, "work")
	if err != nil || got != "secret-1" {
		t.Fatalf("Get(work) = %q, %v", got, err)
	}
	got, err = f.Get(profileSvc, "personal")
	if err != nil || got != "secret-2" {
		t.Fatalf("Get(personal) = %q, %v", got, err)
	}

	// Delete 후 ErrNotFound
	if err := f.Delete(profileSvc, "work"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := f.Get(profileSvc, "work"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete = %v, want ErrNotFound", err)
	}
	// 남은 프로필은 영향 없음
	if got, _ := f.Get(profileSvc, "personal"); got != "secret-2" {
		t.Fatalf("personal profile affected: %q", got)
	}
}

func TestFileStoreDeleteMissing(t *testing.T) {
	f := newFileStore(t)
	if err := f.Delete(profileSvc, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete missing = %v, want ErrNotFound", err)
	}
}

func TestFileStoreGetByServiceUnsupported(t *testing.T) {
	f := newFileStore(t)
	if _, _, err := f.GetByService(profileSvc); err == nil {
		t.Fatal("GetByService(profileSvc) should error")
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"work":        "work",
		"a/b":         "a_b",
		`a\b`:         "a_b",
		"../etc":      ".._etc",
		"with space":  "with space",
	}
	for in, want := range cases {
		got, err := sanitize(in)
		if err != nil {
			t.Errorf("sanitize(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", ".", ".."} {
		if _, err := sanitize(bad); err == nil {
			t.Errorf("sanitize(%q) should error", bad)
		}
	}
}

// FileStore가 Keychain 인터페이스를 만족하는지 컴파일 타임 보증.
var _ Keychain = FileStore{}
