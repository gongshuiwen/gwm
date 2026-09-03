package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gongshuiwen/gwm/internal/hooks"
	"github.com/gongshuiwen/gwm/internal/meta"
)

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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func TestGitFailuresDoNotTriggerCleanupOrPostHook(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	recorder := &recordingExecutor{}
	configuredHook := writeHook(t, repository.base, "#!/bin/sh\nexit 0\n")
	repository.git("-C", repository.root, "config", "--local", "gwm.hooks.post-add", configuredHook)
	repository.git("-C", repository.root, "config", "--local", "gwm.hooks.post-remove", configuredHook)

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
		repository.git("-C", repository.root, "config", "--local", "gwm.hooks."+event, hookPath)
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
		if record.Payload.SchemaVersion != hooks.SchemaVersion || record.Payload.CommonDir == "" || record.Payload.WorktreePath != target {
			t.Fatalf("incomplete payload: %#v", record.Payload)
		}
	}
	if records[0].Payload.Head != nil || records[0].Payload.Branch != nil {
		t.Fatalf("pre-add unexpectedly observed target: %#v", records[0].Payload)
	}
	if records[0].Payload.Metadata == nil || records[0].Payload.Metadata.CreatedAt != nil {
		t.Fatalf("pre-add unexpectedly had created-at: %#v", records[0].Payload)
	}
	if records[1].Payload.Head == nil || records[1].Payload.Branch == nil || records[1].Payload.Metadata == nil || records[1].Payload.Metadata.CreatedAt == nil {
		t.Fatalf("post-add did not observe target: %#v", records[1].Payload)
	}
	if records[2].Payload.Metadata == nil || records[2].Payload.Metadata.CreatedAt == nil || *records[2].Payload.Metadata.CreatedAt != *records[1].Payload.Metadata.CreatedAt {
		t.Fatalf("pre-remove lost created-at: %#v", records[2].Payload)
	}
	if records[3].Payload.Metadata == nil || records[3].Payload.Metadata.Description == nil || *records[3].Payload.Metadata.Description != "hook metadata" || records[3].Payload.Metadata.CreatedAt == nil || *records[3].Payload.Metadata.CreatedAt != *records[1].Payload.Metadata.CreatedAt {
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
	repository.git("-C", repository.root, "config", "--local", "gwm.hooks.pre-add", relativeHookPath)

	recorder := &recordingExecutor{}
	var stdout bytes.Buffer
	var linkedStderr bytes.Buffer
	application := &App{
		git:      repository.runner,
		hooks:    recorder,
		stdout:   &stdout,
		stderr:   &linkedStderr,
		startDir: invocationRoot,
	}
	target := filepath.Join(repository.base, "relative-hook-target")
	exitCode = application.Run(t.Context(), []string{"add", target, "--detach", "--from", "HEAD"})
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
		repository.git("-C", repository.root, "config", "--local", "gwm.hooks.pre-add", failedHook)
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
		repository.git("-C", repository.root, "config", "--local", "gwm.hooks.pre-remove", failedHook)
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
		repository.git("-C", repository.root, "config", "--local", "gwm.hooks.post-add", failedHook)
		target := filepath.Join(repository.base, "partial")
		exitCode, _, stderr := repository.run("add", target, "-b", "partial", "--description", "preserved")
		if exitCode != 2 || !strings.Contains(stderr, "git worktree add completed") {
			t.Fatalf("post failure exit = %d, stderr = %q", exitCode, stderr)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("post-hook failure rolled back worktree: %v", err)
		}
		read, err := meta.Read(t.Context(), repository.runner, target)
		if err != nil || read.Description == nil || *read.Description != "preserved" {
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
		repository.git("-C", repository.root, "config", "--local", "gwm.hooks.post-remove", failedHook)
		exitCode, _, stderr = repository.run("remove", target)
		if exitCode != 2 || !strings.Contains(stderr, "git worktree remove completed") {
			t.Fatalf("post-remove failure exit = %d, stderr = %q", exitCode, stderr)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("post-remove failure restored target: %v", err)
		}
	})
}

func TestHookConfigValidation(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	nonExecutable := filepath.Join(repository.base, "not-executable")
	if err := os.WriteFile(nonExecutable, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	repository.git("-C", repository.root, "config", "--local", "--add", "gwm.hooks.pre-add", nonExecutable)
	target := filepath.Join(repository.base, "invalid-hook")
	exitCode, _, _ := repository.run("add", target, "-b", "invalid-hook")
	if exitCode != 1 {
		t.Fatalf("non-executable hook exit = %d, want 1", exitCode)
	}
	repository.git("-C", repository.root, "config", "--local", "--add", "gwm.hooks.pre-add", nonExecutable)
	exitCode, _, _ = repository.run("add", target, "-b", "invalid-hook")
	if exitCode != 1 {
		t.Fatalf("duplicate hook exit = %d, want 1", exitCode)
	}
}
