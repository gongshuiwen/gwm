package gitcli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Result preserves Git's observable process result without interpreting it.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

func (r Result) Success() bool {
	return r.Err == nil && r.ExitCode == 0
}

// Runner is the only production path for invoking Git.
type Runner interface {
	Run(context.Context, ...string) Result
}

type CommandRunner struct {
	Path string
	Env  []string
}

func New() *CommandRunner {
	return &CommandRunner{Path: "git", Env: CleanEnv(os.Environ())}
}

func (r *CommandRunner) Run(ctx context.Context, args ...string) Result {
	path := r.Path
	if path == "" {
		path = "git"
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = r.Env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: 0, Err: err}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result
	}
	result.ExitCode = -1
	return result
}

// CleanEnv removes caller-controlled Git repository redirection and config
// injection while preserving ordinary process settings such as PATH and HOME.
func CleanEnv(env []string) []string {
	blocked := map[string]struct{}{
		"GIT_DIR":                          {},
		"GIT_WORK_TREE":                    {},
		"GIT_COMMON_DIR":                   {},
		"GIT_INDEX_FILE":                   {},
		"GIT_OBJECT_DIRECTORY":             {},
		"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
		"GIT_NAMESPACE":                    {},
		"GIT_CEILING_DIRECTORIES":          {},
		"GIT_DISCOVERY_ACROSS_FILESYSTEM":  {},
		"GIT_CONFIG":                       {},
		"GIT_CONFIG_GLOBAL":                {},
		"GIT_CONFIG_SYSTEM":                {},
		"GIT_CONFIG_NOSYSTEM":              {},
		"GIT_CONFIG_PARAMETERS":            {},
		"GIT_CONFIG_COUNT":                 {},
	}
	cleaned := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, found := blocked[name]; found {
			continue
		}
		if strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		cleaned = append(cleaned, entry)
	}
	return cleaned
}
