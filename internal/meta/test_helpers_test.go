package meta

import (
	"context"
	"slices"
	"testing"

	"github.com/gongshuiwen/gwm/internal/gitcli"
)

type runnerStep struct {
	args   []string
	result gitcli.Result
}

type scriptedRunner struct {
	t     *testing.T
	steps []runnerStep
	next  int
}

func newScriptedRunner(t *testing.T, steps ...runnerStep) *scriptedRunner {
	t.Helper()
	runner := &scriptedRunner{t: t, steps: steps}
	t.Cleanup(func() {
		if runner.next != len(runner.steps) {
			t.Errorf("runner consumed %d of %d steps", runner.next, len(runner.steps))
		}
	})
	return runner
}

func (r *scriptedRunner) Run(_ context.Context, args ...string) gitcli.Result {
	r.t.Helper()
	if r.next >= len(r.steps) {
		r.t.Fatalf("unexpected runner call: %#v", args)
	}
	step := r.steps[r.next]
	r.next++
	if !slices.Equal(args, step.args) {
		r.t.Fatalf("runner args = %#v, want %#v", args, step.args)
	}
	return step.result
}

func successfulConfig(value string) gitcli.Result {
	return gitcli.Result{Stdout: []byte(value + "\x00")}
}

func missingConfig() gitcli.Result {
	return gitcli.Result{ExitCode: 1}
}

func readArgs(path, key string, canonicalBool bool) []string {
	args := []string{"-C", path, "config", "--worktree", "--null"}
	if canonicalBool {
		args = append(args, "--bool")
	}
	return append(args, "--get-all", key)
}
