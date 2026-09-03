package gitcli

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"testing"
)

const helperProcessEnv = "GWM_GITCLI_HELPER_PROCESS"

func TestResultSuccess(t *testing.T) {
	if !(Result{}).Success() {
		t.Fatal("zero Result was not successful")
	}
	if (Result{ExitCode: 1}).Success() {
		t.Fatal("non-zero Result was successful")
	}
}

func TestCommandRunnerRun(t *testing.T) {
	for _, exitCode := range []int{0, 7} {
		t.Run(strconv.Itoa(exitCode), func(t *testing.T) {
			runner := &CommandRunner{
				Path: os.Args[0],
				Env:  append(os.Environ(), helperProcessEnv+"=1"),
			}
			result := runner.Run(t.Context(), "-test.run=^TestCommandRunnerHelperProcess$", "--", strconv.Itoa(exitCode))
			if string(result.Stdout) != "stdout" || string(result.Stderr) != "stderr" {
				t.Fatalf("output = %q, %q", result.Stdout, result.Stderr)
			}
			if result.ExitCode != exitCode {
				t.Fatalf("exit code = %d, want %d", result.ExitCode, exitCode)
			}
			if result.Success() != (exitCode == 0) {
				t.Fatalf("Success() = %t for exit code %d", result.Success(), exitCode)
			}
		})
	}

	t.Run("start failure", func(t *testing.T) {
		runner := &CommandRunner{Path: t.TempDir() + "/missing"}
		result := runner.Run(t.Context())
		if result.ExitCode != -1 || result.Err == nil {
			t.Fatalf("result = %#v", result)
		}
	})
}

func TestCommandRunnerHelperProcess(t *testing.T) {
	if os.Getenv(helperProcessEnv) != "1" {
		return
	}
	fmt.Fprint(os.Stdout, "stdout")
	fmt.Fprint(os.Stderr, "stderr")
	exitCode, err := strconv.Atoi(os.Args[len(os.Args)-1])
	if err != nil {
		os.Exit(99)
	}
	os.Exit(exitCode)
}

func TestCleanEnv(t *testing.T) {
	env := []string{
		"PATH=/bin",
		"HOME=/tmp/home",
		"GIT_DIR=/tmp/other",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=core.hooksPath",
		"GIT_CONFIG_VALUE_0=/tmp/hooks",
		"OTHER=value",
	}
	want := []string{"PATH=/bin", "HOME=/tmp/home", "OTHER=value"}
	if got := CleanEnv(env); !slices.Equal(got, want) {
		t.Fatalf("CleanEnv() = %#v, want %#v", got, want)
	}
}
