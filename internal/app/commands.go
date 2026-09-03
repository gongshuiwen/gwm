package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"gwm/internal/gitcli"
	"gwm/internal/hooks"
	"gwm/internal/meta"
)

func (a *App) init(ctx context.Context, repository *Repository) int {
	initialized, err := worktreeConfigEnabled(ctx, repository)
	if err != nil {
		return a.fail(err)
	}
	if initialized {
		fmt.Fprintln(a.Out, "GWM already initialized")
		return 0
	}
	for _, key := range []string{"core.worktree", "core.sparseCheckout", "core.sparseCheckoutCone"} {
		values, missing, err := gitcli.ConfigValues(ctx, repository.Git, repository.MainRoot, "--local", key, false)
		if err != nil {
			return a.fail(err)
		}
		if !missing && len(values) > 0 {
			return a.fail(fmt.Errorf("cannot initialize: common config contains %s", key))
		}
	}
	bareValues, missing, err := gitcli.ConfigValues(ctx, repository.Git, repository.MainRoot, "--local", "core.bare", true)
	if err != nil {
		return a.fail(err)
	}
	if !missing {
		for _, value := range bareValues {
			if value == "true" {
				return a.fail(fmt.Errorf("cannot initialize: common config contains core.bare=true"))
			}
		}
	}
	result := repository.RunCommon(ctx, "config", "--local", "--replace-all", "extensions.worktreeConfig", "true")
	afterInitialized, readErr := worktreeConfigEnabled(ctx, repository)
	if !result.Success() {
		return a.fail(gitcli.ResultError("enable extensions.worktreeConfig", result))
	}
	if readErr != nil {
		return a.fail(fmt.Errorf("verify extensions.worktreeConfig: %w", readErr))
	}
	if !afterInitialized {
		return a.fail(fmt.Errorf("extensions.worktreeConfig was not true after write"))
	}
	fmt.Fprintln(a.Out, "GWM initialized")
	return 0
}

func (a *App) list(ctx context.Context, repository *Repository) int {
	initialized, err := worktreeConfigEnabled(ctx, repository)
	if err != nil {
		return a.fail(err)
	}
	worktrees, err := repository.Worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	writer := tabwriter.NewWriter(a.Out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "PATH\tBRANCH\tDESCRIPTION\tPROTECTED")
	for _, worktree := range worktrees {
		branch := "-"
		if !worktree.Bare && !worktree.Detached && worktree.Branch != "" {
			branch = strings.TrimPrefix(worktree.Branch, "refs/heads/")
		}
		description := "-"
		protected := "false"
		if initialized {
			read, readErr := meta.Read(ctx, repository.Git, worktree.Path)
			if readErr != nil || read.Invalid != nil {
				description, protected = "INVALID", "INVALID"
			} else {
				if read.Value.Description != nil {
					description = escapeHuman(*read.Value.Description)
				}
				protected = strconv.FormatBool(read.Value.Protected)
			}
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", escapeHuman(worktree.Path), escapeHuman(branch), description, protected)
	}
	if err := writer.Flush(); err != nil {
		return a.fail(fmt.Errorf("write list output: %w", err))
	}
	return 0
}

func (a *App) add(ctx context.Context, repository *Repository, options addOptions) int {
	if err := requireInitialized(ctx, repository); err != nil {
		return a.fail(err)
	}
	target, err := normalizeWorktreePath(repository.Root, options.Path)
	if err != nil {
		return a.fail(err)
	}
	before, err := repository.Worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	if _, exists := findWorktree(before, target); exists {
		return a.fail(fmt.Errorf("worktree is already registered: %s", escapeHuman(target)))
	}
	requested := meta.Metadata{Description: meta.Pointer(options.Description), Protected: options.Protected}
	if _, err := meta.Encode(requested); err != nil {
		return a.fail(fmt.Errorf("invalid requested metadata: %w", err))
	}
	hookOptions := hooks.Options{
		NewBranch: optionalPointer(options.NewBranch, options.NewBranchProvided),
		From:      optionalPointer(options.From, options.FromProvided),
		Detach:    options.Detach,
	}
	prePayload := a.payload(repository, hooks.PreAdd, target, nil, requestedPointer(requested), hookOptions)
	if err := a.runHook(ctx, repository, prePayload); err != nil {
		return a.fail(err)
	}

	gitArgs := []string{"worktree", "add"}
	if options.NewBranchProvided {
		gitArgs = append(gitArgs, "-b", options.NewBranch)
	} else if options.Detach {
		gitArgs = append(gitArgs, "--detach")
	}
	gitArgs = append(gitArgs, target)
	if options.FromProvided {
		gitArgs = append(gitArgs, options.From)
	}
	result := repository.Run(ctx, gitArgs...)
	a.writeGitResult(result)
	after, listErr := repository.Worktrees(ctx)
	if listErr != nil {
		return a.fail(fmt.Errorf("git worktree add returned; read current worktree state: %w", listErr))
	}
	created, exists := findWorktree(after, target)
	if !result.Success() || !exists {
		if !result.Success() {
			return a.fail(fmt.Errorf("git worktree add failed; target registered after command: %t", exists))
		}
		return a.fail(fmt.Errorf("git worktree add exited successfully but target is not registered"))
	}

	metadataErr := meta.Write(ctx, repository.Git, created.Path, requested)
	observed := observedMetadata(ctx, repository.Git, created.Path)
	postPayload := a.payload(repository, hooks.PostAdd, target, &created, observed, hookOptions)
	postErr := a.runHook(ctx, repository, postPayload)
	if metadataErr != nil || postErr != nil {
		return a.partial("add", metadataErr, postErr)
	}
	fmt.Fprintf(a.Out, "Added worktree %s\n", escapeHuman(target))
	return 0
}

func (a *App) metadata(ctx context.Context, repository *Repository, options metaOptions) int {
	if err := requireInitialized(ctx, repository); err != nil {
		return a.fail(err)
	}
	target, err := normalizeWorktreePath(repository.Root, options.Path)
	if err != nil {
		return a.fail(err)
	}
	worktrees, err := repository.Worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	worktree, exists := findWorktree(worktrees, target)
	if !exists {
		return a.fail(fmt.Errorf("worktree is not registered: %s", escapeHuman(target)))
	}
	read, err := meta.Read(ctx, repository.Git, worktree.Path)
	if err != nil {
		return a.fail(err)
	}
	if read.Invalid != nil {
		return a.fail(fmt.Errorf("invalid gwm.metadata for %s: %w", escapeHuman(target), read.Invalid))
	}
	if !options.DescriptionProvided && !options.ProtectedProvided {
		description := "-"
		if read.Value.Description != nil {
			description = escapeHuman(*read.Value.Description)
		}
		fmt.Fprintf(a.Out, "DESCRIPTION\t%s\nPROTECTED\t%t\n", description, read.Value.Protected)
		return 0
	}
	intended := read.Value
	if options.DescriptionProvided {
		intended.Description = meta.Pointer(options.Description)
	}
	if options.ProtectedProvided {
		intended.Protected = options.Protected
	}
	if meta.Equal(read.Value, intended) {
		fmt.Fprintln(a.Out, "Metadata unchanged")
		return 0
	}
	if err := meta.Write(ctx, repository.Git, worktree.Path, intended); err != nil {
		return a.fail(err)
	}
	fmt.Fprintf(a.Out, "Updated metadata for %s\n", escapeHuman(target))
	return 0
}

func (a *App) remove(ctx context.Context, repository *Repository, options removeOptions) int {
	if err := requireInitialized(ctx, repository); err != nil {
		return a.fail(err)
	}
	target, err := normalizeWorktreePath(repository.Root, options.Path)
	if err != nil {
		return a.fail(err)
	}
	before, err := repository.Worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	worktree, exists := findWorktree(before, target)
	if !exists {
		return a.fail(fmt.Errorf("worktree is not registered: %s", escapeHuman(target)))
	}
	if worktree.IsMain || worktree.Bare || worktree.Locked {
		return a.fail(fmt.Errorf("GWM remove accepts only unlocked linked worktrees"))
	}
	read, err := meta.Read(ctx, repository.Git, worktree.Path)
	if err != nil {
		return a.fail(err)
	}
	if read.Invalid != nil {
		return a.fail(fmt.Errorf("invalid gwm.metadata for %s: %w", escapeHuman(target), read.Invalid))
	}
	if read.Value.Protected {
		return a.fail(fmt.Errorf("worktree is protected; set --protected false before removing it"))
	}
	hookOptions := hooks.Options{Force: options.Force}
	prePayload := a.payload(repository, hooks.PreRemove, target, &worktree, requestedPointer(read.Value), hookOptions)
	if err := a.runHook(ctx, repository, prePayload); err != nil {
		return a.fail(err)
	}

	gitArgs := []string{"worktree", "remove"}
	if options.Force {
		gitArgs = append(gitArgs, "--force")
	}
	gitArgs = append(gitArgs, target)
	result := repository.Run(ctx, gitArgs...)
	a.writeGitResult(result)
	after, listErr := repository.Worktrees(ctx)
	if listErr != nil {
		return a.fail(fmt.Errorf("git worktree remove returned; read current worktree state: %w", listErr))
	}
	_, remains := findWorktree(after, target)
	if !result.Success() || remains {
		if !result.Success() {
			return a.fail(fmt.Errorf("git worktree remove failed; target registered after command: %t", remains))
		}
		return a.fail(fmt.Errorf("git worktree remove exited successfully but target remains registered"))
	}
	postPayload := a.payload(repository, hooks.PostRemove, target, &worktree, requestedPointer(read.Value), hookOptions)
	if err := a.runHook(ctx, repository, postPayload); err != nil {
		return a.partial("remove", err)
	}
	fmt.Fprintf(a.Out, "Removed worktree %s\n", escapeHuman(target))
	return 0
}

func worktreeConfigEnabled(ctx context.Context, repository *Repository) (bool, error) {
	values, missing, err := gitcli.ConfigValues(ctx, repository.Git, repository.MainRoot, "--local", "extensions.worktreeConfig", true)
	if err != nil {
		return false, err
	}
	if missing {
		return false, nil
	}
	if len(values) != 1 {
		return false, fmt.Errorf("extensions.worktreeConfig must have exactly one value")
	}
	return values[0] == "true", nil
}

func requireInitialized(ctx context.Context, repository *Repository) error {
	initialized, err := worktreeConfigEnabled(ctx, repository)
	if err != nil {
		return err
	}
	if !initialized {
		return fmt.Errorf("repository is not initialized; run gwm init first")
	}
	return nil
}

func (a *App) runHook(ctx context.Context, repository *Repository, payload hooks.Payload) error {
	path, configured, err := hooks.ConfiguredPath(ctx, repository.Git, repository.MainRoot, payload.Event)
	if err != nil {
		return err
	}
	if !configured {
		return nil
	}
	return a.Hooks.Run(ctx, path, repository.Root, payload, a.Out, a.Err)
}

func (a *App) payload(repository *Repository, event, path string, worktree *Worktree, metadata *meta.Metadata, options hooks.Options) hooks.Payload {
	payload := hooks.Payload{
		SchemaVersion:  1,
		Event:          event,
		CommonDir:      repository.CommonDir,
		InvocationRoot: repository.Root,
		WorktreePath:   path,
		Metadata:       metadata,
		Options:        options,
	}
	if worktree != nil {
		payload.Head = optionalPointer(worktree.Head, worktree.Head != "")
		payload.Branch = optionalPointer(worktree.Branch, worktree.Branch != "" && !worktree.Detached)
	}
	return payload
}

func observedMetadata(ctx context.Context, runner gitcli.Runner, path string) *meta.Metadata {
	read, err := meta.Read(ctx, runner, path)
	if err != nil || read.Invalid != nil {
		return nil
	}
	return requestedPointer(read.Value)
}

func requestedPointer(value meta.Metadata) *meta.Metadata {
	copy := value
	return &copy
}

func optionalPointer(value string, provided bool) *string {
	if !provided {
		return nil
	}
	copy := value
	return &copy
}

func (a *App) writeGitResult(result gitcli.Result) {
	if len(result.Stdout) > 0 {
		_, _ = a.Out.Write(result.Stdout)
	}
	if len(result.Stderr) > 0 {
		_, _ = a.Err.Write(result.Stderr)
	}
}

func escapeHuman(value string) string {
	quoted := strconv.QuoteToGraphic(value)
	unquoted := strings.TrimSuffix(strings.TrimPrefix(quoted, "\""), "\"")
	if unquoted == "" {
		return `""`
	}
	return unquoted
}
