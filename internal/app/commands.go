package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gongshuiwen/gwm/internal/gitcli"
	"github.com/gongshuiwen/gwm/internal/hooks"
	"github.com/gongshuiwen/gwm/internal/meta"
)

const worktreeConfigKey = "extensions.worktreeConfig"

func (a *App) init(ctx context.Context, repository *repositoryContext) int {
	initialized, err := worktreeConfigEnabled(ctx, repository)
	if err != nil {
		return a.fail(err)
	}
	if initialized {
		fmt.Fprintln(a.stdout, "GWM already initialized")
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
				return a.fail(errors.New("cannot initialize: common config contains core.bare=true"))
			}
		}
	}
	result := repository.runCommon(ctx, "config", "--local", "--replace-all", worktreeConfigKey, "true")
	afterInitialized, readErr := worktreeConfigEnabled(ctx, repository)
	if !result.Success() {
		return a.fail(gitcli.ResultError("enable "+worktreeConfigKey, result))
	}
	if readErr != nil {
		return a.fail(fmt.Errorf("verify %s: %w", worktreeConfigKey, readErr))
	}
	if !afterInitialized {
		return a.fail(fmt.Errorf("%s was not true after write", worktreeConfigKey))
	}
	fmt.Fprintln(a.stdout, "GWM initialized")
	return 0
}

func (a *App) list(ctx context.Context, repository *repositoryContext) int {
	initialized, err := worktreeConfigEnabled(ctx, repository)
	if err != nil {
		return a.fail(err)
	}
	worktrees, err := repository.worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	writer := tabwriter.NewWriter(a.stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "PATH\tBRANCH\tDESCRIPTION\tPROTECTED\tCREATED_AT")
	for _, worktree := range worktrees {
		branch := "-"
		if !worktree.Bare && !worktree.Detached && worktree.Branch != "" {
			branch = strings.TrimPrefix(worktree.Branch, "refs/heads/")
		}
		description := "-"
		protected := "false"
		createdAt := "-"
		if initialized {
			metadata, readErr := meta.Read(ctx, repository.Git, worktree.Path)
			if readErr != nil {
				description, protected = "INVALID", "INVALID"
			} else {
				if metadata.Description != nil {
					description = escapeHuman(*metadata.Description)
				}
				protected = strconv.FormatBool(metadata.Protected)
				createdAt = displayCreatedAt(metadata)
			}
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", escapeHuman(worktree.Path), escapeHuman(branch), description, protected, createdAt)
	}
	if err := writer.Flush(); err != nil {
		return a.fail(fmt.Errorf("write list output: %w", err))
	}
	return 0
}

func (a *App) add(ctx context.Context, repository *repositoryContext, options addOptions) int {
	if err := requireInitialized(ctx, repository); err != nil {
		return a.fail(err)
	}
	target, err := normalizeWorktreePath(repository.Root, options.Path)
	if err != nil {
		return a.fail(err)
	}
	before, err := repository.worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	if _, exists := findWorktree(before, target); exists {
		return a.fail(fmt.Errorf("worktree is already registered: %s", escapeHuman(target)))
	}
	requested := meta.Metadata{Description: meta.DescriptionPointer(options.Description), Protected: options.Protected}
	if err := meta.Validate(requested); err != nil {
		return a.fail(fmt.Errorf("invalid requested metadata: %w", err))
	}
	hookOptions := hooks.Options{
		NewBranch: optionalPointer(options.NewBranch, options.NewBranchProvided),
		From:      optionalPointer(options.From, options.FromProvided),
		Detach:    options.Detach,
	}
	prePayload := a.payload(repository, hooks.PreAdd, target, nil, metadataPointer(requested), hookOptions)
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
	result := repository.run(ctx, gitArgs...)
	a.writeGitResult(result)
	after, listErr := repository.worktrees(ctx)
	if listErr != nil {
		return a.fail(fmt.Errorf("git worktree add returned; read current worktree state: %w", listErr))
	}
	created, exists := findWorktree(after, target)
	if !result.Success() || !exists {
		if !result.Success() {
			return a.fail(fmt.Errorf("git worktree add failed; target registered after command: %t", exists))
		}
		return a.fail(errors.New("git worktree add exited successfully but target is not registered"))
	}

	createdAt := meta.FormatCreatedAt(time.Now())
	metadataErr := meta.Write(ctx, repository.Git, created.Path, requested)
	if metadataErr == nil {
		metadataErr = meta.WriteCreatedAt(ctx, repository.Git, created.Path, createdAt)
	}
	observed := observedMetadata(ctx, repository.Git, created.Path)
	postPayload := a.payload(repository, hooks.PostAdd, target, &created, observed, hookOptions)
	postErr := a.runHook(ctx, repository, postPayload)
	if metadataErr != nil || postErr != nil {
		return a.partial("add", metadataErr, postErr)
	}
	fmt.Fprintf(a.stdout, "Added worktree %s\n", escapeHuman(target))
	return 0
}

func (a *App) metadata(ctx context.Context, repository *repositoryContext, options metaOptions) int {
	if err := requireInitialized(ctx, repository); err != nil {
		return a.fail(err)
	}
	target, err := normalizeWorktreePath(repository.Root, options.Path)
	if err != nil {
		return a.fail(err)
	}
	worktrees, err := repository.worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	worktree, exists := findWorktree(worktrees, target)
	if !exists {
		return a.fail(fmt.Errorf("worktree is not registered: %s", escapeHuman(target)))
	}
	current, err := meta.Read(ctx, repository.Git, worktree.Path)
	if err != nil {
		return a.fail(fmt.Errorf("read worktree metadata for %s: %w", escapeHuman(target), err))
	}
	if !options.DescriptionProvided && !options.ProtectedProvided {
		description := "-"
		if current.Description != nil {
			description = escapeHuman(*current.Description)
		}
		fmt.Fprintf(a.stdout, "DESCRIPTION\t%s\nPROTECTED\t%t\nCREATED_AT\t%s\n", description, current.Protected, displayCreatedAt(current))
		return 0
	}
	intended := current
	if options.DescriptionProvided {
		intended.Description = meta.DescriptionPointer(options.Description)
	}
	if options.ProtectedProvided {
		intended.Protected = options.Protected
	}
	if meta.EqualEditable(current, intended) {
		fmt.Fprintln(a.stdout, "Metadata unchanged")
		return 0
	}
	if err := meta.Write(ctx, repository.Git, worktree.Path, intended); err != nil {
		return a.fail(err)
	}
	fmt.Fprintf(a.stdout, "Updated metadata for %s\n", escapeHuman(target))
	return 0
}

func (a *App) remove(ctx context.Context, repository *repositoryContext, options removeOptions) int {
	if err := requireInitialized(ctx, repository); err != nil {
		return a.fail(err)
	}
	target, err := normalizeWorktreePath(repository.Root, options.Path)
	if err != nil {
		return a.fail(err)
	}
	before, err := repository.worktrees(ctx)
	if err != nil {
		return a.fail(err)
	}
	worktree, exists := findWorktree(before, target)
	if !exists {
		return a.fail(fmt.Errorf("worktree is not registered: %s", escapeHuman(target)))
	}
	if worktree.IsMain || worktree.Bare || worktree.Locked {
		return a.fail(errors.New("GWM remove accepts only unlocked linked worktrees"))
	}
	metadata, err := meta.Read(ctx, repository.Git, worktree.Path)
	if err != nil {
		return a.fail(fmt.Errorf("read worktree metadata for %s: %w", escapeHuman(target), err))
	}
	if metadata.Protected {
		return a.fail(errors.New("worktree is protected; set --protected false before removing it"))
	}
	hookOptions := hooks.Options{Force: options.Force}
	prePayload := a.payload(repository, hooks.PreRemove, target, &worktree, metadataPointer(metadata), hookOptions)
	if err := a.runHook(ctx, repository, prePayload); err != nil {
		return a.fail(err)
	}

	gitArgs := []string{"worktree", "remove"}
	if options.Force {
		gitArgs = append(gitArgs, "--force")
	}
	gitArgs = append(gitArgs, target)
	result := repository.run(ctx, gitArgs...)
	a.writeGitResult(result)
	after, listErr := repository.worktrees(ctx)
	if listErr != nil {
		return a.fail(fmt.Errorf("git worktree remove returned; read current worktree state: %w", listErr))
	}
	_, remains := findWorktree(after, target)
	if !result.Success() || remains {
		if !result.Success() {
			return a.fail(fmt.Errorf("git worktree remove failed; target registered after command: %t", remains))
		}
		return a.fail(errors.New("git worktree remove exited successfully but target remains registered"))
	}
	postPayload := a.payload(repository, hooks.PostRemove, target, &worktree, metadataPointer(metadata), hookOptions)
	if err := a.runHook(ctx, repository, postPayload); err != nil {
		return a.partial("remove", err)
	}
	fmt.Fprintf(a.stdout, "Removed worktree %s\n", escapeHuman(target))
	return 0
}

func worktreeConfigEnabled(ctx context.Context, repository *repositoryContext) (bool, error) {
	values, missing, err := gitcli.ConfigValues(ctx, repository.Git, repository.MainRoot, "--local", worktreeConfigKey, true)
	if err != nil {
		return false, err
	}
	if missing {
		return false, nil
	}
	if len(values) != 1 {
		return false, fmt.Errorf("%s must have exactly one value", worktreeConfigKey)
	}
	return values[0] == "true", nil
}

func requireInitialized(ctx context.Context, repository *repositoryContext) error {
	initialized, err := worktreeConfigEnabled(ctx, repository)
	if err != nil {
		return err
	}
	if !initialized {
		return errors.New("repository is not initialized; run gwm init first")
	}
	return nil
}

func (a *App) runHook(ctx context.Context, repository *repositoryContext, payload hooks.Payload) error {
	path, configured, err := hooks.ConfiguredPath(ctx, repository.Git, repository.MainRoot, payload.Event)
	if err != nil {
		return err
	}
	if !configured {
		return nil
	}
	return a.hooks.Run(ctx, path, repository.Root, payload, a.stdout, a.stderr)
}

func (a *App) payload(repository *repositoryContext, event, path string, worktree *worktree, metadata *meta.Metadata, options hooks.Options) hooks.Payload {
	payload := hooks.Payload{
		SchemaVersion:  hooks.SchemaVersion,
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
	metadata, err := meta.Read(ctx, runner, path)
	if err != nil {
		return nil
	}
	return metadataPointer(metadata)
}

func metadataPointer(value meta.Metadata) *meta.Metadata {
	return &value
}

func displayCreatedAt(metadata meta.Metadata) string {
	if metadata.CreatedAtInvalid {
		return "INVALID"
	}
	if metadata.CreatedAt == nil {
		return "-"
	}
	return *metadata.CreatedAt
}

func optionalPointer(value string, provided bool) *string {
	if !provided {
		return nil
	}
	return &value
}

func (a *App) writeGitResult(result gitcli.Result) {
	if len(result.Stdout) > 0 {
		_, _ = a.stdout.Write(result.Stdout)
	}
	if len(result.Stderr) > 0 {
		_, _ = a.stderr.Write(result.Stderr)
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
