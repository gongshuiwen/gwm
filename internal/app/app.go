// Package app parses and executes GWM command-line operations.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gongshuiwen/gwm/internal/gitcli"
	"github.com/gongshuiwen/gwm/internal/hooks"
)

// version is injected for release builds with go build -ldflags -X.
var version string

const usage = `usage:
  gwm [-C <repository>] --help
  gwm [-C <repository>] --version
  gwm [-C <repository>] <command> --help
  gwm [-C <repository>] init
  gwm [-C <repository>] list
  gwm [-C <repository>] add <path> [-b <new-branch> | --detach] [--from <commit-ish>] [--description <text>] [--protected]
  gwm [-C <repository>] meta <path> [--description <text>] [--protected <true|false>]
  gwm [-C <repository>] remove <path> [--force]`

const rootHelp = `GWM is a thin local wrapper around git worktree.

` + usage + `

commands:
  init    Enable worktree-specific Git config
  list    List worktrees and GWM metadata
  add     Add a worktree and write GWM metadata
  meta    Show or update worktree metadata
  remove  Remove an unprotected linked worktree

options:
  -C <repository>  Run as if GWM was started in this repository
  --help           Show help
  --version        Show version`

// App owns the process boundaries and streams used by the CLI.
type App struct {
	git      gitcli.Runner
	hooks    hooks.Executor
	stdout   io.Writer
	stderr   io.Writer
	startDir string
}

// New creates an App backed by the system Git executable and process streams.
func New() *App {
	startDir, err := os.Getwd()
	if err != nil {
		startDir = "."
	}
	return &App{
		git:      gitcli.New(),
		hooks:    hooks.NewExecutor(),
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		startDir: startDir,
	}
}

// Run executes one CLI invocation and returns its process exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	if a == nil || a.stderr == nil {
		return 1
	}
	if a.git == nil || a.hooks == nil || a.stdout == nil {
		return a.fail(errors.New("application dependencies are not configured"))
	}
	start := a.startDir
	if start == "" {
		start = "."
	}
	if len(args) > 0 && args[0] == "-C" {
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
	if len(args) == 1 {
		switch args[0] {
		case "--help":
			fmt.Fprintln(a.stdout, rootHelp)
			return 0
		case "--version":
			fmt.Fprintf(a.stdout, "gwm %s\n", currentVersion())
			return 0
		}
	}
	if len(args) == 2 && args[1] == "--help" {
		if help, ok := commandHelp(args[0]); ok {
			fmt.Fprintln(a.stdout, help)
			return 0
		}
	}
	if strings.HasPrefix(args[0], "-") {
		return a.usageError("options must follow a command, except for -C")
	}
	var execute func(*repositoryContext) int
	switch args[0] {
	case "init":
		if len(args) != 1 {
			return a.usageError("init accepts no arguments")
		}
		execute = func(repository *repositoryContext) int { return a.init(ctx, repository) }
	case "list":
		if len(args) != 1 {
			return a.usageError("list accepts no arguments")
		}
		execute = func(repository *repositoryContext) int { return a.list(ctx, repository) }
	case "add":
		options, err := parseAdd(args[1:])
		if err != nil {
			return a.usageError(err.Error())
		}
		execute = func(repository *repositoryContext) int { return a.add(ctx, repository, options) }
	case "meta":
		options, err := parseMeta(args[1:])
		if err != nil {
			return a.usageError(err.Error())
		}
		execute = func(repository *repositoryContext) int { return a.metadata(ctx, repository, options) }
	case "remove":
		options, err := parseRemove(args[1:])
		if err != nil {
			return a.usageError(err.Error())
		}
		execute = func(repository *repositoryContext) int { return a.remove(ctx, repository, options) }
	default:
		return a.usageError(fmt.Sprintf("unknown command %q", args[0]))
	}

	repository, err := discover(ctx, a.git, start)
	if err != nil {
		return a.fail(err)
	}
	return execute(repository)
}

func currentVersion() string {
	if version == "" {
		return "unreleased"
	}
	return version
}

func commandHelp(command string) (string, bool) {
	switch command {
	case "init":
		return `usage: gwm [-C <repository>] init

Enable extensions.worktreeConfig for the repository.`, true
	case "list":
		return `usage: gwm [-C <repository>] list

List Git worktrees with description, protection, and creation metadata.`, true
	case "add":
		return `usage: gwm [-C <repository>] add <path> [-b <new-branch> | --detach] [--from <commit-ish>] [--description <text>] [--protected]

Add a worktree through Git, write metadata, and run configured lifecycle hooks.`, true
	case "meta":
		return `usage: gwm [-C <repository>] meta <path> [--description <text>] [--protected <true|false>]

Show or update editable metadata for a registered worktree.`, true
	case "remove":
		return `usage: gwm [-C <repository>] remove <path> [--force]

Remove an unprotected linked worktree through Git.`, true
	default:
		return "", false
	}
}

func (a *App) usageError(message string) int {
	fmt.Fprintf(a.stderr, "gwm: %s\n%s\n", message, usage)
	return 1
}

func (a *App) fail(err error) int {
	fmt.Fprintf(a.stderr, "gwm: %v\n", err)
	return 1
}

func (a *App) partial(operation string, errs ...error) int {
	fmt.Fprintf(a.stderr, "gwm: partial success: git worktree %s completed; GWM follow-up failed", operation)
	for _, err := range errs {
		if err != nil {
			fmt.Fprintf(a.stderr, ": %v", err)
		}
	}
	fmt.Fprintln(a.stderr)
	return 2
}

func validText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}
