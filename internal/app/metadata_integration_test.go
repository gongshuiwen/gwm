package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gongshuiwen/gwm/internal/gitcli"
	"github.com/gongshuiwen/gwm/internal/hooks"
	"github.com/gongshuiwen/gwm/internal/meta"
)

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
	if strings.Contains(joined, "config\x00--worktree\x00--replace-all\x00gwm.worktree.protected") {
		return gitcli.Result{ExitCode: 9, Err: errors.New("injected metadata write failure")}
	}
	return r.inner.Run(ctx, args...)
}

type failingCreatedAtRunner struct {
	inner gitcli.Runner
}

func (r *failingCreatedAtRunner) Run(ctx context.Context, args ...string) gitcli.Result {
	joined := strings.Join(args, "\x00")
	if strings.Contains(joined, "config\x00--worktree\x00--replace-all\x00gwm.worktree.created-at") {
		return gitcli.Result{ExitCode: 9, Err: errors.New("injected created-at write failure")}
	}
	return r.inner.Run(ctx, args...)
}

func TestMetadataLifecycleAndNativeBoundaries(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()

	target := filepath.Join(repository.base, "linked 空间")
	started := time.Now().UTC().Add(-time.Second)
	exitCode, _, stderr := repository.run("add", target, "-b", "feature/topic", "--from", "HEAD", "--description", "line\nnext", "--protected")
	finished := time.Now().UTC().Add(time.Second)
	if exitCode != 0 {
		t.Fatalf("gwm add exited %d: %s", exitCode, stderr)
	}
	read, err := meta.Read(t.Context(), repository.runner, target)
	if err != nil || read.Description == nil || *read.Description != "line\nnext" || !read.Protected || read.CreatedAt == nil || read.CreatedAtInvalid {
		t.Fatalf("metadata after add = %#v, error = %v", read, err)
	}
	createdAt, err := time.Parse(time.RFC3339, *read.CreatedAt)
	if err != nil || createdAt.Before(started) || createdAt.After(finished) {
		t.Fatalf("created-at = %q, range = [%s, %s], error = %v", *read.CreatedAt, started, finished, err)
	}
	originalCreatedAt := *read.CreatedAt
	description := repository.git("-C", target, "config", "--worktree", "--get-all", "gwm.worktree.description")
	if string(description.Stdout) != "line\nnext\n" {
		t.Fatalf("description config = %q", description.Stdout)
	}
	protected := repository.git("-C", target, "config", "--worktree", "--bool", "--get-all", "gwm.worktree.protected")
	if string(protected.Stdout) != "true\n" {
		t.Fatalf("protected config = %q", protected.Stdout)
	}
	createdAtConfig := repository.git("-C", target, "config", "--worktree", "--get-all", "gwm.worktree.created-at")
	if string(createdAtConfig.Stdout) != originalCreatedAt+"\n" {
		t.Fatalf("created-at config = %q", createdAtConfig.Stdout)
	}
	legacy := repository.runner.Run(t.Context(), "-C", target, "config", "--worktree", "--get-all", "gwm.metadata")
	if legacy.Success() {
		t.Fatalf("legacy metadata key was written: %q", legacy.Stdout)
	}

	exitCode, stdout, stderr := repository.run("list")
	if exitCode != 0 {
		t.Fatalf("gwm list exited %d: %s", exitCode, stderr)
	}
	if !strings.Contains(stdout, "CREATED_AT") || !strings.Contains(stdout, originalCreatedAt) || !strings.Contains(stdout, "feature/topic") || !strings.Contains(stdout, `line\nnext`) || strings.Contains(stdout, "line\nnext") {
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
	read, err = meta.Read(t.Context(), repository.runner, target)
	if err != nil || read.Description != nil || read.Protected || read.CreatedAt == nil || *read.CreatedAt != originalCreatedAt {
		t.Fatalf("metadata after edit = %#v, error = %v", read, err)
	}
	description = repository.runner.Run(t.Context(), "-C", target, "config", "--worktree", "--get-all", "gwm.worktree.description")
	if description.Success() {
		t.Fatalf("empty description was retained: %q", description.Stdout)
	}
	protected = repository.git("-C", target, "config", "--worktree", "--bool", "--get-all", "gwm.worktree.protected")
	if string(protected.Stdout) != "false\n" {
		t.Fatalf("protected config after edit = %q", protected.Stdout)
	}
	exitCode, stdout, stderr = repository.run("meta", target)
	if exitCode != 0 || !strings.Contains(stdout, "CREATED_AT\t"+originalCreatedAt) {
		t.Fatalf("gwm meta display exited %d: %s\n%s", exitCode, stderr, stdout)
	}

	exitCode, _, stderr = repository.run("remove", target)
	if exitCode != 0 {
		t.Fatalf("gwm remove exited %d: %s", exitCode, stderr)
	}
	branchResult := repository.runner.Run(t.Context(), "-C", repository.root, "show-ref", "--verify", "refs/heads/feature/topic")
	if !branchResult.Success() {
		t.Fatal("remove deleted the worktree branch")
	}

	exitCode, _, _ = repository.run("remove", repository.root)
	if exitCode != 1 {
		t.Fatalf("main worktree remove exit = %d, want 1", exitCode)
	}
}

func TestCreatedAtNativeBoundaryAndInvalidIsolation(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()

	nativeTarget := filepath.Join(repository.base, "native-created")
	repository.git("-C", repository.root, "worktree", "add", "-b", "native-created", nativeTarget)
	nativeMetadata, err := meta.Read(t.Context(), repository.runner, nativeTarget)
	if err != nil || nativeMetadata.CreatedAt != nil || nativeMetadata.CreatedAtInvalid {
		t.Fatalf("native metadata = %#v, error = %v", nativeMetadata, err)
	}
	exitCode, stdout, stderr := repository.run("meta", nativeTarget)
	if exitCode != 0 || !strings.Contains(stdout, "CREATED_AT\t-") {
		t.Fatalf("native meta exited %d: %s\n%s", exitCode, stderr, stdout)
	}

	target := filepath.Join(repository.base, "invalid-created-at")
	exitCode, _, stderr = repository.run("add", target, "-b", "invalid-created-at")
	if exitCode != 0 {
		t.Fatalf("gwm add exited %d: %s", exitCode, stderr)
	}
	repository.git("-C", target, "config", "--worktree", "--replace-all", "gwm.worktree.created-at", "not-a-time")
	metadata, err := meta.Read(t.Context(), repository.runner, target)
	if err != nil || metadata.CreatedAt != nil || !metadata.CreatedAtInvalid {
		t.Fatalf("invalid created-at metadata = %#v, error = %v", metadata, err)
	}
	exitCode, stdout, stderr = repository.run("list")
	if exitCode != 0 || !lineForPathContains(stdout, target, "INVALID") {
		t.Fatalf("invalid created-at list exited %d: %s\n%s", exitCode, stderr, stdout)
	}
	exitCode, _, stderr = repository.run("meta", target, "--description", "still editable")
	if exitCode != 0 {
		t.Fatalf("meta with invalid created-at exited %d: %s", exitCode, stderr)
	}
	repository.git("-C", target, "config", "--worktree", "--add", "gwm.worktree.created-at", "also-invalid")
	metadata, err = meta.Read(t.Context(), repository.runner, target)
	if err != nil || !metadata.CreatedAtInvalid {
		t.Fatalf("duplicate created-at metadata = %#v, error = %v", metadata, err)
	}
	exitCode, _, stderr = repository.run("remove", target)
	if exitCode != 0 {
		t.Fatalf("remove with invalid created-at exited %d: %s", exitCode, stderr)
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
	repository.git("-C", target, "config", "--worktree", "--add", "gwm.worktree.description", "first")
	repository.git("-C", target, "config", "--worktree", "--add", "gwm.worktree.description", "second")

	exitCode, stdout, stderr = repository.run("list")
	if exitCode != 0 || !lineForPathContains(stdout, target, "INVALID") {
		t.Fatalf("duplicate metadata list exited %d: %s\n%s", exitCode, stderr, stdout)
	}
	repository.git("-C", target, "config", "--worktree", "--replace-all", "gwm.worktree.description", "valid")
	repository.git("-C", target, "config", "--worktree", "--replace-all", "gwm.worktree.protected", "not-a-boolean")
	exitCode, stdout, stderr = repository.run("list")
	if exitCode != 0 || !lineForPathContains(stdout, target, "INVALID") {
		t.Fatalf("invalid boolean list exited %d: %s\n%s", exitCode, stderr, stdout)
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

func TestMetadataWriteFailureIsPartialAndStillRunsPostAdd(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	recorder := &recordingExecutor{}
	runner := &failingMetadataRunner{inner: repository.runner}
	target := filepath.Join(repository.base, "metadata-partial")
	repository.git("-C", repository.root, "config", "--local", "gwm.hooks.post-add", writeHook(t, repository.base, "#!/bin/sh\nexit 0\n"))
	exitCode, _, stderr := repository.runWith(runner, recorder, "add", target, "-b", "metadata-partial", "--description", "written-before-failure")
	if exitCode != 2 || !strings.Contains(stderr, "git worktree add completed") || !strings.Contains(stderr, "metadata may be partially updated") {
		t.Fatalf("metadata failure exit = %d, stderr = %q", exitCode, stderr)
	}
	if len(recorder.events) != 1 || recorder.events[0] != hooks.PostAdd {
		t.Fatalf("post-add was not run after metadata failure: %v", recorder.events)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("metadata failure rolled back worktree: %v", err)
	}
	description := repository.git("-C", target, "config", "--worktree", "--get-all", "gwm.worktree.description")
	if string(description.Stdout) != "written-before-failure\n" {
		t.Fatalf("description write was rolled back: %q", description.Stdout)
	}
	protected := repository.runner.Run(t.Context(), "-C", target, "config", "--worktree", "--get-all", "gwm.worktree.protected")
	if protected.Success() {
		t.Fatalf("failed protected write unexpectedly persisted: %q", protected.Stdout)
	}
}

func TestCreatedAtWriteFailureIsPartialAndStillRunsPostAdd(t *testing.T) {
	repository := newTemporaryRepository(t)
	repository.initGWM()
	recorder := &recordingExecutor{}
	runner := &failingCreatedAtRunner{inner: repository.runner}
	target := filepath.Join(repository.base, "created-at-partial")
	repository.git("-C", repository.root, "config", "--local", "gwm.hooks.post-add", writeHook(t, repository.base, "#!/bin/sh\nexit 0\n"))
	exitCode, _, stderr := repository.runWith(runner, recorder, "add", target, "-b", "created-at-partial", "--description", "preserved", "--protected")
	if exitCode != 2 || !strings.Contains(stderr, "git worktree add completed") || !strings.Contains(stderr, "metadata may be partially updated") {
		t.Fatalf("created-at failure exit = %d, stderr = %q", exitCode, stderr)
	}
	metadata, err := meta.Read(t.Context(), repository.runner, target)
	if err != nil || metadata.Description == nil || *metadata.Description != "preserved" || !metadata.Protected || metadata.CreatedAt != nil {
		t.Fatalf("metadata after created-at failure = %#v, error = %v", metadata, err)
	}
	if len(recorder.payloads) != 1 || recorder.payloads[0].Metadata == nil || recorder.payloads[0].Metadata.CreatedAt != nil {
		t.Fatalf("post-add payloads = %#v", recorder.payloads)
	}
}
