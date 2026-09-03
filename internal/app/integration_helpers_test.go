package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gongshuiwen/gwm/internal/gitcli"
	"github.com/gongshuiwen/gwm/internal/hooks"
)

type temporaryRepository struct {
	t      *testing.T
	base   string
	root   string
	runner gitcli.Runner
}

func newTemporaryRepository(t *testing.T) *temporaryRepository {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "repository")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	environment := append(gitcli.CleanEnv(os.Environ()),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	runner := &gitcli.CommandRunner{Path: "git", Env: environment}
	repository := &temporaryRepository{t: t, base: base, root: root, runner: runner}
	repository.git("-C", root, "init", "-b", "main")
	repository.git("-C", root, "config", "user.name", "GWM Test")
	repository.git("-C", root, "config", "user.email", "gwm-test@example.invalid")
	tracked := filepath.Join(root, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("initial\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository.git("-C", root, "add", "tracked.txt")
	repository.git("-C", root, "commit", "-m", "initial")
	return repository
}

func (r *temporaryRepository) git(args ...string) gitcli.Result {
	r.t.Helper()
	result := r.runner.Run(r.t.Context(), args...)
	if !result.Success() {
		r.t.Fatalf("git %v failed: %s", args, strings.TrimSpace(string(result.Stderr)))
	}
	return result
}

func (r *temporaryRepository) run(args ...string) (int, string, string) {
	r.t.Helper()
	return r.runWith(r.runner, hooks.NewExecutor(), args...)
}

func (r *temporaryRepository) runWith(runner gitcli.Runner, executor hooks.Executor, args ...string) (int, string, string) {
	r.t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	application := &App{
		git:      runner,
		hooks:    executor,
		stdout:   &stdout,
		stderr:   &stderr,
		startDir: r.root,
	}
	exitCode := application.Run(r.t.Context(), args)
	return exitCode, stdout.String(), stderr.String()
}

func (r *temporaryRepository) initGWM() {
	r.t.Helper()
	exitCode, _, stderr := r.run("init")
	if exitCode != 0 {
		r.t.Fatalf("gwm init exited %d: %s", exitCode, stderr)
	}
}

func writeHook(t *testing.T, directory, contents string) string {
	t.Helper()
	path := filepath.Join(directory, strings.ReplaceAll(t.Name(), "/", "-")+"-hook")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

type recordingExecutor struct {
	events   []string
	paths    []string
	payloads []hooks.Payload
}

func (e *recordingExecutor) Run(_ context.Context, path string, _ string, payload hooks.Payload, _, _ io.Writer) error {
	e.events = append(e.events, payload.Event)
	e.paths = append(e.paths, path)
	e.payloads = append(e.payloads, payload)
	return nil
}
