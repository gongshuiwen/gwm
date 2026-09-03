package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"gwm/internal/gitcli"
	"gwm/internal/meta"
)

const (
	PreAdd     = "pre-add"
	PostAdd    = "post-add"
	PreRemove  = "pre-remove"
	PostRemove = "post-remove"
)

type Options struct {
	NewBranch *string `json:"new_branch"`
	From      *string `json:"from"`
	Detach    bool    `json:"detach"`
	Force     bool    `json:"force"`
}

type Payload struct {
	SchemaVersion  int            `json:"schema_version"`
	Event          string         `json:"event"`
	CommonDir      string         `json:"common_dir"`
	InvocationRoot string         `json:"invocation_root"`
	WorktreePath   string         `json:"worktree_path"`
	Head           *string        `json:"head"`
	Branch         *string        `json:"branch"`
	Metadata       *meta.Metadata `json:"metadata"`
	Options        Options        `json:"options"`
}

type Executor interface {
	Run(context.Context, string, string, Payload, io.Writer, io.Writer) error
}

type CommandExecutor struct {
	Env []string
}

func NewExecutor() *CommandExecutor {
	return &CommandExecutor{Env: gitcli.CleanEnv(os.Environ())}
}

func (e *CommandExecutor) Run(ctx context.Context, path, dir string, payload Payload, stdout, stderr io.Writer) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode hook payload: %w", err)
	}
	data = append(data, '\n')
	cmd := exec.CommandContext(ctx, path)
	cmd.Dir = dir
	cmd.Env = e.Env
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("hook %s failed: %w", payload.Event, err)
	}
	return nil
}

// ConfiguredPath reads one Hook from repository-local config and validates the
// executable before a modifying Git command is allowed to run.
func ConfiguredPath(ctx context.Context, runner gitcli.Runner, repositoryRoot, event string) (string, bool, error) {
	if !validEvent(event) {
		return "", false, fmt.Errorf("unknown hook event %q", event)
	}
	key := "gwm.hook." + event
	values, missing, err := gitcli.ConfigValues(ctx, runner, repositoryRoot, "--local", key, false)
	if err != nil {
		return "", false, err
	}
	if missing {
		return "", false, nil
	}
	if len(values) != 1 {
		return "", false, fmt.Errorf("%s must have exactly one value", key)
	}
	path := values[0]
	if !filepath.IsAbs(path) {
		return "", false, fmt.Errorf("%s must be an absolute path", key)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", false, fmt.Errorf("inspect %s: %w", key, err)
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("%s must point to a regular file", key)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", false, fmt.Errorf("%s must point to an executable file", key)
	}
	return path, true, nil
}

func validEvent(event string) bool {
	switch event {
	case PreAdd, PostAdd, PreRemove, PostRemove:
		return true
	default:
		return false
	}
}
