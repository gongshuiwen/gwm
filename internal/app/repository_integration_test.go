package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gongshuiwen/gwm/internal/gitcli"
	"github.com/gongshuiwen/gwm/internal/hooks"
)

func TestInitIsExplicitIdempotentAndRejectsSparsePreflight(t *testing.T) {
	t.Run("explicit and idempotent", func(t *testing.T) {
		repository := newTemporaryRepository(t)
		exitCode, stdout, stderr := repository.run("list")
		if exitCode != 0 || !strings.Contains(stdout, "PATH") {
			t.Fatalf("list before init exited %d: %s\n%s", exitCode, stderr, stdout)
		}
		target := filepath.Join(repository.base, "before-init")
		exitCode, _, stderr = repository.run("add", target, "-b", "before-init")
		if exitCode != 1 || !strings.Contains(stderr, "gwm init") {
			t.Fatalf("add before init exited %d: %q", exitCode, stderr)
		}
		repository.initGWM()
		exitCode, _, stderr = repository.run("init")
		if exitCode != 0 {
			t.Fatalf("second init exited %d: %s", exitCode, stderr)
		}
		values := repository.git("-C", repository.root, "config", "--local", "--get-all", "extensions.worktreeConfig")
		if string(values.Stdout) != "true\n" {
			t.Fatalf("extension values = %q", values.Stdout)
		}
	})

	t.Run("sparse config requires manual migration", func(t *testing.T) {
		repository := newTemporaryRepository(t)
		repository.git("-C", repository.root, "config", "--local", "core.sparseCheckout", "false")
		exitCode, _, stderr := repository.run("init")
		if exitCode != 1 || !strings.Contains(stderr, "core.sparseCheckout") {
			t.Fatalf("sparse preflight exit = %d, stderr = %q", exitCode, stderr)
		}
		result := repository.runner.Run(t.Context(), "-C", repository.root, "config", "--local", "--get", "extensions.worktreeConfig")
		if result.Success() {
			t.Fatal("failed preflight still enabled extensions.worktreeConfig")
		}
	})
}

func TestRelativeRepositoryAndWorktreePaths(t *testing.T) {
	repository := newTemporaryRepository(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	application := &App{
		git:      repository.runner,
		hooks:    hooks.NewExecutor(),
		stdout:   &stdout,
		stderr:   &stderr,
		startDir: repository.base,
	}
	exitCode := application.Run(t.Context(), []string{"-C", filepath.Base(repository.root), "init"})
	if exitCode != 0 {
		t.Fatalf("relative -C init exited %d: %s", exitCode, stderr.String())
	}
	exitCode = application.Run(t.Context(), []string{"-C", filepath.Base(repository.root), "add", "../relative-tree", "-b", "relative-tree"})
	if exitCode != 0 {
		t.Fatalf("relative worktree add exited %d: %s", exitCode, stderr.String())
	}
	worktrees := repository.git("-C", repository.root, "worktree", "list", "--porcelain", "-z")
	if !strings.Contains(string(worktrees.Stdout), filepath.Join(repository.base, "relative-tree")) {
		t.Fatalf("relative target was resolved incorrectly: %q", worktrees.Stdout)
	}
}

func TestRemoveInvocationLinkedWorktree(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	target := filepath.Join(repository.base, "self-remove")
	exitCode, _, stderr := repository.run("add", target, "-b", "self-remove")
	if exitCode != 0 {
		t.Fatalf("setup add exited %d: %s", exitCode, stderr)
	}
	var stdout bytes.Buffer
	var linkedStderr bytes.Buffer
	application := &App{
		git:      repository.runner,
		hooks:    hooks.NewExecutor(),
		stdout:   &stdout,
		stderr:   &linkedStderr,
		startDir: target,
	}
	exitCode = application.Run(t.Context(), []string{"remove", "."})
	if exitCode != 0 {
		t.Fatalf("remove from linked invocation root exited %d: %s", exitCode, linkedStderr.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("linked invocation worktree still exists: %v", err)
	}
	branch := repository.runner.Run(t.Context(), "-C", repository.root, "show-ref", "--verify", "refs/heads/self-remove")
	if !branch.Success() {
		t.Fatal("self-remove deleted its branch")
	}
}

func TestUsageErrorsReturnOne(t *testing.T) {
	repository := newTemporaryRepository(t)
	for _, args := range [][]string{{"unknown"}, {"add"}, {"meta", "tree", "--protected", "yes"}, {"list", "extra"}} {
		exitCode, _, stderr := repository.run(args...)
		if exitCode != 1 || !strings.Contains(stderr, "usage:") {
			t.Fatalf("args %v: exit = %d, stderr = %q", args, exitCode, stderr)
		}
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	application := &App{git: gitcli.New(), hooks: hooks.NewExecutor(), stdout: &stdout, stderr: &stderr, startDir: t.TempDir()}
	if exitCode := application.Run(t.Context(), []string{"unknown"}); exitCode != 1 || !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("usage outside repository: exit = %d, stderr = %q", exitCode, stderr.String())
	}
}
