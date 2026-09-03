package gitcli

import (
	"slices"
	"testing"
)

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
