// Package app은 claude-switch의 핵심 전환 로직이다.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"

	"github.com/YangTaeyoung/claude-switch/internal/claudejson"
	"github.com/YangTaeyoung/claude-switch/internal/config"
	"github.com/YangTaeyoung/claude-switch/internal/keychain"
	"github.com/YangTaeyoung/claude-switch/internal/limit"
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
		return errors.New("no Claude Code credentials in the Keychain. Run claude and log in with /login first")
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
		fmt.Fprintf(a.Errw, "warning: failed to read oauthAccount from %s: %v\n", a.ClaudeJSONPath, err)
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
	fmt.Fprintf(a.Out, "Saved profile %q (active: %s)\n", name, displayEmail(p.Email))
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
		return fmt.Errorf("profile %q not found. Check with: claude-switch list", name)
	}

	a.syncBack(cfg)

	if name == cfg.Active {
		fmt.Fprintf(a.Out, "Profile %q is already active\n", name)
		return nil
	}

	cred, err := a.KC.Get(ProfileService, name)
	if err != nil {
		return fmt.Errorf("failed to read credentials for profile %q: %w", name, err)
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
			fmt.Fprintf(a.Errw, "warning: failed to swap oauthAccount in %s: %v\n", a.ClaudeJSONPath, err)
		}
	}

	cfg.Active = name
	if err := cfg.Save(a.ConfigPath); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Switched to profile %q (%s)\n", name, displayEmail(p.Email))
	fmt.Fprintln(a.Out, "Takes effect for new claude sessions. Restart any running session.")
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
		return fmt.Errorf("profile %q not found", name)
	}
	if name == cfg.Active {
		return fmt.Errorf("cannot delete the active profile %q. Switch to another profile first", name)
	}
	if err := a.KC.Delete(ProfileService, name); err != nil && !errors.Is(err, keychain.ErrNotFound) {
		return err
	}
	cfg.Remove(name)
	if err := cfg.Save(a.ConfigPath); err != nil {
		return err
	}
	fmt.Fprintf(a.Out, "Deleted profile %q\n", name)
	return nil
}

// List는 프로필 목록을 출력한다.
func (a *App) List() error {
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(a.Out, "No profiles registered. Log in with claude /login, then run: claude-switch save <name>")
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

// LimitChecker는 limit.Check를 추상화한다. nil이면 리밋 조회를 생략한다.
type LimitChecker func(ctx context.Context, accessToken string) limit.Result

// Config는 현재 설정 파일을 로드한다.
func (a *App) Config() (*config.Config, error) {
	return config.Load(a.ConfigPath)
}

// Usage는 프로필의 자격증명으로 리밋 상태를 조회한다. 실패는 Result.Err로 반환한다.
func (a *App) Usage(ctx context.Context, cfg *config.Config, name string, check LimitChecker) limit.Result {
	var cred string
	var err error
	if name == cfg.Active {
		cred, _, err = a.KC.GetByService(ClaudeService)
	} else {
		cred, err = a.KC.Get(ProfileService, name)
	}
	if err != nil {
		return limit.Result{Err: fmt.Errorf("no credentials: %w", err)}
	}
	token, err := limit.AccessToken(cred)
	if err != nil {
		return limit.Result{Err: err}
	}
	return check(ctx, token)
}

// Status는 활성 프로필과 계정별 리밋 상태를 출력한다.
func (a *App) Status(ctx context.Context, check LimitChecker) error {
	cfg, err := config.Load(a.ConfigPath)
	if err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(a.Out, "No profiles registered. Log in with claude /login, then run: claude-switch save <name>")
		return nil
	}
	fmt.Fprintf(a.Out, "Active profile: %s\n\n", cfg.Active)
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

// limitLine은 프로필 하나의 리밋 상태 문자열을 만든다. 모든 실패는 "limit unavailable"로 수렴한다.
func (a *App) limitLine(ctx context.Context, cfg *config.Config, name string, check LimitChecker) string {
	r := a.Usage(ctx, cfg, name, check)
	if r.Err != nil {
		return "limit unavailable (" + r.Err.Error() + ")"
	}
	line := "limit: " + r.Status
	if w := formatWindow("5h", r.FiveHour); w != "" {
		line += " | " + w
	}
	if w := formatWindow("7d", r.SevenDay); w != "" {
		line += " | " + w
	}
	return line
}

// formatWindow는 "5h 77% (resets 06-12 16:00)" 형태의 윈도우 요약을 만든다.
func formatWindow(label string, w limit.Window) string {
	if w.Utilization < 0 && w.Status == "" {
		return ""
	}
	s := label
	if w.Utilization >= 0 {
		s += fmt.Sprintf(" %.0f%%", w.Utilization*100)
	} else {
		s += " " + w.Status
	}
	if !w.ResetsAt.IsZero() {
		s += " (resets " + w.ResetsAt.Local().Format("01-02 15:04") + ")"
	}
	return s
}

// syncBack은 현재 키체인 자격증명을 활성 프로필 스냅샷에 반영한다 (토큰 회전 대응).
func (a *App) syncBack(cfg *config.Config) {
	if cfg.Active == "" || cfg.Find(cfg.Active) == nil {
		return
	}
	cred, _, err := a.KC.GetByService(ClaudeService)
	if err != nil {
		fmt.Fprintf(a.Errw, "warning: sync-back of current credentials failed: %v\n", err)
		return
	}
	if err := a.KC.Set(ProfileService, cfg.Active, cred); err != nil {
		fmt.Fprintf(a.Errw, "warning: sync-back to profile %q failed: %v\n", cfg.Active, err)
	}
}

func displayEmail(email string) string {
	if email == "" {
		return "unknown email"
	}
	return email
}
