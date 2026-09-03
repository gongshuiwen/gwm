package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"gwm/internal/gitcli"
)

const maxValueBytes = 16 * 1024
const maxDescriptionBytes = 4096

type Metadata struct {
	Description *string `json:"description"`
	Protected   bool    `json:"protected"`
}

type ReadResult struct {
	Value   Metadata
	Present bool
	Invalid error
}

func Default() Metadata {
	return Metadata{}
}

func Decode(data []byte) (Metadata, error) {
	if len(data) > maxValueBytes {
		return Metadata{}, fmt.Errorf("metadata exceeds 16 KiB")
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return Metadata{}, fmt.Errorf("metadata must be valid UTF-8 without NUL")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil {
		return Metadata{}, fmt.Errorf("decode metadata: %w", err)
	}
	delim, ok := first.(json.Delim)
	if !ok || delim != '{' {
		return Metadata{}, fmt.Errorf("metadata must be a JSON object")
	}
	var value Metadata
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return Metadata{}, fmt.Errorf("decode metadata key: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return Metadata{}, fmt.Errorf("metadata key must be a string")
		}
		if seen[key] {
			return Metadata{}, fmt.Errorf("metadata contains duplicate key %q", key)
		}
		seen[key] = true
		switch key {
		case "description":
			if err := decoder.Decode(&value.Description); err != nil {
				return Metadata{}, fmt.Errorf("description must be a string or null")
			}
		case "protected":
			if err := decoder.Decode(&value.Protected); err != nil {
				return Metadata{}, fmt.Errorf("protected must be boolean")
			}
		default:
			return Metadata{}, fmt.Errorf("metadata contains unknown field %q", key)
		}
	}
	closing, err := decoder.Token()
	if err != nil {
		return Metadata{}, fmt.Errorf("decode metadata: %w", err)
	}
	if closing != json.Delim('}') {
		return Metadata{}, fmt.Errorf("metadata object is not closed")
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return Metadata{}, fmt.Errorf("metadata contains trailing JSON")
		}
		return Metadata{}, fmt.Errorf("decode trailing metadata: %w", err)
	}
	if !seen["description"] || !seen["protected"] {
		return Metadata{}, fmt.Errorf("metadata must contain description and protected")
	}
	if value.Description != nil {
		if !utf8.ValidString(*value.Description) || strings.ContainsRune(*value.Description, '\x00') {
			return Metadata{}, fmt.Errorf("description must be valid UTF-8 without NUL")
		}
		if len([]byte(*value.Description)) > maxDescriptionBytes {
			return Metadata{}, fmt.Errorf("description exceeds 4096 UTF-8 bytes")
		}
	}
	return value, nil
}

func Encode(value Metadata) ([]byte, error) {
	if value.Description != nil {
		if !utf8.ValidString(*value.Description) || strings.ContainsRune(*value.Description, '\x00') {
			return nil, fmt.Errorf("description must be valid UTF-8 without NUL")
		}
		if len([]byte(*value.Description)) > maxDescriptionBytes {
			return nil, fmt.Errorf("description exceeds 4096 UTF-8 bytes")
		}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	if len(data) > maxValueBytes {
		return nil, fmt.Errorf("metadata exceeds 16 KiB")
	}
	return data, nil
}

func Read(ctx context.Context, runner gitcli.Runner, worktreePath string) (ReadResult, error) {
	values, missing, err := gitcli.ConfigValues(ctx, runner, worktreePath, "--worktree", "gwm.metadata", false)
	if err != nil {
		return ReadResult{}, err
	}
	if missing {
		return ReadResult{Value: Default()}, nil
	}
	if len(values) != 1 {
		return ReadResult{Value: Default(), Present: true, Invalid: fmt.Errorf("gwm.metadata must have exactly one value")}, nil
	}
	value, decodeErr := Decode([]byte(values[0]))
	if decodeErr != nil {
		return ReadResult{Value: Default(), Present: true, Invalid: decodeErr}, nil
	}
	return ReadResult{Value: value, Present: true}, nil
}

func Write(ctx context.Context, runner gitcli.Runner, worktreePath string, intended Metadata) error {
	data, err := Encode(intended)
	if err != nil {
		return err
	}
	result := runner.Run(ctx, "-C", worktreePath, "config", "--worktree", "--replace-all", "gwm.metadata", string(data))
	after, readErr := Read(ctx, runner, worktreePath)
	if !result.Success() {
		return gitcli.ResultError("write gwm.metadata", result)
	}
	if readErr != nil {
		return fmt.Errorf("verify gwm.metadata: %w", readErr)
	}
	if after.Invalid != nil || !after.Present || !Equal(after.Value, intended) {
		return fmt.Errorf("gwm.metadata did not match the intended value after write")
	}
	return nil
}

func Equal(left, right Metadata) bool {
	if left.Protected != right.Protected {
		return false
	}
	if left.Description == nil || right.Description == nil {
		return left.Description == nil && right.Description == nil
	}
	return *left.Description == *right.Description
}

func Pointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
