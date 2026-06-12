# claude-switch 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** macOS 키체인의 Claude Code 자격증명을 프로필 단위로 스냅샷·교체해 명령 하나로 구독 계정을 전환하는 Go CLI.

**Architecture:** `security` CLI를 감싼 keychain 패키지 위에, 프로필 메타데이터(config)·`~/.claude.json` 조작(claudejson)·핵심 전환 로직(app)을 둔다. 비밀값은 키체인에만 저장하고 저장소·설정 파일에는 절대 쓰지 않는다. 전환 시 sync-back(현재 키체인 → 활성 프로필 갱신) 후 교체한다.

**Tech Stack:** Go 1.25, stdlib만 사용 (외부 의존성 0).

**민감 정보 규칙 (전 태스크 공통):** 저장소 내 어떤 파일에도 실제 토큰·이메일·계정 정보를 쓰지 않는다. 테스트 픽스처는 `fake-access-token-A` 같은 명백한 가짜 값만 사용한다. 마지막 태스크에서 전체 감사를 수행한다.

---

## 파일 구조

```
claude-switch/
├── go.mod                            # module github.com/YangTaeyoung/claude-switch
├── .gitignore                        # bin/
├── README.md
├── main.go                           # 서브커맨드 디스패치
└── internal/
    ├── keychain/
    │   ├── keychain.go               # Keychain 인터페이스 + security CLI 구현
    │   └── keychain_test.go          # acct 파싱 단위 테스트
    ├── config/
    │   ├── config.go                 # ~/.config/claude-switch/config.json (비밀값 없음)
    │   └── config_test.go
    ├── claudejson/
    │   ├── claudejson.go             # ~/.claude.json oauthAccount read-modify-write
    │   └── claudejson_test.go
    ├── app/
    │   ├── app.go                    # save/use/next/list/status/delete 핵심 로직
    │   └── app_test.go               # fake keychain으로 sync-back 등 검증
    └── limit/
        └── limit.go                  # 리밋 상태 조회 (베스트 에포트)
```

---

### Task 1: 프로젝트 스캐폴딩

**Files:**
- Create: `go.mod`, `.gitignore`, `README.md`(스텁)

- [ ] **Step 1: go.mod / .gitignore 생성**

```bash
cd ~/dev/personal/claude-switch
go mod init github.com/YangTaeyoung/claude-switch
printf 'bin/\n' > .gitignore
```

- [ ] **Step 2: README 스텁 작성**

```markdown
# claude-switch

Claude Code 구독 계정을 여러 개 등록해두고 명령 하나로 전환하는 macOS 전용 CLI.

> ⚠️ 이 저장소에는 어떤 시크릿·계정 정보도 커밋하지 않는다. 자격증명은 macOS 키체인에만 저장된다.

(사용법은 구현 완료 후 작성)
```

- [ ] **Step 3: 커밋**

```bash
git add -A && git commit -m "chore: 프로젝트 스캐폴딩"
```

---

### Task 2: config 패키지 (TDD)

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: 실패하는 테스트 작성**

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestNext(t *testing.T) {
	t.Run("프로필이 2개 미만이면 에러", func(t *testing.T) {
		c := &Config{Profiles: []Profile{{Name: "only"}}, Active: "only"}
		if _, err := c.Next(); err == nil {
			t.Fatal("에러를 기대했으나 nil")
		}
	})
	t.Run("활성 프로필 다음 순서로 순환", func(t *testing.T) {
		c := &Config{
			Profiles: []Profile{{Name: "a"}, {Name: "b"}, {Name: "c"}},
			Active:   "c",
		}
		got, err := c.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got != "a" {
			t.Fatalf("got %q, want %q", got, "a")
		}
	})
	t.Run("활성 프로필이 목록에 없으면 첫 프로필", func(t *testing.T) {
		c := &Config{Profiles: []Profile{{Name: "a"}, {Name: "b"}}, Active: "deleted"}
		got, err := c.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got != "a" {
			t.Fatalf("got %q, want %q", got, "a")
		}
	})
}

func TestUpsert(t *testing.T) {
	c := &Config{}
	c.Upsert(Profile{Name: "work", Email: "old"})
	c.Upsert(Profile{Name: "work", Email: "new"})
	if len(c.Profiles) != 1 {
		t.Fatalf("프로필 수 = %d, want 1", len(c.Profiles))
	}
	if c.Profiles[0].Email != "new" {
		t.Fatalf("Email = %q, want %q", c.Profiles[0].Email, "new")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	missing, err := Load(path)
	if err != nil {
		t.Fatalf("없는 파일 Load 실패: %v", err)
	}
	if len(missing.Profiles) != 0 {
		t.Fatal("없는 파일은 빈 Config여야 함")
	}

	c := &Config{Active: "work", Profiles: []Profile{{Name: "work", Email: "fake@example.com"}}}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Active != "work" || len(loaded.Profiles) != 1 || loaded.Profiles[0].Name != "work" {
		t.Fatalf("round-trip 불일치: %+v", loaded)
	}
}

func TestRemove(t *testing.T) {
	c := &Config{Profiles: []Profile{{Name: "a"}, {Name: "b"}}}
	if !c.Remove("a") {
		t.Fatal("Remove(a) = false, want true")
	}
	if c.Find("a") != nil {
		t.Fatal("a가 남아 있음")
	}
	if c.Remove("ghost") {
		t.Fatal("Remove(ghost) = true, want false")
	}
}
```

- [ ] **Step 2: 실패 확인**

Run: `go test ./internal/config/`
Expected: FAIL (컴파일 에러 — Config 미정의)

- [ ] **Step 3: 구현**

```go
// Package config는 claude-switch의 프로필 메타데이터를 관리한다.
// 비밀값(토큰)은 절대 이 파일에 저장하지 않는다 — 키체인 전용.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Profile은 등록된 Claude Code 계정 하나의 메타데이터다.
type Profile struct {
	Name string `json:"name"`
	// Email은 표시용 계정 이메일.
	Email string `json:"email,omitempty"`
	// OAuthAccount는 ~/.claude.json의 oauthAccount 필드 원문 (비밀값 아님 — 계정 식별 메타데이터).
	OAuthAccount json.RawMessage `json:"oauthAccount,omitempty"`
}

type Config struct {
	// Active는 현재 키체인에 들어 있는 프로필명.
	Active string `json:"active,omitempty"`
	// ClaudeKeychainAcct는 Claude Code 키체인 항목의 account 속성(보통 macOS 사용자명).
	ClaudeKeychainAcct string    `json:"claudeKeychainAccount,omitempty"`
	Profiles           []Profile `json:"profiles"`
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "claude-switch", "config.json"), nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config 파싱 실패 (%s): %w", path, err)
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Config) Find(name string) *Profile {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i]
		}
	}
	return nil
}

// Upsert는 프로필을 추가하거나 같은 이름의 기존 프로필을 교체한다.
func (c *Config) Upsert(p Profile) {
	if existing := c.Find(p.Name); existing != nil {
		*existing = p
		return
	}
	c.Profiles = append(c.Profiles, p)
}

// Remove는 프로필을 삭제하고 삭제 여부를 반환한다.
func (c *Config) Remove(name string) bool {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			c.Profiles = append(c.Profiles[:i], c.Profiles[i+1:]...)
			return true
		}
	}
	return false
}

// Next는 활성 프로필 다음 순서의 프로필명을 반환한다 (순환).
func (c *Config) Next() (string, error) {
	if len(c.Profiles) < 2 {
		return "", errors.New("전환할 프로필이 2개 이상 필요합니다. claude-switch save <name>으로 계정을 등록하세요")
	}
	for i := range c.Profiles {
		if c.Profiles[i].Name == c.Active {
			return c.Profiles[(i+1)%len(c.Profiles)].Name, nil
		}
	}
	return c.Profiles[0].Name, nil
}
```

- [ ] **Step 4: 통과 확인**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 5: 커밋**

```bash
git add internal/config && git commit -m "feat: 프로필 메타데이터 config 패키지"
```

---

### Task 3: claudejson 패키지 (TDD)

**Files:**
- Create: `internal/claudejson/claudejson.go`
- Test: `internal/claudejson/claudejson_test.go`

- [ ] **Step 1: 실패하는 테스트 작성**

```go
package claudejson

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadOAuthAccount(t *testing.T) {
	path := writeFixture(t, `{"oauthAccount":{"emailAddress":"fake@example.com"},"other":1}`)
	got, err := ReadOAuthAccount(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"emailAddress":"fake@example.com"}` {
		t.Fatalf("got %s", got)
	}
}

func TestWriteOAuthAccountPreservesOtherFields(t *testing.T) {
	path := writeFixture(t, `{"oauthAccount":{"emailAddress":"old@example.com"},"numStartups":42,"projects":{"/tmp":{"history":[]}}}`)

	if err := WriteOAuthAccount(path, json.RawMessage(`{"emailAddress":"new@example.com"}`)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["oauthAccount"]) != `{"emailAddress":"new@example.com"}` {
		t.Fatalf("oauthAccount = %s", doc["oauthAccount"])
	}
	if string(doc["numStartups"]) != "42" {
		t.Fatalf("numStartups 보존 실패: %s", doc["numStartups"])
	}
	if _, ok := doc["projects"]; !ok {
		t.Fatal("projects 필드 유실")
	}
}

func TestEmail(t *testing.T) {
	if got := Email(json.RawMessage(`{"emailAddress":"a@example.com"}`)); got != "a@example.com" {
		t.Fatalf("got %q", got)
	}
	if got := Email(json.RawMessage(`{"email":"b@example.com"}`)); got != "b@example.com" {
		t.Fatalf("got %q", got)
	}
	if got := Email(json.RawMessage(`not-json`)); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
```

- [ ] **Step 2: 실패 확인**

Run: `go test ./internal/claudejson/`
Expected: FAIL (컴파일 에러)

- [ ] **Step 3: 구현**

```go
// Package claudejson은 ~/.claude.json의 oauthAccount 필드를 다른 필드를 보존한 채 읽고 쓴다.
package claudejson

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadOAuthAccount는 oauthAccount 필드를 원문 그대로 반환한다. 필드가 없으면 nil.
func ReadOAuthAccount(path string) (json.RawMessage, error) {
	doc, err := load(path)
	if err != nil {
		return nil, err
	}
	return doc["oauthAccount"], nil
}

// WriteOAuthAccount는 다른 필드를 보존한 채 oauthAccount만 교체한다.
func WriteOAuthAccount(path string, oauthAccount json.RawMessage) error {
	doc, err := load(path)
	if err != nil {
		return err
	}
	if len(oauthAccount) == 0 {
		delete(doc, "oauthAccount")
	} else {
		doc["oauthAccount"] = oauthAccount
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// Email은 oauthAccount JSON에서 이메일을 추출한다. 없으면 빈 문자열.
func Email(oauthAccount json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(oauthAccount, &m) != nil {
		return ""
	}
	for _, key := range []string{"emailAddress", "email"} {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

func load(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s 파싱 실패: %w", path, err)
	}
	return doc, nil
}
```

- [ ] **Step 4: 통과 확인**

Run: `go test ./internal/claudejson/`
Expected: PASS

- [ ] **Step 5: 커밋**

```bash
git add internal/claudejson && git commit -m "feat: ~/.claude.json oauthAccount 조작 패키지"
```

---

### Task 4: keychain 패키지

**Files:**
- Create: `internal/keychain/keychain.go`
- Test: `internal/keychain/keychain_test.go`

security CLI 실행 자체는 통합 영역이라 단위 테스트에서 제외하고, 출력 파싱만 테스트한다.

- [ ] **Step 1: acct 파싱 실패 테스트 작성**

```go
package keychain

import "testing"

const sampleMeta = `keychain: "/Users/fakeuser/Library/Keychains/login.keychain-db"
version: 512
class: "genp"
attributes:
    0x00000007 <blob>="Claude Code-credentials"
    "acct"<blob>="fakeuser"
    "svce"<blob>="Claude Code-credentials"
`

func TestParseAccount(t *testing.T) {
	if got := parseAccount(sampleMeta); got != "fakeuser" {
		t.Fatalf("got %q, want %q", got, "fakeuser")
	}
	if got := parseAccount("no acct line"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
```

- [ ] **Step 2: 실패 확인**

Run: `go test ./internal/keychain/`
Expected: FAIL (컴파일 에러)

- [ ] **Step 3: 구현**

```go
// Package keychain은 macOS security CLI로 키체인 generic password 항목을 다룬다.
// 비밀값은 프로세스 인자/메모리로만 다루고 디스크에 쓰지 않는다.
package keychain

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var ErrNotFound = errors.New("키체인 항목을 찾을 수 없습니다 (keychain item not found)")

// Keychain은 generic password 읽기/쓰기/삭제를 추상화한다. 테스트에서는 fake로 대체한다.
type Keychain interface {
	// Get은 service+account로 비밀값을 조회한다.
	Get(service, account string) (string, error)
	// GetByService는 service만으로 조회해 (비밀값, account 속성)을 반환한다.
	GetByService(service string) (value string, account string, err error)
	Set(service, account, value string) error
	Delete(service, account string) error
}

// SecurityCLI는 /usr/bin/security를 호출하는 실제 구현이다.
type SecurityCLI struct{}

func run(args ...string) (string, error) {
	cmd := exec.Command("/usr/bin/security", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "could not be found") {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("security %s 실패: %v: %s", args[0], err, msg)
	}
	return stdout.String(), nil
}

func (SecurityCLI) Get(service, account string) (string, error) {
	out, err := run("find-generic-password", "-s", service, "-a", account, "-w")
	if err != nil {
		return "", err
	}
	return strings.TrimRight(out, "\n"), nil
}

func (SecurityCLI) GetByService(service string) (string, string, error) {
	meta, err := run("find-generic-password", "-s", service)
	if err != nil {
		return "", "", err
	}
	value, err := run("find-generic-password", "-s", service, "-w")
	if err != nil {
		return "", "", err
	}
	return strings.TrimRight(value, "\n"), parseAccount(meta), nil
}

func (SecurityCLI) Set(service, account, value string) error {
	_, err := run("add-generic-password", "-U", "-s", service, "-a", account, "-w", value)
	return err
}

func (SecurityCLI) Delete(service, account string) error {
	_, err := run("delete-generic-password", "-s", service, "-a", account)
	return err
}

var acctRe = regexp.MustCompile(`"acct"<blob>="((?:[^"\\]|\\.)*)"`)

// parseAccount는 find-generic-password 메타 출력에서 acct 속성을 추출한다.
func parseAccount(metaOutput string) string {
	m := acctRe.FindStringSubmatch(metaOutput)
	if m == nil {
		return ""
	}
	return m[1]
}
```

- [ ] **Step 4: 통과 확인**

Run: `go test ./internal/keychain/`
Expected: PASS

- [ ] **Step 5: 커밋**

```bash
git add internal/keychain && git commit -m "feat: security CLI 키체인 래퍼"
```

---

### Task 5: app 패키지 — 핵심 전환 로직 (TDD)

**Files:**
- Create: `internal/app/app.go`
- Test: `internal/app/app_test.go`

- [ ] **Step 1: fake keychain + 핵심 시나리오 실패 테스트 작성**

```go
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
```

- [ ] **Step 2: 실패 확인**

Run: `go test ./internal/app/`
Expected: FAIL (컴파일 에러 — App 미정의)

- [ ] **Step 3: 구현**

```go
// Package app은 claude-switch의 핵심 전환 로직이다.
package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"

	"github.com/YangTaeyoung/claude-switch/internal/claudejson"
	"github.com/YangTaeyoung/claude-switch/internal/config"
	"github.com/YangTaeyoung/claude-switch/internal/keychain"
)

const (
	// ClaudeService는 Claude Code가 사용하는 키체인 service 이름.
	ClaudeService = "Claude Code-credentials"
	// ProfileService는 claude-switch 프로필 스냅샷의 키체인 service 이름.
	ProfileService = "claude-switch-profile"
)

type App struct {
	KC             keychain.Keychain
	ConfigPath     string
	ClaudeJSONPath string
	Out            io.Writer
	Errw           io.Writer
}

// New는 실제 환경(키체인 + 홈 디렉토리) 기반 App을 만든다.
func New() (*App, error) {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &App{
		KC:             keychain.SecurityCLI{},
		ConfigPath:     cfgPath,
		ClaudeJSONPath: filepath.Join(home, ".claude.json"),
		Out:            os.Stdout,
		Errw:           os.Stderr,
	}, nil
}

// Save는 현재 키체인 자격증명을 프로필로 저장하고 활성 프로필로 표시한다.
func (a *App) Save(name string) error {
	cred, acct, err := a.KC.GetByService(ClaudeService)
	if errors.Is(err, keychain.ErrNotFound) {
		return errors.New("Claude Code 자격증명이 키체인에 없습니다. 먼저 claude를 실행해 /login으로 로그인하세요")
	}
	if err != nil {
		return err
	}

	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}

	p := config.Profile{Name: name}
	if oauth, err := claudejson.ReadOAuthAccount(a.ClaudeJSONPath); err != nil {
		fmt.Fprintf(a.Errw, "경고: %s에서 oauthAccount를 읽지 못했습니다: %v\n", a.ClaudeJSONPath, err)
	} else {
		p.OAuthAccount = oauth
		p.Email = claudejson.Email(oauth)
	}

	if err := a.KC.Set(ProfileService, name, cred); err != nil {
		return err
	}

	cfg.Upsert(p)
	cfg.Active = name
	if acct != "" {
		cfg.ClaudeKeychainAcct = acct
	}
	if err := cfg.Save(a.ConfigPath); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "프로필 %q 저장 완료 (활성: %s)\n", name, displayEmail(p.Email))
	return nil
}

// Use는 sync-back 후 지정 프로필의 자격증명으로 전환한다.
func (a *App) Use(name string) error {
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	p := cfg.Find(name)
	if p == nil {
		return fmt.Errorf("프로필 %q이(가) 없습니다. claude-switch list로 확인하세요", name)
	}

	a.syncBack(cfg)

	if name == cfg.Active {
		fmt.Fprintf(a.Out, "이미 %q 프로필이 활성입니다\n", name)
		return nil
	}

	cred, err := a.KC.Get(ProfileService, name)
	if err != nil {
		return fmt.Errorf("프로필 %q 자격증명 읽기 실패: %w", name, err)
	}

	acct := cfg.ClaudeKeychainAcct
	if acct == "" {
		u, err := user.Current()
		if err != nil {
			return err
		}
		acct = u.Username
	}
	if err := a.KC.Set(ClaudeService, acct, cred); err != nil {
		return err
	}

	if len(p.OAuthAccount) > 0 {
		if err := claudejson.WriteOAuthAccount(a.ClaudeJSONPath, p.OAuthAccount); err != nil {
			fmt.Fprintf(a.Errw, "경고: %s oauthAccount 교체 실패: %v\n", a.ClaudeJSONPath, err)
		}
	}

	cfg.Active = name
	if err := cfg.Save(a.ConfigPath); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "%q 프로필로 전환했습니다 (%s)\n", name, displayEmail(p.Email))
	fmt.Fprintln(a.Out, "새로 시작하는 claude 세션부터 적용됩니다. 실행 중인 세션은 재시작하세요.")
	return nil
}

// Next는 등록 순서상 다음 프로필로 순환 전환한다.
func (a *App) Next() error {
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	target, err := cfg.Next()
	if err != nil {
		return err
	}
	return a.Use(target)
}

// Delete는 프로필을 삭제한다. 활성 프로필은 삭제할 수 없다.
func (a *App) Delete(name string) error {
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	if cfg.Find(name) == nil {
		return fmt.Errorf("프로필 %q이(가) 없습니다", name)
	}
	if name == cfg.Active {
		return fmt.Errorf("활성 프로필 %q은(는) 삭제할 수 없습니다. 다른 프로필로 전환 후 삭제하세요", name)
	}
	if err := a.KC.Delete(ProfileService, name); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		return err
	}
	cfg.Remove(name)
	if err := cfg.Save(a.ConfigPath); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "프로필 %q 삭제 완료\n", name)
	return nil
}

// List는 프로필 목록을 출력한다.
func (a *App) List() error {
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(a.Out, "등록된 프로필이 없습니다. claude /login 후 claude-switch save <name>으로 등록하세요")
		return nil
	}
	for _, p := range cfg.Profiles {
		marker := "  "
		if p.Name == cfg.Active {
			marker = "* "
		}
		fmt.Fprintf(a.Out, "%s%s\t%s\n", marker, p.Name, displayEmail(p.Email))
	}
	return nil
}

// syncBack은 현재 키체인 자격증명을 활성 프로필 스냅샷에 반영한다 (토큰 회전 대응).
func (a *App) syncBack(cfg *config.Config) {
	if cfg.Active == "" || cfg.Find(cfg.Active) == nil {
		return
	}
	cred, _, err := a.KC.GetByService(ClaudeService)
	if err != nil {
		fmt.Fprintf(a.Errw, "경고: 현재 자격증명 sync-back 실패: %v\n", err)
		return
	}
	if err := a.KC.Set(ProfileService, cfg.Active, cred); err != nil {
		fmt.Fprintf(a.Errw, "경고: 프로필 %q sync-back 실패: %v\n", cfg.Active, err)
	}
}

func displayEmail(email string) string {
	if email == "" {
		return "이메일 미상"
	}
	return email
}
```

- [ ] **Step 4: 통과 확인**

Run: `go test ./internal/app/`
Expected: PASS (전체 7개 테스트)

- [ ] **Step 5: 커밋**

```bash
git add internal/app && git commit -m "feat: save/use/next/list/delete 전환 로직"
```

---

### Task 6: limit 패키지 + Status (베스트 에포트, 실측 검증 포함)

**Files:**
- Create: `internal/limit/limit.go`
- Modify: `internal/app/app.go` (Status 메서드 추가)
- Test: `internal/app/app_test.go` (Status는 limit 비활성 경로만 단위 테스트)

스펙상 이 기능은 베스트 에포트다. 엔드포인트·헤더는 학습 데이터 기반 추정이므로 **Step 4에서 반드시 실측 검증**하고, 동작하지 않으면 "확인 불가" 표시로 두는 것까지가 이 태스크의 완료 조건이다.

- [ ] **Step 1: limit 패키지 구현**

```go
// Package limit은 각 계정의 리밋 상태를 베스트 에포트로 조회한다.
// 실패는 정상 경로다 — 호출부는 Err가 있으면 "확인 불가"로 표시한다.
package limit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// probeURL은 OAuth 토큰으로 인증되는 저비용 엔드포인트.
// anthropic-ratelimit-unified-* 응답 헤더를 기대한다 (실측 검증 대상).
const probeURL = "https://api.anthropic.com/api/oauth/profile"

type Result struct {
	// Status는 리밋 상태 헤더 값 (예: allowed, allowed_warning, rejected). 미확인이면 빈 문자열.
	Status string
	// ResetsAt은 리밋 리셋 시각. 헤더가 없으면 zero value.
	ResetsAt time.Time
	Err      error
}

// Check는 accessToken으로 인증 요청을 보내 리밋 헤더를 읽는다.
func Check(ctx context.Context, accessToken string) Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return Result{Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return Result{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Result{Err: fmt.Errorf("인증 실패 (HTTP %d) — 토큰이 만료되었을 수 있습니다", resp.StatusCode)}
	}

	r := Result{Status: resp.Header.Get("anthropic-ratelimit-unified-status")}
	if reset := resp.Header.Get("anthropic-ratelimit-unified-reset"); reset != "" {
		if sec, err := strconv.ParseInt(reset, 10, 64); err == nil {
			r.ResetsAt = time.Unix(sec, 0)
		}
	}
	if r.Status == "" {
		r.Err = fmt.Errorf("리밋 헤더 없음 (HTTP %d)", resp.StatusCode)
	}
	return r
}

// AccessToken은 키체인 자격증명 JSON에서 accessToken을 추출한다.
// 최상위 또는 claudeAiOauth 하위에서 찾는다.
func AccessToken(credJSON string) (string, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(credJSON), &doc); err != nil {
		return "", fmt.Errorf("자격증명 JSON 파싱 실패: %w", err)
	}
	if raw, ok := doc["claudeAiOauth"]; ok {
		if err := json.Unmarshal(raw, &doc); err != nil {
			return "", fmt.Errorf("claudeAiOauth 파싱 실패: %w", err)
		}
	}
	var token string
	if raw, ok := doc["accessToken"]; ok {
		if err := json.Unmarshal(raw, &token); err != nil {
			return "", err
		}
	}
	if token == "" {
		return "", fmt.Errorf("accessToken 필드를 찾을 수 없습니다")
	}
	return token, nil
}
```

- [ ] **Step 2: app에 Status 추가**

`internal/app/app.go`에 추가 (import에 `context`, `time`, limit 패키지 추가):

```go
// LimitChecker는 limit.Check를 추상화한다. 테스트에서는 nil로 두어 리밋 조회를 생략한다.
type LimitChecker func(ctx context.Context, accessToken string) limit.Result

// Status는 활성 프로필과 계정별 리밋 상태를 출력한다.
func (a *App) Status(ctx context.Context, check LimitChecker) error {
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(a.Out, "등록된 프로필이 없습니다. claude /login 후 claude-switch save <name>으로 등록하세요")
		return nil
	}
	fmt.Fprintf(a.Out, "활성 프로필: %s\n\n", cfg.Active)
	for _, p := range cfg.Profiles {
		marker := "  "
		if p.Name == cfg.Active {
			marker = "* "
		}
		line := fmt.Sprintf("%s%s\t%s", marker, p.Name, displayEmail(p.Email))
		if check != nil {
			line += "\t" + a.limitLine(ctx, cfg, p.Name, check)
		}
		fmt.Fprintln(a.Out, line)
	}
	return nil
}

// limitLine은 프로필 하나의 리밋 상태 문자열을 만든다. 모든 실패는 "확인 불가"로 수렴한다.
func (a *App) limitLine(ctx context.Context, cfg *config.Config, name string, check LimitChecker) string {
	var cred string
	var err error
	if name == cfg.Active {
		cred, _, err = a.KC.GetByService(ClaudeService)
	} else {
		cred, err = a.KC.Get(ProfileService, name)
	}
	if err != nil {
		return "리밋 확인 불가 (자격증명 없음)"
	}
	token, err := limit.AccessToken(cred)
	if err != nil {
		return "리밋 확인 불가 (" + err.Error() + ")"
	}
	r := check(ctx, token)
	if r.Err != nil {
		return "리밋 확인 불가 (" + r.Err.Error() + ")"
	}
	line := "리밋: " + r.Status
	if !r.ResetsAt.IsZero() {
		line += " (리셋 " + r.ResetsAt.Local().Format("01-02 15:04") + ")"
	}
	return line
}
```

- [ ] **Step 3: Status 단위 테스트 추가 후 전체 테스트**

`internal/app/app_test.go`에 추가:

```go
func TestStatusWithoutLimitChecker(t *testing.T) {
	a, kc := newTestApp(t)
	loginAs(t, a, kc, "acct-a@example.com", "fake-cred-A")
	if err := a.Save("work"); err != nil {
		t.Fatal(err)
	}
	out := a.Out.(*bytes.Buffer)
	out.Reset()
	if err := a.Status(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "work") {
		t.Fatalf("출력에 프로필명이 없음: %q", out.String())
	}
}
```

(테스트 파일 import에 `context` 추가)

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: 실측 검증 (필수)**

빌드 후 실제 로그인된 계정으로 status를 실행해 리밋 헤더가 실제로 오는지 확인:

```bash
go build -o bin/claude-switch . && ./bin/claude-switch save probe-test && ./bin/claude-switch status
```

- 헤더가 오면: 그대로 완료
- `리밋 확인 불가 (리밋 헤더 없음...)` 이면: probeURL을 messages 엔드포인트 최소 요청 등으로 바꿔 1회 재시도해보고, 그래도 안 되면 **"확인 불가" 동작을 정식 결과로 받아들이고 README에 한계로 기록** (스펙상 베스트 에포트)
- 검증에 사용한 `probe-test` 프로필은 `./bin/claude-switch use probe-test` 상태이므로 실제 프로필명으로 다시 save하거나 삭제

- [ ] **Step 5: 커밋**

```bash
git add internal/limit internal/app && git commit -m "feat: status 명령 + 리밋 상태 베스트 에포트 조회"
```

---

### Task 7: main.go 디스패치 + README

**Files:**
- Create: `main.go`
- Modify: `README.md`

- [ ] **Step 1: main.go 작성**

```go
// claude-switch는 macOS 키체인의 Claude Code 자격증명을 프로필 단위로 교체해
// 구독 계정을 전환하는 CLI다.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/YangTaeyoung/claude-switch/internal/app"
	"github.com/YangTaeyoung/claude-switch/internal/limit"
)

const usage = `claude-switch — Claude Code 구독 계정 전환 도구

사용법:
  claude-switch save <name>    현재 로그인된 계정을 프로필로 저장
  claude-switch use <name>     지정 프로필로 전환
  claude-switch next           다음 프로필로 순환 전환
  claude-switch list           프로필 목록
  claude-switch status         활성 프로필 + 계정별 리밋 상태
  claude-switch delete <name>  프로필 삭제

계정 등록 (계정마다 1회):
  claude 실행 → /login 으로 로그인 → claude-switch save <name>
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "오류:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}

	a, err := app.New()
	if err != nil {
		return err
	}

	cmd, rest := args[0], args[1:]
	name := func() (string, error) {
		if len(rest) != 1 {
			return "", fmt.Errorf("%s 명령에는 프로필 이름이 하나 필요합니다", cmd)
		}
		return rest[0], nil
	}

	switch cmd {
	case "save":
		n, err := name()
		if err != nil {
			return err
		}
		return a.Save(n)
	case "use":
		n, err := name()
		if err != nil {
			return err
		}
		return a.Use(n)
	case "next":
		return a.Next()
	case "list":
		return a.List()
	case "status":
		return a.Status(context.Background(), limit.Check)
	case "delete":
		n, err := name()
		if err != nil {
			return err
		}
		return a.Delete(n)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	default:
		fmt.Print(usage)
		return fmt.Errorf("알 수 없는 명령: %s", cmd)
	}
}
```

- [ ] **Step 2: 빌드 + 전체 테스트**

Run: `go build -o bin/claude-switch . && go test ./... && go vet ./...`
Expected: 빌드 성공, 전체 PASS

- [ ] **Step 3: README 완성**

설치(`go install` 또는 `go build`), 사용 흐름(등록 → next), 키체인 허용 프롬프트 안내, 동작 원리(키체인 스왑 + sync-back), 한계(실행 중 세션은 재시작 필요, 리밋 표시는 베스트 에포트, `add-generic-password -w` 인자가 실행 순간 프로세스 목록에 노출될 수 있음 — 로컬 단일 사용자 전제) 작성.

- [ ] **Step 4: 커밋**

```bash
git add main.go README.md && git commit -m "feat: CLI 디스패치 및 README"
```

---

### Task 8: 수동 E2E + 민감 정보 감사 (최종)

- [ ] **Step 1: 수동 E2E**

실제 키체인으로 동작 확인 (현재 로그인된 계정 1개만으로도 save/list/status/use 자기 자신 전환 확인 가능):

```bash
./bin/claude-switch save main-account
./bin/claude-switch list      # * main-account 표시
./bin/claude-switch status
./bin/claude-switch use main-account   # "이미 활성" 출력 확인
claude --help > /dev/null && echo OK   # claude가 여전히 정상 인증되는지
```

- [ ] **Step 2: 민감 정보 감사 (사용자 요구사항)**

저장소 전체(추적 파일 + git 히스토리)에서 토큰·이메일·실명 패턴 검사:

```bash
cd ~/dev/personal/claude-switch
# 작업 트리: 실토큰 패턴(sk-ant-, oat01-), JWT 형태, 실제 이메일
grep -rInE 'sk-ant-|oat01|eyJ[A-Za-z0-9_-]{20,}|[A-Za-z0-9._%+-]+@(gmail|naver|kakao)' --exclude-dir=.git . || echo "작업 트리 깨끗함"
# git 히스토리 전체
git log -p --all | grep -nE 'sk-ant-|oat01|eyJ[A-Za-z0-9_-]{20,}' || echo "히스토리 깨끗함"
# 테스트 픽스처가 가짜 값(@example.com, fake-)만 쓰는지 육안 확인
grep -rIn '@' --include='*_test.go' .
```

Expected: 모두 깨끗함. 발견 시 해당 커밋 수정(amend/rebase) 후 재감사.

- [ ] **Step 3: 최종 커밋 및 결과 보고**

```bash
git status   # clean 확인
```

감사 결과를 사용자에게 보고한다.
