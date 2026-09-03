package hooks

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"gwm/internal/meta"
)

func TestPayloadShape(t *testing.T) {
	description := "work"
	branch := "refs/heads/topic"
	newBranch := "topic"
	payload := Payload{
		SchemaVersion:  1,
		Event:          PreAdd,
		CommonDir:      "/repo/.git",
		InvocationRoot: "/repo",
		WorktreePath:   "/topic",
		Branch:         &branch,
		Metadata:       &meta.Metadata{Description: &description, Protected: true},
		Options:        Options{NewBranch: &newBranch},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"event":"pre-add","common_dir":"/repo/.git","invocation_root":"/repo","worktree_path":"/topic","head":null,"branch":"refs/heads/topic","metadata":{"description":"work","protected":true},"options":{"new_branch":"topic","from":null,"detach":false,"force":false}}`
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
	err := NewExecutor().Run(context.Background(), path, t.TempDir(), Payload{
		SchemaVersion: 1,
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
