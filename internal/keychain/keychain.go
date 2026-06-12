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
