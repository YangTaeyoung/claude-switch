package keychain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileStore는 평문 파일 기반 Keychain 구현이다 (Linux/Windows).
// macOS는 Keychain을 쓰지만 Linux/Windows의 Claude Code는 자격증명을
// ~/.claude/.credentials.json(권한 0600)에 평문으로 저장하므로, 그 파일을
// 직접 읽고 쓴다. 프로필 스냅샷은 BaseDir 밑에 service/account별 파일로 둔다.
//
// 비밀값은 파일 내용 그대로(통째로) 다루며 파싱하지 않는다. macOS 구현의
// SecurityCLI와 달리 account 속성 개념이 없으므로 GetByService는 빈 account를 반환한다.
type FileStore struct {
	// ClaudeService는 Claude Code 자격증명 파일에 매핑되는 service 이름이다(app.ClaudeService).
	ClaudeService string
	// ClaudeCredPath는 Claude Code 자격증명 파일 경로다(보통 ~/.claude/.credentials.json).
	ClaudeCredPath string
	// BaseDir는 프로필 스냅샷을 저장하는 루트 디렉토리다.
	BaseDir string
}

// path는 (service, account)에 해당하는 파일 경로를 반환한다.
// ClaudeService는 고정 경로로, 나머지는 BaseDir/<service>/<account>로 매핑한다.
func (f FileStore) path(service, account string) (string, error) {
	if service == f.ClaudeService {
		return f.ClaudeCredPath, nil
	}
	s, err := sanitize(service)
	if err != nil {
		return "", fmt.Errorf("invalid service: %w", err)
	}
	a, err := sanitize(account)
	if err != nil {
		return "", fmt.Errorf("invalid account: %w", err)
	}
	return filepath.Join(f.BaseDir, s, a), nil
}

func (f FileStore) Get(service, account string) (string, error) {
	p, err := f.path(service, account)
	if err != nil {
		return "", err
	}
	return readSecret(p)
}

func (f FileStore) GetByService(service string) (string, string, error) {
	// 파일 모델은 account가 경로의 일부이므로 service 단독 조회는 Claude 자격증명에만 의미가 있다.
	if service != f.ClaudeService {
		return "", "", fmt.Errorf("GetByService is unsupported for service %q in the file store", service)
	}
	value, err := readSecret(f.ClaudeCredPath)
	if err != nil {
		return "", "", err
	}
	return value, "", nil
}

func (f FileStore) Set(service, account, value string) error {
	p, err := f.path(service, account)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(value), 0o600)
}

func (f FileStore) Delete(service, account string) error {
	p, err := f.path(service, account)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// readSecret은 파일 내용을 통째로 반환한다. 없으면 ErrNotFound.
func readSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", err
	}
	return string(data), nil
}

// sanitize는 프로필/서비스 이름을 파일명으로 안전하게 만든다.
// 경로 구분자와 제어문자는 '_'로 치환하고, 빈 값이나 상위 경로 참조는 거부한다.
func sanitize(name string) (string, error) {
	if name == "" {
		return "", errors.New("empty name")
	}
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\':
			return '_'
		}
		if r < 0x20 {
			return '_'
		}
		return r
	}, name)
	if cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("invalid name %q", name)
	}
	return cleaned, nil
}
