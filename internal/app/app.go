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
