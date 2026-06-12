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
