package gitcli

import (
	"context"
	"fmt"
	"strings"
)

// ConfigValues reads all values for a key using NUL delimiters. Missing is
// distinct from a Git failure because metadata absence is a normal state.
func ConfigValues(ctx context.Context, runner Runner, root, scope, key string, canonicalBool bool) (values []string, missing bool, err error) {
	args := []string{"-C", root, "config", scope, "--null"}
	if canonicalBool {
		args = append(args, "--bool")
	}
	args = append(args, "--get-all", key)
	result := runner.Run(ctx, args...)
	if result.Success() {
		return splitNUL(result.Stdout), false, nil
	}
	if result.ExitCode == 1 && len(result.Stdout) == 0 && len(result.Stderr) == 0 {
		return nil, true, nil
	}
	return nil, false, ResultError("read git config "+key, result)
}

func ResultError(action string, result Result) error {
	detail := strings.TrimSpace(string(result.Stderr))
	if detail == "" && result.Err != nil {
		detail = result.Err.Error()
	}
	if detail == "" {
		detail = fmt.Sprintf("exit code %d", result.ExitCode)
	}
	return fmt.Errorf("%s: %s", action, detail)
}

func splitNUL(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	if data[len(data)-1] == 0 {
		data = data[:len(data)-1]
	}
	parts := strings.Split(string(data), "\x00")
	return parts
}
