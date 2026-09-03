package app

import "testing"

func TestParseWorktrees(t *testing.T) {
	data := []byte("worktree /repo\x00HEAD abc\x00branch refs/heads/main\x00\x00" +
		"worktree /repo feature\x00HEAD def\x00detached\x00locked reason here\x00unknown value\x00\x00")
	got, err := parseWorktrees(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(got))
	}
	if !got[0].IsMain || got[0].Branch != "refs/heads/main" {
		t.Fatalf("unexpected main worktree: %#v", got[0])
	}
	if got[1].Path != "/repo feature" || !got[1].Detached || !got[1].Locked {
		t.Fatalf("unexpected linked worktree: %#v", got[1])
	}
}
