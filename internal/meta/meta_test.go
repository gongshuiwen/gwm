package meta

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gongshuiwen/gwm/internal/gitcli"
)

func TestValidateDescription(t *testing.T) {
	valid := strings.Repeat("界", 1365)
	if err := Validate(Metadata{Description: &valid}); err != nil {
		t.Fatalf("Validate(valid) error = %v", err)
	}

	tests := []struct {
		name        string
		description string
	}{
		{name: "too large", description: strings.Repeat("界", 1366)},
		{name: "nul", description: "bad\x00value"},
		{name: "invalid UTF-8", description: string([]byte{0xff})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(Metadata{Description: &test.description}); err == nil {
				t.Fatal("Validate() succeeded")
			}
		})
	}
}

func TestFormatCreatedAt(t *testing.T) {
	value := time.Date(2026, 9, 3, 16, 30, 0, 999, time.FixedZone("UTC+8", 8*60*60))
	if got := FormatCreatedAt(value); got != "2026-09-03T08:30:00Z" {
		t.Fatalf("FormatCreatedAt() = %q", got)
	}
	for _, invalid := range []string{
		"2026-09-03T08:30:00+00:00",
		"2026-09-03T08:30:00.1Z",
		"2026-09-03 08:30:00Z",
	} {
		if validCreatedAt(invalid) {
			t.Fatalf("validCreatedAt(%q) = true", invalid)
		}
	}
}

func TestDescriptionPointer(t *testing.T) {
	if DescriptionPointer("") != nil {
		t.Fatal("DescriptionPointer(empty) was not nil")
	}
	value := DescriptionPointer("text")
	if value == nil || *value != "text" {
		t.Fatalf("DescriptionPointer(text) = %#v", value)
	}
}

func TestRead(t *testing.T) {
	path := "/tree"
	runner := newScriptedRunner(t,
		runnerStep{args: readArgs(path, descriptionKey, false), result: successfulConfig("daily work")},
		runnerStep{args: readArgs(path, protectedKey, true), result: successfulConfig("true")},
		runnerStep{args: readArgs(path, createdAtKey, false), result: successfulConfig("2026-09-03T08:30:00Z")},
	)
	got, err := Read(t.Context(), runner, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description == nil || *got.Description != "daily work" || !got.Protected {
		t.Fatalf("Read() = %#v", got)
	}
	if got.CreatedAt == nil || *got.CreatedAt != "2026-09-03T08:30:00Z" || got.CreatedAtInvalid {
		t.Fatalf("Read() creation metadata = %#v", got)
	}
}

func TestReadMissingAndInvalidValues(t *testing.T) {
	t.Run("all missing", func(t *testing.T) {
		runner := newScriptedRunner(t,
			runnerStep{args: readArgs("/tree", descriptionKey, false), result: missingConfig()},
			runnerStep{args: readArgs("/tree", protectedKey, true), result: missingConfig()},
			runnerStep{args: readArgs("/tree", createdAtKey, false), result: missingConfig()},
		)
		got, err := Read(t.Context(), runner, "/tree")
		if err != nil || got != (Metadata{}) {
			t.Fatalf("Read() = %#v, %v", got, err)
		}
	})

	t.Run("invalid created-at is isolated", func(t *testing.T) {
		runner := newScriptedRunner(t,
			runnerStep{args: readArgs("/tree", descriptionKey, false), result: missingConfig()},
			runnerStep{args: readArgs("/tree", protectedKey, true), result: successfulConfig("false")},
			runnerStep{args: readArgs("/tree", createdAtKey, false), result: successfulConfig("invalid")},
		)
		got, err := Read(t.Context(), runner, "/tree")
		if err != nil || !got.CreatedAtInvalid || got.CreatedAt != nil {
			t.Fatalf("Read() = %#v, %v", got, err)
		}
	})

	t.Run("duplicate description", func(t *testing.T) {
		runner := newScriptedRunner(t,
			runnerStep{args: readArgs("/tree", descriptionKey, false), result: gitcli.Result{Stdout: []byte("one\x00two\x00")}},
		)
		_, err := Read(t.Context(), runner, "/tree")
		if err == nil || !strings.Contains(err.Error(), "must have exactly one value") {
			t.Fatalf("Read() error = %v", err)
		}
	})
}

func TestWrite(t *testing.T) {
	path := "/tree"
	description := "daily work"
	runner := newScriptedRunner(t,
		runnerStep{args: []string{"-C", path, "config", "--worktree", "--replace-all", descriptionKey, description}},
		runnerStep{args: []string{"-C", path, "config", "--worktree", "--replace-all", protectedKey, "true"}},
		runnerStep{args: readArgs(path, descriptionKey, false), result: successfulConfig(description)},
		runnerStep{args: readArgs(path, protectedKey, true), result: successfulConfig("true")},
		runnerStep{args: readArgs(path, createdAtKey, false), result: missingConfig()},
	)
	if err := Write(t.Context(), runner, path, Metadata{Description: &description, Protected: true}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteClearsDescription(t *testing.T) {
	path := "/tree"
	empty := ""
	runner := newScriptedRunner(t,
		runnerStep{args: []string{"-C", path, "config", "--worktree", "--unset-all", descriptionKey}, result: gitcli.Result{ExitCode: 5}},
		runnerStep{args: []string{"-C", path, "config", "--worktree", "--replace-all", protectedKey, "false"}},
		runnerStep{args: readArgs(path, descriptionKey, false), result: missingConfig()},
		runnerStep{args: readArgs(path, protectedKey, true), result: successfulConfig("false")},
		runnerStep{args: readArgs(path, createdAtKey, false), result: missingConfig()},
	)
	if err := Write(t.Context(), runner, path, Metadata{Description: &empty}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFailures(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		invalid := "bad\x00value"
		runner := newScriptedRunner(t)
		if err := Write(t.Context(), runner, "/tree", Metadata{Description: &invalid}); err == nil {
			t.Fatal("Write() succeeded")
		}
	})

	t.Run("description write", func(t *testing.T) {
		description := "work"
		runner := newScriptedRunner(t,
			runnerStep{
				args:   []string{"-C", "/tree", "config", "--worktree", "--replace-all", descriptionKey, description},
				result: gitcli.Result{ExitCode: 2, Stderr: []byte("denied")},
			},
		)
		err := Write(t.Context(), runner, "/tree", Metadata{Description: &description})
		if err == nil || !strings.Contains(err.Error(), "write "+descriptionKey) {
			t.Fatalf("Write() error = %v", err)
		}
	})

	t.Run("partial protected write", func(t *testing.T) {
		runner := newScriptedRunner(t,
			runnerStep{args: []string{"-C", "/tree", "config", "--worktree", "--unset-all", descriptionKey}, result: gitcli.Result{ExitCode: 5}},
			runnerStep{
				args:   []string{"-C", "/tree", "config", "--worktree", "--replace-all", protectedKey, "true"},
				result: gitcli.Result{ExitCode: 2, Stderr: []byte("denied")},
			},
		)
		err := Write(t.Context(), runner, "/tree", Metadata{Protected: true})
		if err == nil || !strings.Contains(err.Error(), "metadata may be partially updated") {
			t.Fatalf("Write() error = %v", err)
		}
	})

	t.Run("verification mismatch", func(t *testing.T) {
		description := "intended"
		runner := newScriptedRunner(t,
			runnerStep{args: []string{"-C", "/tree", "config", "--worktree", "--replace-all", descriptionKey, description}},
			runnerStep{args: []string{"-C", "/tree", "config", "--worktree", "--replace-all", protectedKey, "false"}},
			runnerStep{args: readArgs("/tree", descriptionKey, false), result: successfulConfig("other")},
			runnerStep{args: readArgs("/tree", protectedKey, true), result: successfulConfig("false")},
			runnerStep{args: readArgs("/tree", createdAtKey, false), result: missingConfig()},
		)
		err := Write(t.Context(), runner, "/tree", Metadata{Description: &description})
		if err == nil || !strings.Contains(err.Error(), "did not match") {
			t.Fatalf("Write() error = %v", err)
		}
	})
}

func TestWriteCreatedAt(t *testing.T) {
	const value = "2026-09-03T08:30:00Z"
	runner := newScriptedRunner(t,
		runnerStep{args: []string{"-C", "/tree", "config", "--worktree", "--replace-all", createdAtKey, value}},
		runnerStep{args: readArgs("/tree", createdAtKey, false), result: successfulConfig(value)},
	)
	if err := WriteCreatedAt(t.Context(), runner, "/tree", value); err != nil {
		t.Fatal(err)
	}
}

func TestWriteCreatedAtFailures(t *testing.T) {
	t.Run("invalid input", func(t *testing.T) {
		runner := newScriptedRunner(t)
		if err := WriteCreatedAt(t.Context(), runner, "/tree", "invalid"); err == nil {
			t.Fatal("WriteCreatedAt() succeeded")
		}
	})

	t.Run("partial write", func(t *testing.T) {
		const value = "2026-09-03T08:30:00Z"
		runner := newScriptedRunner(t,
			runnerStep{
				args:   []string{"-C", "/tree", "config", "--worktree", "--replace-all", createdAtKey, value},
				result: gitcli.Result{ExitCode: 2, Err: errors.New("failed")},
			},
		)
		err := WriteCreatedAt(t.Context(), runner, "/tree", value)
		if err == nil || !strings.Contains(err.Error(), "metadata may be partially updated") {
			t.Fatalf("WriteCreatedAt() error = %v", err)
		}
	})

	t.Run("verification mismatch", func(t *testing.T) {
		const value = "2026-09-03T08:30:00Z"
		runner := newScriptedRunner(t,
			runnerStep{args: []string{"-C", "/tree", "config", "--worktree", "--replace-all", createdAtKey, value}},
			runnerStep{args: readArgs("/tree", createdAtKey, false), result: successfulConfig("2026-09-03T08:30:01Z")},
		)
		err := WriteCreatedAt(t.Context(), runner, "/tree", value)
		if err == nil || !strings.Contains(err.Error(), "did not match") {
			t.Fatalf("WriteCreatedAt() error = %v", err)
		}
	})
}

func TestEqualEditable(t *testing.T) {
	one := "one"
	two := "two"
	tests := []struct {
		name        string
		left, right Metadata
		want        bool
	}{
		{name: "empty", want: true},
		{name: "equal", left: Metadata{Description: &one, Protected: true}, right: Metadata{Description: &one, Protected: true}, want: true},
		{name: "protection differs", left: Metadata{Protected: true}, right: Metadata{}},
		{name: "nil differs", left: Metadata{Description: &one}, right: Metadata{}},
		{name: "description differs", left: Metadata{Description: &one}, right: Metadata{Description: &two}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EqualEditable(test.left, test.right); got != test.want {
				t.Fatalf("EqualEditable() = %t, want %t", got, test.want)
			}
		})
	}
}
