// Package meta reads and writes GWM-owned worktree metadata.
package meta

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gongshuiwen/gwm/internal/gitcli"
)

const maxDescriptionBytes = 4096

const createdAtLayout = "2006-01-02T15:04:05Z"

const (
	descriptionKey = "gwm.worktree.description"
	protectedKey   = "gwm.worktree.protected"
	createdAtKey   = "gwm.worktree.created-at"
)

// Metadata contains the GWM-owned values stored in worktree-scoped Git config.
type Metadata struct {
	Description      *string `json:"description"`
	Protected        bool    `json:"protected"`
	CreatedAt        *string `json:"created_at"`
	CreatedAtInvalid bool    `json:"-"`
}

// Validate checks the editable metadata fields.
func Validate(value Metadata) error {
	if value.Description == nil {
		return nil
	}
	description := *value.Description
	if !utf8.ValidString(description) || strings.ContainsRune(description, '\x00') {
		return errors.New("description must be valid UTF-8 without NUL")
	}
	if len(description) > maxDescriptionBytes {
		return fmt.Errorf("description exceeds %d UTF-8 bytes", maxDescriptionBytes)
	}
	return nil
}

// Read loads and validates metadata from one worktree.
func Read(ctx context.Context, runner gitcli.Runner, worktreePath string) (Metadata, error) {
	var metadata Metadata
	descriptions, descriptionMissing, err := gitcli.ConfigValues(ctx, runner, worktreePath, "--worktree", descriptionKey, false)
	if err != nil {
		return Metadata{}, err
	}
	if !descriptionMissing {
		if len(descriptions) != 1 {
			return Metadata{}, fmt.Errorf("%s must have exactly one value", descriptionKey)
		}
		if descriptions[0] != "" {
			description := descriptions[0]
			metadata.Description = &description
		}
		if err := Validate(metadata); err != nil {
			return Metadata{}, err
		}
	}

	protected, protectedMissing, err := gitcli.ConfigValues(ctx, runner, worktreePath, "--worktree", protectedKey, true)
	if err != nil {
		return Metadata{}, err
	}
	if !protectedMissing {
		if len(protected) != 1 {
			return Metadata{}, fmt.Errorf("%s must have exactly one value", protectedKey)
		}
		metadata.Protected = protected[0] == "true"
	}

	createdAt, invalid, err := readCreatedAt(ctx, runner, worktreePath)
	if err != nil {
		return Metadata{}, err
	}
	metadata.CreatedAt = createdAt
	metadata.CreatedAtInvalid = invalid
	return metadata, nil
}

// Write updates editable metadata and verifies the resulting values.
func Write(ctx context.Context, runner gitcli.Runner, worktreePath string, intended Metadata) error {
	if intended.Description != nil && *intended.Description == "" {
		intended.Description = nil
	}
	if err := Validate(intended); err != nil {
		return err
	}
	if intended.Description == nil {
		result := runner.Run(ctx, "-C", worktreePath, "config", "--worktree", "--unset-all", descriptionKey)
		if !result.Success() && result.ExitCode != 5 {
			return gitcli.ResultError("unset "+descriptionKey, result)
		}
	} else {
		result := runner.Run(ctx, "-C", worktreePath, "config", "--worktree", "--replace-all", descriptionKey, *intended.Description)
		if !result.Success() {
			return gitcli.ResultError("write "+descriptionKey, result)
		}
	}

	protected := "false"
	if intended.Protected {
		protected = "true"
	}
	result := runner.Run(ctx, "-C", worktreePath, "config", "--worktree", "--replace-all", protectedKey, protected)
	if !result.Success() {
		return fmt.Errorf("metadata may be partially updated: %w", gitcli.ResultError("write "+protectedKey, result))
	}

	after, readErr := Read(ctx, runner, worktreePath)
	if readErr != nil {
		return fmt.Errorf("verify worktree metadata: %w", readErr)
	}
	if !EqualEditable(after, intended) {
		return errors.New("worktree metadata did not match the intended value after write")
	}
	return nil
}

// WriteCreatedAt stores and verifies the creation timestamp written by add.
func WriteCreatedAt(ctx context.Context, runner gitcli.Runner, worktreePath, intended string) error {
	if !validCreatedAt(intended) {
		return errors.New("created-at must be a UTC RFC 3339 timestamp with second precision")
	}
	result := runner.Run(ctx, "-C", worktreePath, "config", "--worktree", "--replace-all", createdAtKey, intended)
	if !result.Success() {
		return fmt.Errorf("metadata may be partially updated: %w", gitcli.ResultError("write "+createdAtKey, result))
	}
	after, invalid, err := readCreatedAt(ctx, runner, worktreePath)
	if err != nil {
		return fmt.Errorf("metadata may be partially updated: verify %s: %w", createdAtKey, err)
	}
	if invalid || after == nil || *after != intended {
		return fmt.Errorf("metadata may be partially updated: %s did not match the intended value after write", createdAtKey)
	}
	return nil
}

// EqualEditable reports whether description and protection values are equal.
func EqualEditable(left, right Metadata) bool {
	if left.Protected != right.Protected {
		return false
	}
	if left.Description == nil || right.Description == nil {
		return left.Description == nil && right.Description == nil
	}
	return *left.Description == *right.Description
}

// FormatCreatedAt returns the canonical timestamp representation.
func FormatCreatedAt(value time.Time) string {
	return value.UTC().Truncate(time.Second).Format(createdAtLayout)
}

func readCreatedAt(ctx context.Context, runner gitcli.Runner, worktreePath string) (*string, bool, error) {
	values, missing, err := gitcli.ConfigValues(ctx, runner, worktreePath, "--worktree", createdAtKey, false)
	if err != nil {
		return nil, false, err
	}
	if missing {
		return nil, false, nil
	}
	if len(values) != 1 || !validCreatedAt(values[0]) {
		return nil, true, nil
	}
	value := values[0]
	return &value, false, nil
}

func validCreatedAt(value string) bool {
	parsed, err := time.Parse(time.RFC3339, value)
	return err == nil && parsed.UTC().Format(createdAtLayout) == value
}

// DescriptionPointer converts the external empty-string representation to
// the nil representation used by Metadata.
func DescriptionPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
