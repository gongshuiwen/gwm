package hooks

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gongshuiwen/gwm/internal/gitcli"
	"github.com/gongshuiwen/gwm/internal/meta"
)

type fixedRunner struct {
	result gitcli.Result
	args   []string
}

func (r *fixedRunner) Run(_ context.Context, args ...string) gitcli.Result {
	r.args = append([]string(nil), args...)
	return r.result
}

func TestPayloadShape(t *testing.T) {
	description := "work"
	createdAt := "2026-09-03T08:30:00Z"
	branch := "refs/heads/topic"
	newBranch := "topic"
	payload := Payload{
		SchemaVersion:  SchemaVersion,
		Event:          PostAdd,
		CommonDir:      "/repo/.git",
		InvocationRoot: "/repo",
		WorktreePath:   "/topic",
		Branch:         &branch,
		Metadata:       &meta.Metadata{Description: &description, Protected: true, CreatedAt: &createdAt},
		Options:        Options{NewBranch: &newBranch},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":2,"event":"post-add","common_dir":"/repo/.git","invocation_root":"/repo","worktree_path":"/topic","head":null,"branch":"refs/heads/topic","metadata":{"description":"work","protected":true,"created_at":"2026-09-03T08:30:00Z"},"options":{"new_branch":"topic","from":null,"detach":false,"force":false}}`
	if string(data) != want {
		t.Fatalf("payload = %s\nwant    = %s", data, want)
	}
}

func TestCommandExecutorWritesJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a local test fixture")
	}
	path := writeExecutable(t, "#!/bin/sh\ncat\n")
	var stdout strings.Builder
	err := NewExecutor().Run(t.Context(), path, t.TempDir(), Payload{
		SchemaVersion: SchemaVersion,
		Event:         PostRemove,
		Options:       Options{},
	}, &stdout, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("hook stdin was not JSON: %v", err)
	}
	if decoded["event"] != PostRemove {
		t.Fatalf("event = %v", decoded["event"])
	}
}

func TestCommandExecutorReportsFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("executes a local test fixture")
	}
	path := writeExecutable(t, "#!/bin/sh\nexit 7\n")
	err := NewExecutor().Run(t.Context(), path, t.TempDir(), Payload{Event: PreAdd}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "hook pre-add failed") {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestConfiguredPath(t *testing.T) {
	t.Run("relative executable", func(t *testing.T) {
		root := t.TempDir()
		directory := filepath.Join(root, ".githooks")
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, "lifecycle")
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		runner := &fixedRunner{result: gitcli.Result{Stdout: []byte(".githooks/lifecycle\x00")}}
		got, configured, err := ConfiguredPath(t.Context(), runner, root, PreAdd)
		if err != nil {
			t.Fatal(err)
		}
		if !configured || got != path {
			t.Fatalf("ConfiguredPath() = %q, %t", got, configured)
		}
		wantArgs := []string{"-C", root, "config", "--local", "--null", "--get-all", "gwm.hooks.pre-add"}
		if !slices.Equal(runner.args, wantArgs) {
			t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
		}
	})

	t.Run("missing", func(t *testing.T) {
		runner := &fixedRunner{result: gitcli.Result{ExitCode: 1}}
		path, configured, err := ConfiguredPath(t.Context(), runner, t.TempDir(), PostAdd)
		if err != nil || configured || path != "" {
			t.Fatalf("ConfiguredPath() = %q, %t, %v", path, configured, err)
		}
	})

	tests := []struct {
		name  string
		value string
		setup func(*testing.T, string)
		want  string
	}{
		{name: "duplicate", value: "one\x00two\x00", want: "must have exactly one value"},
		{name: "empty", value: "\x00", want: "must not be empty"},
		{name: "missing file", value: "missing\x00", want: "inspect gwm.hooks.pre-remove"},
		{name: "directory", value: "hook\x00", setup: func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, "hook"), 0o700); err != nil {
				t.Fatal(err)
			}
		}, want: "must point to a regular file"},
		{name: "not executable", value: "hook\x00", setup: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "hook"), []byte("hook"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: "must point to an executable file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.setup != nil {
				test.setup(t, root)
			}
			runner := &fixedRunner{result: gitcli.Result{Stdout: []byte(test.value)}}
			_, _, err := ConfiguredPath(t.Context(), runner, root, PreRemove)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ConfiguredPath() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestConfiguredPathRejectsUnknownEvent(t *testing.T) {
	_, _, err := ConfiguredPath(t.Context(), &fixedRunner{}, t.TempDir(), "unknown")
	if err == nil || !strings.Contains(err.Error(), "unknown hook event") {
		t.Fatalf("ConfiguredPath() error = %v", err)
	}
}

func TestValidEvent(t *testing.T) {
	for _, event := range []string{PreAdd, PostAdd, PreRemove, PostRemove} {
		if !validEvent(event) {
			t.Fatalf("validEvent(%q) = false", event)
		}
	}
	if validEvent("other") {
		t.Fatal("validEvent(other) = true")
	}
}
