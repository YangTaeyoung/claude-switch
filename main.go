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

const usage = `claude-switch — switch between Claude Code subscription accounts

Usage:
  claude-switch save <name>    Save the currently logged-in account as a profile
  claude-switch use <name>     Switch to a specific profile
  claude-switch next           Cycle to the next profile
  claude-switch list           List profiles (* = active)
  claude-switch status         Per-account usage limits and reset times
  claude-switch delete <name>  Delete a profile

Register each account once:
  run claude → log in with /login → claude-switch save <name>
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
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
			return "", fmt.Errorf("%s requires exactly one profile name", cmd)
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
		return fmt.Errorf("unknown command: %s", cmd)
	}
}
