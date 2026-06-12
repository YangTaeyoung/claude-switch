// claude-switch는 macOS 키체인의 Claude Code 자격증명을 프로필 단위로 교체해
// 구독 계정을 전환하는 CLI다.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/YangTaeyoung/claude-switch/internal/app"
	"github.com/YangTaeyoung/claude-switch/internal/config"
	"github.com/YangTaeyoung/claude-switch/internal/i18n"
	"github.com/YangTaeyoung/claude-switch/internal/limit"
	"github.com/YangTaeyoung/claude-switch/internal/tui"
)

// version은 빌드 시 GoReleaser ldflags로 주입된다.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	initLang()

	if len(args) == 0 {
		if stat, err := os.Stdout.Stat(); err == nil && stat.Mode()&os.ModeCharDevice == 0 {
			fmt.Print(i18n.T("cli.usage")) // 파이프 등 비TTY면 도움말 출력
			return nil
		}
		a, err := app.New()
		if err != nil {
			return err
		}
		return tui.Run(a, version)
	}

	cmd, rest := args[0], args[1:]
	name := func() (string, error) {
		if len(rest) != 1 {
			return "", fmt.Errorf("%s requires exactly one profile name", cmd)
		}
		return rest[0], nil
	}

	switch cmd {
	case "version", "--version", "-v":
		fmt.Println("claude-switch", version)
		return nil
	case "help", "-h", "--help":
		fmt.Print(i18n.T("cli.usage"))
		return nil
	case "lang":
		if len(rest) != 1 || !i18n.Valid(i18n.Lang(rest[0])) {
			return fmt.Errorf("lang requires one of: en, ko")
		}
		return setLang(i18n.Lang(rest[0]))
	}

	a, err := app.New()
	if err != nil {
		return err
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
	case "rename":
		if len(rest) != 2 {
			return fmt.Errorf("rename requires: rename <old> <new>")
		}
		return a.Rename(rest[0], rest[1])
	case "delete":
		n, err := name()
		if err != nil {
			return err
		}
		return a.Delete(n)
	default:
		fmt.Print(i18n.T("cli.usage"))
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

// initLang은 config의 언어 설정을 i18n에 반영한다. 실패하면 영어로 둔다.
func initLang() {
	path, err := config.DefaultPath()
	if err != nil {
		return
	}
	cfg, err := config.Load(path)
	if err != nil {
		return
	}
	i18n.SetLang(i18n.Lang(cfg.Language))
}

// setLang은 표시 언어를 config에 저장한다.
func setLang(l i18n.Lang) error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	cfg.Language = string(l)
	if err := cfg.Save(path); err != nil {
		return err
	}
	i18n.SetLang(l)
	fmt.Printf(i18n.T("cli.langSet"), l)
	return nil
}
