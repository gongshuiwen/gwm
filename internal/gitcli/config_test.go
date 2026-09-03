package gitcli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

type fixedRunner struct {
	result Result
	args   []string
}

func (r *fixedRunner) Run(_ context.Context, args ...string) Result {
	r.args = append([]string(nil), args...)
	return r.result
}

func TestConfigValues(t *testing.T) {
	t.Run("values", func(t *testing.T) {
		runner := &fixedRunner{result: Result{Stdout: []byte("first\x00second\x00")}}
		values, missing, err := ConfigValues(t.Context(), runner, "/repo", "--worktree", "gwm.key", true)
		if err != nil {
			t.Fatal(err)
		}
		if missing || !slices.Equal(values, []string{"first", "second"}) {
			t.Fatalf("ConfigValues() = %#v, %t", values, missing)
		}
		wantArgs := []string{"-C", "/repo", "config", "--worktree", "--null", "--bool", "--get-all", "gwm.key"}
		if !slices.Equal(runner.args, wantArgs) {
			t.Fatalf("args = %#v, want %#v", runner.args, wantArgs)
		}
	})

	t.Run("missing", func(t *testing.T) {
		runner := &fixedRunner{result: Result{ExitCode: 1}}
		values, missing, err := ConfigValues(t.Context(), runner, "/repo", "--local", "gwm.key", false)
		if err != nil || !missing || values != nil {
			t.Fatalf("ConfigValues() = %#v, %t, %v", values, missing, err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		runner := &fixedRunner{result: Result{Stderr: []byte("denied\n"), ExitCode: 2, Err: errors.New("exit status 2")}}
		_, missing, err := ConfigValues(t.Context(), runner, "/repo", "--local", "gwm.key", false)
		if err == nil || missing || !strings.Contains(err.Error(), "read git config gwm.key: denied") {
			t.Fatalf("ConfigValues() missing = %t, error = %v", missing, err)
		}
	})
}

func TestResultError(t *testing.T) {
	tests := []struct {
		name   string
		result Result
		want   string
	}{
		{name: "stderr", result: Result{Stderr: []byte("  failed\n"), ExitCode: 3}, want: "action: failed"},
		{name: "process error", result: Result{ExitCode: -1, Err: errors.New("not found")}, want: "action: not found"},
		{name: "exit code", result: Result{ExitCode: 9}, want: "action: exit code 9"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ResultError("action", test.result).Error(); got != test.want {
				t.Fatalf("ResultError() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSplitNUL(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []string
	}{
		{name: "empty"},
		{name: "trailing delimiter", data: []byte("one\x00two\x00"), want: []string{"one", "two"}},
		{name: "without trailing delimiter", data: []byte("one\x00two"), want: []string{"one", "two"}},
		{name: "empty value", data: []byte("\x00"), want: []string{""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := splitNUL(test.data); !slices.Equal(got, test.want) {
				t.Fatalf("splitNUL() = %#v, want %#v", got, test.want)
			}
		})
	}
}
