package meta

import (
	"strings"
	"testing"
	"time"
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

func TestPointer(t *testing.T) {
	if Pointer("") != nil {
		t.Fatal("Pointer(empty) was not nil")
	}
	value := Pointer("text")
	if value == nil || *value != "text" {
		t.Fatalf("Pointer(text) = %#v", value)
	}
}
