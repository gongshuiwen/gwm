package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gwm/internal/gitcli"
	"gwm/internal/hooks"
)

const usage = `usage:
  gwm [-C <repository>] init
  gwm [-C <repository>] list
  gwm [-C <repository>] add <path> [-b <new-branch> | --detach] [--from <commit-ish>] [--description <text>] [--protected]
  gwm [-C <repository>] meta <path> [--description <text>] [--protected <true|false>]
  gwm [-C <repository>] remove <path> [--force]`

type App struct {
	Git      gitcli.Runner
	Hooks    hooks.Executor
	Out      io.Writer
	Err      io.Writer
	StartDir string
}

func New() *App {
	startDir, err := os.Getwd()
	if err != nil {
		startDir = "."
	}
	return &App{
		Git:      gitcli.New(),
		Hooks:    hooks.NewExecutor(),
		Out:      os.Stdout,
		Err:      os.Stderr,
		StartDir: startDir,
	}
}

func (a *App) Run(ctx context.Context, args []string) int {
	if a.Git == nil || a.Hooks == nil || a.Out == nil || a.Err == nil {
		return a.fail(fmt.Errorf("application dependencies are not configured"))
	}
	start := a.StartDir
	if start == "" {
		start = "."
	}
	if len(args) >= 1 && args[0] == "-C" {
		if len(args) < 2 || args[1] == "" {
			return a.usageError("-C requires a repository path")
		}
		if !validText(args[1]) {
			return a.usageError("repository path must be valid UTF-8 without NUL")
		}
		if filepath.IsAbs(args[1]) {
			start = args[1]
		} else {
			start = filepath.Join(start, args[1])
		}
		args = args[2:]
	}
	if len(args) == 0 {
		return a.usageError("missing command")
	}
	if strings.HasPrefix(args[0], "-") {
		return a.usageError("options must follow a command, except for -C")
	}
	var execute func(*Repository) int
	switch args[0] {
	case "init":
		if len(args) != 1 {
			return a.usageError("init accepts no arguments")
		}
		execute = func(repository *Repository) int { return a.init(ctx, repository) }
	case "list":
		if len(args) != 1 {
			return a.usageError("list accepts no arguments")
		}
		execute = func(repository *Repository) int { return a.list(ctx, repository) }
	case "add":
		options, err := parseAdd(args[1:])
		if err != nil {
			return a.usageError(err.Error())
		}
		execute = func(repository *Repository) int { return a.add(ctx, repository, options) }
	case "meta":
		options, err := parseMeta(args[1:])
		if err != nil {
			return a.usageError(err.Error())
		}
		execute = func(repository *Repository) int { return a.metadata(ctx, repository, options) }
	case "remove":
		options, err := parseRemove(args[1:])
		if err != nil {
			return a.usageError(err.Error())
		}
		execute = func(repository *Repository) int { return a.remove(ctx, repository, options) }
	default:
		return a.usageError(fmt.Sprintf("unknown command %q", args[0]))
	}

	repository, err := Discover(ctx, a.Git, start)
	if err != nil {
		return a.fail(err)
	}
	return execute(repository)
}

func (a *App) usageError(message string) int {
	fmt.Fprintf(a.Err, "gwm: %s\n%s\n", message, usage)
	return 1
}

func (a *App) fail(err error) int {
	fmt.Fprintf(a.Err, "gwm: %v\n", err)
	return 1
}

func (a *App) partial(operation string, errs ...error) int {
	fmt.Fprintf(a.Err, "gwm: partial success: git worktree %s completed; GWM follow-up failed", operation)
	for _, err := range errs {
		if err != nil {
			fmt.Fprintf(a.Err, ": %v", err)
		}
	}
	fmt.Fprintln(a.Err)
	return 2
}

func validText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}
