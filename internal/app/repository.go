package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"gwm/internal/gitcli"
)

type Repository struct {
	Root      string
	MainRoot  string
	CommonDir string
	Git       gitcli.Runner
}

type Worktree struct {
	Path     string
	Head     string
	Branch   string
	IsMain   bool
	Bare     bool
	Detached bool
	Locked   bool
}

func Discover(ctx context.Context, runner gitcli.Runner, start string) (*Repository, error) {
	if err := requireGitVersion(ctx, runner); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, fmt.Errorf("resolve repository path: %w", err)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(abs); resolveErr == nil {
		abs = resolved
	}
	rootResult := runner.Run(ctx, "-C", abs, "rev-parse", "--show-toplevel")
	if !rootResult.Success() {
		return nil, gitcli.ResultError("discover non-bare repository", rootResult)
	}
	root := strings.TrimSpace(string(rootResult.Stdout))
	if !utf8.ValidString(root) || root == "" {
		return nil, fmt.Errorf("repository root is not valid UTF-8")
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	commonResult := runner.Run(ctx, "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !commonResult.Success() {
		return nil, gitcli.ResultError("discover common directory", commonResult)
	}
	commonDir := strings.TrimSpace(string(commonResult.Stdout))
	if !utf8.ValidString(commonDir) || commonDir == "" {
		return nil, fmt.Errorf("common directory is not valid UTF-8")
	}
	commonDir = filepath.Clean(commonDir)
	bareResult := runner.Run(ctx, "-C", root, "rev-parse", "--is-bare-repository")
	if !bareResult.Success() {
		return nil, gitcli.ResultError("check bare repository", bareResult)
	}
	bare, err := strconv.ParseBool(strings.TrimSpace(string(bareResult.Stdout)))
	if err != nil || bare {
		return nil, fmt.Errorf("GWM supports only ordinary non-bare repositories")
	}
	repository := &Repository{Root: filepath.Clean(root), CommonDir: commonDir, Git: runner}
	worktrees, err := repository.worktreesAt(ctx, repository.Root)
	if err != nil {
		return nil, err
	}
	if len(worktrees) == 0 || worktrees[0].Path == "" {
		return nil, fmt.Errorf("repository has no main worktree")
	}
	repository.MainRoot = worktrees[0].Path
	return repository, nil
}

func requireGitVersion(ctx context.Context, runner gitcli.Runner) error {
	result := runner.Run(ctx, "--version")
	if !result.Success() {
		return gitcli.ResultError("run git --version", result)
	}
	var major, minor int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(result.Stdout)), "git version %d.%d", &major, &minor); err != nil {
		return fmt.Errorf("cannot parse Git version %q", strings.TrimSpace(string(result.Stdout)))
	}
	if major < 2 || (major == 2 && minor < 39) {
		return fmt.Errorf("Git 2.39 or newer is required")
	}
	return nil
}

func (r *Repository) Run(ctx context.Context, args ...string) gitcli.Result {
	return r.runAt(ctx, r.Root, args...)
}

func (r *Repository) RunCommon(ctx context.Context, args ...string) gitcli.Result {
	return r.runAt(ctx, r.MainRoot, args...)
}

func (r *Repository) runAt(ctx context.Context, root string, args ...string) gitcli.Result {
	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", root)
	full = append(full, args...)
	return r.Git.Run(ctx, full...)
}

func (r *Repository) Worktrees(ctx context.Context) ([]Worktree, error) {
	return r.worktreesAt(ctx, r.MainRoot)
}

func (r *Repository) worktreesAt(ctx context.Context, root string) ([]Worktree, error) {
	result := r.runAt(ctx, root, "worktree", "list", "--porcelain", "-z")
	if !result.Success() {
		return nil, gitcli.ResultError("list worktrees", result)
	}
	return parseWorktrees(result.Stdout)
}

func parseWorktrees(data []byte) ([]Worktree, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("worktree list contains non-UTF-8 paths or attributes")
	}
	parts := strings.Split(string(data), "\x00")
	worktrees := make([]Worktree, 0)
	var current *Worktree
	flush := func() error {
		if current == nil {
			return nil
		}
		if current.Path == "" {
			return fmt.Errorf("worktree record has no path")
		}
		current.IsMain = len(worktrees) == 0
		worktrees = append(worktrees, *current)
		current = nil
		return nil
	}
	for _, part := range parts {
		if part == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		key, value, hasValue := strings.Cut(part, " ")
		if key == "worktree" {
			if err := flush(); err != nil {
				return nil, err
			}
			if !hasValue || value == "" {
				return nil, fmt.Errorf("worktree attribute has no path")
			}
			current = &Worktree{Path: filepath.Clean(value)}
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("worktree attribute %q appears before a worktree path", key)
		}
		switch key {
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = value
		case "bare":
			current.Bare = true
		case "detached":
			current.Detached = true
		case "locked":
			current.Locked = true
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return worktrees, nil
}

func findWorktree(worktrees []Worktree, path string) (Worktree, bool) {
	clean := filepath.Clean(path)
	for _, worktree := range worktrees {
		if filepath.Clean(worktree.Path) == clean {
			return worktree, true
		}
	}
	return Worktree{}, false
}

func normalizeWorktreePath(base, input string) (string, error) {
	if input == "" || !utf8.ValidString(input) || strings.ContainsRune(input, '\x00') {
		return "", fmt.Errorf("worktree path must be non-empty UTF-8 without NUL")
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve worktree path: %w", err)
	}
	abs = filepath.Clean(abs)
	if resolved, resolveErr := filepath.EvalSymlinks(abs); resolveErr == nil {
		return filepath.Clean(resolved), nil
	}
	parent, name := filepath.Split(abs)
	resolvedParent, err := filepath.EvalSymlinks(filepath.Clean(parent))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("worktree parent directory does not exist")
		}
		return "", fmt.Errorf("resolve worktree parent: %w", err)
	}
	return filepath.Join(resolvedParent, name), nil
}
