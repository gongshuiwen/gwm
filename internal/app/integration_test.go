package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gwm/internal/gitcli"
	"gwm/internal/hooks"
	"gwm/internal/meta"
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
	result := r.runner.Run(context.Background(), args...)
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
		Git:      runner,
		Hooks:    executor,
		Out:      &stdout,
		Err:      &stderr,
		StartDir: r.root,
	}
	exitCode := application.Run(context.Background(), args)
	return exitCode, stdout.String(), stderr.String()
}

func (r *temporaryRepository) initGWM() {
	r.t.Helper()
	exitCode, _, stderr := r.run("init")
	if exitCode != 0 {
		r.t.Fatalf("gwm init exited %d: %s", exitCode, stderr)
	}
}

func TestMetadataLifecycleAndNativeBoundaries(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()

	target := filepath.Join(repository.base, "linked 空间")
	exitCode, _, stderr := repository.run("add", target, "-b", "feature/topic", "--from", "HEAD", "--description", "line\nnext", "--protected")
	if exitCode != 0 {
		t.Fatalf("gwm add exited %d: %s", exitCode, stderr)
	}
	read, err := meta.Read(context.Background(), repository.runner, target)
	if err != nil || read.Invalid != nil || read.Value.Description == nil || *read.Value.Description != "line\nnext" || !read.Value.Protected {
		t.Fatalf("metadata after add = %#v, error = %v", read, err)
	}

	exitCode, stdout, stderr := repository.run("list")
	if exitCode != 0 {
		t.Fatalf("gwm list exited %d: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "feature/topic") || !strings.Contains(stdout, `line\nnext`) || strings.Contains(stdout, "line\nnext") {
		t.Fatalf("list did not escape one worktree per line:\n%s", stdout)
	}

	exitCode, _, _ = repository.run("remove", target)
	if exitCode != 1 {
		t.Fatalf("protected remove exit = %d, want 1", exitCode)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("protected worktree was removed: %v", err)
	}

	exitCode, _, stderr = repository.run("meta", target, "--description", "", "--protected", "false")
	if exitCode != 0 {
		t.Fatalf("gwm meta exited %d: %s", exitCode, stderr)
	}
	read, err = meta.Read(context.Background(), repository.runner, target)
	if err != nil || read.Value.Description != nil || read.Value.Protected {
		t.Fatalf("metadata after edit = %#v, error = %v", read, err)
	}

	exitCode, _, stderr = repository.run("remove", target)
	if exitCode != 0 {
		t.Fatalf("gwm remove exited %d: %s", exitCode, stderr)
	}
	branchResult := repository.runner.Run(context.Background(), "-C", repository.root, "show-ref", "--verify", "refs/heads/feature/topic")
	if !branchResult.Success() {
		t.Fatal("remove deleted the worktree branch")
	}

	exitCode, _, _ = repository.run("remove", repository.root)
	if exitCode != 1 {
		t.Fatalf("main worktree remove exit = %d, want 1", exitCode)
	}
}

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
		result := repository.runner.Run(context.Background(), "-C", repository.root, "config", "--local", "--get", "extensions.worktreeConfig")
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
		Git:      repository.runner,
		Hooks:    hooks.NewExecutor(),
		Out:      &stdout,
		Err:      &stderr,
		StartDir: repository.base,
	}
	exitCode := application.Run(context.Background(), []string{"-C", filepath.Base(repository.root), "init"})
	if exitCode != 0 {
		t.Fatalf("relative -C init exited %d: %s", exitCode, stderr.String())
	}
	exitCode = application.Run(context.Background(), []string{"-C", filepath.Base(repository.root), "add", "../relative-tree", "-b", "relative-tree"})
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
		Git:      repository.runner,
		Hooks:    hooks.NewExecutor(),
		Out:      &stdout,
		Err:      &linkedStderr,
		StartDir: target,
	}
	exitCode = application.Run(context.Background(), []string{"remove", "."})
	if exitCode != 0 {
		t.Fatalf("remove from linked invocation root exited %d: %s", exitCode, linkedStderr.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("linked invocation worktree still exists: %v", err)
	}
	branch := repository.runner.Run(context.Background(), "-C", repository.root, "show-ref", "--verify", "refs/heads/self-remove")
	if !branch.Success() {
		t.Fatal("self-remove deleted its branch")
	}
}

func TestDetachedLockedAndInvalidMetadata(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	target := filepath.Join(repository.base, "detached")
	exitCode, _, stderr := repository.run("add", target, "--detach", "--from", "HEAD")
	if exitCode != 0 {
		t.Fatalf("detached add exited %d: %s", exitCode, stderr)
	}
	exitCode, stdout, stderr := repository.run("list")
	if exitCode != 0 || !lineForPathContains(stdout, target, "-") {
		t.Fatalf("detached list exited %d: %s\n%s", exitCode, stderr, stdout)
	}

	repository.git("-C", repository.root, "worktree", "lock", target)
	exitCode, _, _ = repository.run("remove", target)
	if exitCode != 1 {
		t.Fatalf("locked remove exit = %d, want 1", exitCode)
	}
	repository.git("-C", repository.root, "worktree", "unlock", target)
	repository.git("-C", target, "config", "--worktree", "--replace-all", "gwm.metadata", `{"description":null}`)

	exitCode, stdout, stderr = repository.run("list")
	if exitCode != 0 || !lineForPathContains(stdout, target, "INVALID") {
		t.Fatalf("invalid metadata list exited %d: %s\n%s", exitCode, stderr, stdout)
	}
	exitCode, _, _ = repository.run("meta", target, "--protected", "false")
	if exitCode != 1 {
		t.Fatalf("meta over invalid value exit = %d, want 1", exitCode)
	}
	exitCode, _, _ = repository.run("remove", target, "--force")
	if exitCode != 1 {
		t.Fatalf("remove with invalid value exit = %d, want 1", exitCode)
	}
}

func TestGitFailuresDoNotTriggerCleanupOrPostHook(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	recorder := &recordingExecutor{}
	configuredHook := writeHook(t, repository.base, "#!/bin/sh\nexit 0\n")
	repository.git("-C", repository.root, "config", "--local", "gwm.hook.post-add", configuredHook)
	repository.git("-C", repository.root, "config", "--local", "gwm.hook.post-remove", configuredHook)

	existing := filepath.Join(repository.base, "occupied")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(existing, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	exitCode, _, _ := repository.runWith(repository.runner, recorder, "add", existing, "-b", "will-not-exist")
	if exitCode != 1 {
		t.Fatalf("failed add exit = %d, want 1", exitCode)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "keep" {
		t.Fatalf("failed add cleaned user directory: data=%q err=%v", data, err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("failed add ran post hook: %v", recorder.events)
	}

	target := filepath.Join(repository.base, "dirty")
	exitCode, _, stderr := repository.run("add", target, "-b", "dirty-branch")
	if exitCode != 0 {
		t.Fatalf("add dirty target exited %d: %s", exitCode, stderr)
	}
	if err := os.WriteFile(filepath.Join(target, "tracked.txt"), []byte("modified\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recorder.events = nil
	exitCode, _, _ = repository.runWith(repository.runner, recorder, "remove", target)
	if exitCode != 1 {
		t.Fatalf("native dirty refusal exit = %d, want 1", exitCode)
	}
	if _, err := os.Stat(filepath.Join(target, "tracked.txt")); err != nil {
		t.Fatalf("failed remove recursively cleaned target: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("failed remove ran post hook: %v", recorder.events)
	}
}

func TestHookOrderPayloadAndNativeBypass(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	logPath := filepath.Join(repository.base, "hook.log")
	hookPath := writeHook(t, repository.base, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\t' \"$PWD\" >> %s\ncat >> %s\n", shellQuote(logPath), shellQuote(logPath)))
	for _, event := range []string{hooks.PreAdd, hooks.PostAdd, hooks.PreRemove, hooks.PostRemove} {
		repository.git("-C", repository.root, "config", "--local", "gwm.hook."+event, hookPath)
	}

	target := filepath.Join(repository.base, "hooked")
	exitCode, _, stderr := repository.run("add", target, "-b", "hooked", "--description", "hook metadata")
	if exitCode != 0 {
		t.Fatalf("hooked add exited %d: %s", exitCode, stderr)
	}
	exitCode, _, stderr = repository.run("remove", target)
	if exitCode != 0 {
		t.Fatalf("hooked remove exited %d: %s", exitCode, stderr)
	}
	records := readHookRecords(t, logPath)
	wantEvents := []string{hooks.PreAdd, hooks.PostAdd, hooks.PreRemove, hooks.PostRemove}
	if len(records) != len(wantEvents) {
		t.Fatalf("hook record count = %d, want %d: %#v", len(records), len(wantEvents), records)
	}
	for index, record := range records {
		if record.CWD != repository.root || record.Payload.Event != wantEvents[index] {
			t.Fatalf("record %d = %#v", index, record)
		}
		if record.Payload.SchemaVersion != 1 || record.Payload.CommonDir == "" || record.Payload.WorktreePath != target {
			t.Fatalf("incomplete payload: %#v", record.Payload)
		}
	}
	if records[0].Payload.Head != nil || records[0].Payload.Branch != nil {
		t.Fatalf("pre-add unexpectedly observed target: %#v", records[0].Payload)
	}
	if records[1].Payload.Head == nil || records[1].Payload.Branch == nil || records[1].Payload.Metadata == nil {
		t.Fatalf("post-add did not observe target: %#v", records[1].Payload)
	}
	if records[3].Payload.Metadata == nil || records[3].Payload.Metadata.Description == nil || *records[3].Payload.Metadata.Description != "hook metadata" {
		t.Fatalf("post-remove lost prior metadata: %#v", records[3].Payload)
	}

	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	nativeTarget := filepath.Join(repository.base, "native")
	repository.git("-C", repository.root, "worktree", "add", "-b", "native", nativeTarget)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("native git unexpectedly triggered GWM hook: %q", data)
	}
}

func TestRelativeHookPathResolvesFromMainWorktree(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()

	invocationRoot := filepath.Join(repository.base, "hook-invocation")
	exitCode, _, stderr := repository.run("add", invocationRoot, "-b", "hook-invocation")
	if exitCode != 0 {
		t.Fatalf("setup add exited %d: %s", exitCode, stderr)
	}

	hookDirectory := filepath.Join(repository.root, ".githooks")
	if err := os.Mkdir(hookDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	hookPath := writeHook(t, hookDirectory, "#!/bin/sh\nexit 0\n")
	relativeHookPath, err := filepath.Rel(repository.root, hookPath)
	if err != nil {
		t.Fatal(err)
	}
	repository.git("-C", repository.root, "config", "--local", "gwm.hook.pre-add", relativeHookPath)

	recorder := &recordingExecutor{}
	var stdout bytes.Buffer
	var linkedStderr bytes.Buffer
	application := &App{
		Git:      repository.runner,
		Hooks:    recorder,
		Out:      &stdout,
		Err:      &linkedStderr,
		StartDir: invocationRoot,
	}
	target := filepath.Join(repository.base, "relative-hook-target")
	exitCode = application.Run(context.Background(), []string{"add", target, "--detach", "--from", "HEAD"})
	if exitCode != 0 {
		t.Fatalf("linked invocation add exited %d: %s", exitCode, linkedStderr.String())
	}
	if len(recorder.paths) != 1 || recorder.paths[0] != hookPath {
		t.Fatalf("resolved hook paths = %q, want [%q]", recorder.paths, hookPath)
	}
}

func TestPreAndPostHookFailures(t *testing.T) {
	t.Run("pre blocks git", func(t *testing.T) {
		repository := newTemporaryRepository(t)
		repository.initGWM()
		failedHook := writeHook(t, repository.base, "#!/bin/sh\nexit 7\n")
		repository.git("-C", repository.root, "config", "--local", "gwm.hook.pre-add", failedHook)
		target := filepath.Join(repository.base, "blocked")
		exitCode, _, _ := repository.run("add", target, "-b", "blocked")
		if exitCode != 1 {
			t.Fatalf("pre failure exit = %d, want 1", exitCode)
		}
		worktrees := repository.git("-C", repository.root, "worktree", "list", "--porcelain", "-z")
		if strings.Contains(string(worktrees.Stdout), target) {
			t.Fatal("pre-hook failure did not block git worktree add")
		}
	})

	t.Run("pre-remove blocks git", func(t *testing.T) {
		repository := newTemporaryRepository(t)
		repository.initGWM()
		target := filepath.Join(repository.base, "remove-blocked")
		exitCode, _, stderr := repository.run("add", target, "-b", "remove-blocked")
		if exitCode != 0 {
			t.Fatalf("setup add exited %d: %s", exitCode, stderr)
		}
		failedHook := writeHook(t, repository.base, "#!/bin/sh\nexit 7\n")
		repository.git("-C", repository.root, "config", "--local", "gwm.hook.pre-remove", failedHook)
		exitCode, _, _ = repository.run("remove", target)
		if exitCode != 1 {
			t.Fatalf("pre-remove failure exit = %d, want 1", exitCode)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("pre-remove failure did not block git: %v", err)
		}
	})

	t.Run("post reports partial without rollback", func(t *testing.T) {
		repository := newTemporaryRepository(t)
		repository.initGWM()
		failedHook := writeHook(t, repository.base, "#!/bin/sh\nexit 8\n")
		repository.git("-C", repository.root, "config", "--local", "gwm.hook.post-add", failedHook)
		target := filepath.Join(repository.base, "partial")
		exitCode, _, stderr := repository.run("add", target, "-b", "partial")
		if exitCode != 2 || !strings.Contains(stderr, "git worktree add completed") {
			t.Fatalf("post failure exit = %d, stderr = %q", exitCode, stderr)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("post-hook failure rolled back worktree: %v", err)
		}
		read, err := meta.Read(context.Background(), repository.runner, target)
		if err != nil || read.Invalid != nil || !read.Present {
			t.Fatalf("metadata missing after post-hook failure: %#v, %v", read, err)
		}
	})

	t.Run("post-remove reports partial without rollback", func(t *testing.T) {
		repository := newTemporaryRepository(t)
		repository.initGWM()
		target := filepath.Join(repository.base, "removed-partial")
		exitCode, _, stderr := repository.run("add", target, "-b", "removed-partial")
		if exitCode != 0 {
			t.Fatalf("setup add exited %d: %s", exitCode, stderr)
		}
		failedHook := writeHook(t, repository.base, "#!/bin/sh\nexit 8\n")
		repository.git("-C", repository.root, "config", "--local", "gwm.hook.post-remove", failedHook)
		exitCode, _, stderr = repository.run("remove", target)
		if exitCode != 2 || !strings.Contains(stderr, "git worktree remove completed") {
			t.Fatalf("post-remove failure exit = %d, stderr = %q", exitCode, stderr)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("post-remove failure restored target: %v", err)
		}
	})
}

func TestOversizedDescriptionFailsBeforeGit(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	target := filepath.Join(repository.base, "oversized")
	exitCode, _, stderr := repository.run("add", target, "-b", "oversized", "--description", strings.Repeat("界", 1366))
	if exitCode != 1 || !strings.Contains(stderr, "4096") {
		t.Fatalf("oversized metadata exit = %d, stderr = %q", exitCode, stderr)
	}
	worktrees := repository.git("-C", repository.root, "worktree", "list", "--porcelain", "-z")
	if strings.Contains(string(worktrees.Stdout), target) {
		t.Fatal("invalid requested metadata was checked after git worktree add")
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
	application := &App{Git: gitcli.New(), Hooks: hooks.NewExecutor(), Out: &stdout, Err: &stderr, StartDir: t.TempDir()}
	if exitCode := application.Run(context.Background(), []string{"unknown"}); exitCode != 1 || !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("usage outside repository: exit = %d, stderr = %q", exitCode, stderr.String())
	}
}

func TestMetadataWriteFailureIsPartialAndStillRunsPostAdd(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	recorder := &recordingExecutor{}
	runner := &failingMetadataRunner{inner: repository.runner}
	target := filepath.Join(repository.base, "metadata-partial")
	repository.git("-C", repository.root, "config", "--local", "gwm.hook.post-add", writeHook(t, repository.base, "#!/bin/sh\nexit 0\n"))
	exitCode, _, stderr := repository.runWith(runner, recorder, "add", target, "-b", "metadata-partial")
	if exitCode != 2 || !strings.Contains(stderr, "git worktree add completed") {
		t.Fatalf("metadata failure exit = %d, stderr = %q", exitCode, stderr)
	}
	if len(recorder.events) != 1 || recorder.events[0] != hooks.PostAdd {
		t.Fatalf("post-add was not run after metadata failure: %v", recorder.events)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("metadata failure rolled back worktree: %v", err)
	}
}

func TestHookConfigValidation(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	nonExecutable := filepath.Join(repository.base, "not-executable")
	if err := os.WriteFile(nonExecutable, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository.git("-C", repository.root, "config", "--local", "--add", "gwm.hook.pre-add", nonExecutable)
	target := filepath.Join(repository.base, "invalid-hook")
	exitCode, _, _ := repository.run("add", target, "-b", "invalid-hook")
	if exitCode != 1 {
		t.Fatalf("non-executable hook exit = %d, want 1", exitCode)
	}
	repository.git("-C", repository.root, "config", "--local", "--add", "gwm.hook.pre-add", nonExecutable)
	exitCode, _, _ = repository.run("add", target, "-b", "invalid-hook")
	if exitCode != 1 {
		t.Fatalf("duplicate hook exit = %d, want 1", exitCode)
	}
}

type hookRecord struct {
	CWD     string
	Payload hooks.Payload
}

func readHookRecords(t *testing.T, path string) []hookRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	records := make([]hookRecord, 0, len(lines))
	for _, line := range lines {
		cwd, payloadJSON, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("invalid hook record: %q", line)
		}
		var payload hooks.Payload
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			t.Fatalf("invalid hook payload %q: %v", payloadJSON, err)
		}
		records = append(records, hookRecord{CWD: cwd, Payload: payload})
	}
	return records
}

func writeHook(t *testing.T, directory, contents string) string {
	t.Helper()
	path := filepath.Join(directory, strings.ReplaceAll(t.Name(), "/", "-")+"-hook")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func lineForPathContains(output, path, value string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, path) && strings.Contains(line, value) {
			return true
		}
	}
	return false
}

type failingMetadataRunner struct {
	inner gitcli.Runner
}

func (r *failingMetadataRunner) Run(ctx context.Context, args ...string) gitcli.Result {
	joined := strings.Join(args, "\x00")
	if strings.Contains(joined, "config\x00--worktree\x00--replace-all\x00gwm.metadata") {
		return gitcli.Result{ExitCode: 9, Err: errors.New("injected metadata write failure")}
	}
	return r.inner.Run(ctx, args...)
}

type recordingExecutor struct {
	events []string
	paths  []string
}

func (e *recordingExecutor) Run(_ context.Context, path string, _ string, payload hooks.Payload, _, _ io.Writer) error {
	e.events = append(e.events, payload.Event)
	e.paths = append(e.paths, path)
	return nil
}
