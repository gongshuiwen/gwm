package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/gongshuiwen/gwm/internal/gitcli"
	"github.com/gongshuiwen/gwm/internal/hooks"
)

func TestHelpAndVersionDoNotDiscoverRepository(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		want  string
		exact bool
	}{
		{name: "root help", args: []string{"--help"}, want: "commands:"},
		{name: "version", args: []string{"--version"}, want: "gwm unreleased\n", exact: true},
		{name: "version after C", args: []string{"-C", "does-not-exist", "--version"}, want: "gwm unreleased\n", exact: true},
		{name: "init help", args: []string{"init", "--help"}, want: "extensions"},
		{name: "list help", args: []string{"list", "--help"}, want: "creation metadata"},
		{name: "add help", args: []string{"add", "--help"}, want: "--description"},
		{name: "meta help", args: []string{"meta", "--help"}, want: "--protected"},
		{name: "remove help after C", args: []string{"-C", "does-not-exist", "remove", "--help"}, want: "--force"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &countingGitRunner{}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			application := &App{
				git:      runner,
				hooks:    hooks.NewExecutor(),
				stdout:   &stdout,
				stderr:   &stderr,
				startDir: t.TempDir(),
			}
			if exitCode := application.Run(t.Context(), test.args); exitCode != 0 {
				t.Fatalf("Run() exit = %d, stderr = %q", exitCode, stderr.String())
			}
			if test.exact && stdout.String() != test.want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), test.want)
			}
			if !test.exact && !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("stdout = %q, want substring %q", stdout.String(), test.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
			if runner.calls != 0 {
				t.Fatalf("Git calls = %d, want 0", runner.calls)
			}
		})
	}
}

func TestInvalidHelpAndVersionCombinationsAreUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"--help", "extra"},
		{"--version", "extra"},
		{"add", "--help", "extra"},
		{"init", "--version"},
		{"unknown", "--help"},
	} {
		runner := &countingGitRunner{}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		application := &App{
			git:      runner,
			hooks:    hooks.NewExecutor(),
			stdout:   &stdout,
			stderr:   &stderr,
			startDir: t.TempDir(),
		}
		if exitCode := application.Run(t.Context(), args); exitCode != 1 {
			t.Fatalf("args %v: exit = %d, want 1", args, exitCode)
		}
		if stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("args %v: stdout = %q, stderr = %q", args, stdout.String(), stderr.String())
		}
		if runner.calls != 0 {
			t.Fatalf("args %v: Git calls = %d, want 0", args, runner.calls)
		}
	}
}

func TestRunRejectsIncompleteConfiguration(t *testing.T) {
	var nilApp *App
	if exitCode := nilApp.Run(t.Context(), nil); exitCode != 1 {
		t.Fatalf("nil App exit = %d, want 1", exitCode)
	}

	var stderr bytes.Buffer
	application := &App{stderr: &stderr}
	if exitCode := application.Run(t.Context(), nil); exitCode != 1 {
		t.Fatalf("incomplete App exit = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "dependencies are not configured") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

type countingGitRunner struct {
	calls int
}

func (r *countingGitRunner) Run(context.Context, ...string) gitcli.Result {
	r.calls++
	return gitcli.Result{}
}
