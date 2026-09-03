package app

import "testing"

func TestParseAdd(t *testing.T) {
	options, err := parseAdd([]string{"tree", "-b", "topic", "--from", "HEAD", "--description", "work", "--protected"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Path != "tree" || options.NewBranch != "topic" || options.From != "HEAD" || options.Description != "work" || !options.Protected {
		t.Fatalf("unexpected options: %#v", options)
	}
	if _, err := parseAdd([]string{"tree", "-b", "topic", "--detach"}); err == nil {
		t.Fatal("mutually exclusive modes were accepted")
	}
	if _, err := parseAdd([]string{"tree", "--unknown"}); err == nil {
		t.Fatal("unknown option was accepted")
	}
}

func TestParseMetaRequiresExplicitBoolean(t *testing.T) {
	options, err := parseMeta([]string{"tree", "--protected", "false", "--description", ""})
	if err != nil {
		t.Fatal(err)
	}
	if !options.ProtectedProvided || options.Protected || !options.DescriptionProvided || options.Description != "" {
		t.Fatalf("unexpected options: %#v", options)
	}
	if _, err := parseMeta([]string{"tree", "--protected", "yes"}); err == nil {
		t.Fatal("non-boolean value was accepted")
	}
}

func TestParseRejectsInvalidText(t *testing.T) {
	if _, err := parseAdd([]string{"tree", "--description", "bad\x00text"}); err == nil {
		t.Fatal("NUL option value was accepted")
	}
	if _, err := parseRemove([]string{""}); err == nil {
		t.Fatal("empty path was accepted")
	}
}
