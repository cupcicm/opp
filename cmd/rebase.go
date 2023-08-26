package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/cupcicm/opp/git"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

func RebaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rebase",
		Aliases: []string{"reb", "rebase"},
		Short:   "rebase the current branch and dependent PRs if needed.",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var repo = git.Current()
			pr, headIsAPr := repo.PrForHead()
			if !headIsAPr {
				fmt.Println("You can run rebase only on local pr branches")
				return nil
			}
			if err := repo.Fetch(cmd.Context()); err != nil {
				return fmt.Errorf("error during fetch: %w", err)
			}
			if !repo.CanRebase() {
				return errors.New("there are uncommitted changes. Cannot run rebase")
			}
			_, err := rebase(cmd.Context(), repo, pr, true)
			repo.Checkout(pr)
			return err
		},
	}
	return cmd
}

func rebase(ctx context.Context, repo *git.Repo, pr *git.LocalPr, first bool) (git.Branch, error) {
	ancestor, err := pr.GetAncestor()
	var baseBranch git.Branch = ancestor
	if err != nil {
		return nil, err
	}
	if ancestor.IsPr() {
		baseBranch, err = rebase(ctx, repo, ancestor.(*git.LocalPr), false)
		if err != nil {
			return nil, err
		}
	}
	_, err = repo.GetLocalTip(pr)
	if err == plumbing.ErrReferenceNotFound {
		// The branch has been merged and deleted.
		repo.CleanupAfterMerge(pr)
	}

	if !first {
		fmt.Printf("Rebasing dependent PR %s...\n", pr.LocalBranch())
	} else {
		fmt.Println()
	}
	if err := repo.Checkout(pr); err != nil {
		return nil, fmt.Errorf("error during checkout: %w", err)
	}
	if err := repo.Rebase(ctx, ancestor); err != nil {
		return nil, fmt.Errorf("error during rebase: %w", err)
	}
	if !ancestor.IsPr() {
		remoteBaseBranchTip := git.Must(repo.GetRemoteTip(ancestor))
		localPrTip := git.Must(repo.GetLocalTip(pr))
		if git.Must(localPrTip.IsAncestor(remoteBaseBranchTip)) {
			// PR has been merged : the local branch is now part
			// of the history of the main branch.
			repo.CleanupAfterMerge(pr)
		}
	}

	return baseBranch, nil
}
