package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v2"
)

func RebaseCommand(repo *core.Repo) *cli.Command {
	cmd := &cli.Command{
		Name:    "rebase",
		Aliases: []string{"reb", "r", "pull"},
		Usage:   "rebase the current branch and dependent PRs if needed.",
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() > 0 {
				return errors.New("too many arguments")
			}
			pr, headIsAPr := repo.PrForHead()
			if !headIsAPr {
				return cli.Exit("You can run rebase only on local pr branches", 1)
			}
			if err := repo.Fetch(cCtx.Context); err != nil {
				return cli.Exit(fmt.Errorf("error during fetch: %w", err), 1)
			}
			if !repo.NoLocalChanges(cCtx.Context) {
				return cli.Exit("there are uncommitted changes. Cannot run rebase", 1)
			}
			hasBeenMerged, err := rebase(cCtx.Context, repo, pr, true)
			if err != nil {
				return err
			}
			if hasBeenMerged {
				repo.Checkout(repo.BaseBranch())
			} else {
				repo.Checkout(pr)
			}
			return nil
		},
	}
	return cmd
}

// Return true when the current PR has been merged and does not actually exist anymore.
func rebase(ctx context.Context, repo *core.Repo, pr *core.LocalPr, first bool) (bool, error) {
	_, err := repo.GetLocalTip(pr)
	if err == plumbing.ErrReferenceNotFound {
		// The branch has been merged and deleted.
		repo.CleanupAfterMerge(ctx, pr)
		return true, nil
	}

	ancestor, err := pr.GetAncestor()
	if err != nil {
		return false, cli.Exit(
			fmt.Errorf(".opp/state/pr/%d is invalid, not sure what to rebase on", pr.PrNumber), 1)
	}

	if ancestor.IsPr() {
		return rebaseOnDependentPr(ctx, repo, pr, ancestor.(*core.LocalPr), first)
	} else {
		return rebaseOnBaseBranch(ctx, repo, pr, first)
	}
}

// Return true when the current PR has been merged and does not actually exist anymore.
func rebaseOnBaseBranch(ctx context.Context, repo *core.Repo, pr *core.LocalPr, first bool) (bool, error) {
	if !first {
		fmt.Printf("Rebasing dependent PR %s...\n", pr.LocalBranch())
	}

	if err := repo.Checkout(pr); err != nil {
		return false, fmt.Errorf("error during checkout: %w", err)
	}
	if err := repo.Rebase(ctx, repo.BaseBranch(), true); err != nil {
		return false, fmt.Errorf("error during rebase: %w", err)
	}
	remoteBaseBranchTip := core.Must(repo.GetRemoteTip(repo.BaseBranch()))
	localPrTip := core.Must(repo.GetLocalTip(pr))
	pr.RememberCurrentTip()
	if core.Must(localPrTip.IsAncestor(remoteBaseBranchTip)) {
		// PR has been merged : the local branch is now part
		// of the history of the main branch.
		repo.CleanupAfterMerge(ctx, pr)
		return true, nil
	}
	return false, nil
}

// The strategy here is: try to rebase silently.
// If it works, great. If not, run an interactive rebase because
// if the dependant branch has been modified git doesn't know where
// the current PR starts exactly. The user will know.
// Return true when the current PR has been merged and does not actually exist anymore.
func rebaseOnDependentPr(ctx context.Context, repo *core.Repo, pr *core.LocalPr, ancestor *core.LocalPr, first bool) (bool, error) {
	hasBeenMerged, err := rebase(ctx, repo, ancestor, false)
	if err != nil {
		return false, err
	}
	if hasBeenMerged {
		pr.ReloadState()
		return rebaseOnBaseBranch(ctx, repo, pr, first)
	}

	if !first {
		fmt.Printf("Rebasing dependent PR %s...\n", pr.LocalBranch())
	} else {
		fmt.Println()
	}
	if err := repo.Checkout(pr); err != nil {
		return false, fmt.Errorf("error during checkout: %w", err)
	}
	// Try to rebase silently once.
	if !repo.TryRebaseCurrentBranchSilently(ctx, ancestor) {
		fmt.Printf("%s cannot be cleanly rebased on top of %s.\n", pr.LocalBranch(), ancestor.LocalBranch())
		fmt.Printf("This usually happens when you modified (e.g. amended) some commits in %s.\n", ancestor.LocalBranch())
		fmt.Printf("Here is an editor window where you need to pick only the commits in %s.\n", pr.LocalBranch())
		fmt.Printf("Please delete all lines that represent commits in %s\n", ancestor.LocalBranch())
		err := repo.InteractiveRebase(ctx, ancestor)
		if err != nil {
			return false, errors.New("please finish the interactive rebase then re-run")
		}
	}
	pr.RememberCurrentTip()
	return false, nil
}
