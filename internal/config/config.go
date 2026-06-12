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
	// Language는 표시 언어 ("en"/"ko"). 빈 값은 en.
	Language string `json:"language,omitempty"`
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
		return nil, fmt.Errorf("failed to parse config (%s): %w", path, err)
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
		return "", errors.New("need at least 2 profiles to switch. Register accounts with: claude-switch save <name>")
	}
	for i := range c.Profiles {
		if c.Profiles[i].Name == c.Active {
			return c.Profiles[(i+1)%len(c.Profiles)].Name, nil
		}
	}
	return c.Profiles[0].Name, nil
}
