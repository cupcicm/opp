package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cupcicm/opp/core"
	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

type merger struct {
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

func MergeCommand(repo *core.Repo, gh core.GhPullRequest) *cobra.Command {
	mergeDeps := false
	cmd := &cobra.Command{
		Use:     "merge",
		Aliases: []string{"m"},
		Args:    cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			var head = core.Must(repo.Head())
			pr, found := repo.PrForHead()
			if !head.Name().IsBranch() || !found {
				return errors.New("you need to be on a PR branch to run opp merge")
			}
			merger := merger{Repo: repo, PullRequests: gh}
			ancestors := pr.AllAncestors()
			mergeable, reason := merger.IsMergeable(cmd.Context(), pr, ancestors)
			if !mergeable {
				return reason
			}
			if len(ancestors) >= 1 && !mergeDeps {
				return errors.New("use --deps to merge dependent PRs before merging this one")
			}
			merger.Merge(cmd.Context(), ancestors...)
			merger.Merge(cmd.Context(), pr)

			// merge !
			return nil
		},
	}
	cmd.Flags().BoolVar(&mergeDeps, "deps", false, "Pass true to merge dependent PRs if needed")
	return cmd
}

// If IsMergeable returns true, err will be nil but if false err will
// be non-nil.
func (m *merger) IsMergeable(ctx context.Context, pr *core.LocalPr, ancestors []*core.LocalPr) (bool, error) {

	overallMergeable, err := m.isMergeable(ctx, pr)
	if !overallMergeable {
		return false, fmt.Errorf("%s is not mergeable (❌ - %s)", pr.Url(), err)
	}
	if len(ancestors) > 0 {
		fmt.Printf("%s is mergeable (✅). It depends on \n", pr.Url())
		for _, pr := range ancestors {
			mergeable, err := m.isMergeable(ctx, pr)
			var details string
			if mergeable {
				details = "(✅)"
			} else {
				details = fmt.Sprintf("(❌ - %s)", err)
				overallMergeable = false
			}
			fmt.Printf("  - %s %s\n", pr.Url(), details)
		}
	} else {
		fmt.Printf("%s is mergeable (✅).\n", pr.Url())
	}
	if !overallMergeable {
		return false, errors.New("some dependent PRs are unmergeable")
	}
	return true, nil
}

// Is this PR, separately from its ancestor, mergeable in itself ?
func (m *merger) isMergeable(ctx context.Context, pr *core.LocalPr) (bool, error) {
	githubPr, _, err := m.PullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
	if err != nil {
		return false, err
	}
	if githubPr.Mergeable == nil {
		return false, fmt.Errorf("%s is still being checked by github. Retry soon", pr.Url())
	}
	if *githubPr.Mergeable {
		return true, nil
	}
	switch *githubPr.MergeableState {
	case "dirty":
		return false, fmt.Errorf("cannot be merged cleanly into %s", m.Repo.BaseBranch().RemoteName())
	case "blocked":
		return false, errors.New("has some failing checks")
	default:
		return false, errors.New("cannot be merged right now")
	}
}

func (m *merger) Merge(ctx context.Context, prs ...*core.LocalPr) error {
	for _, pr := range prs {
		tip, err := m.Repo.GetLocalTip(pr)
		if err != nil {
			return err
		}
		fmt.Printf("Merging %s... ", pr.LocalBranch())
		_, r, err := m.PullRequests.Merge(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber, "",
			&github.PullRequestOptions{
				SHA:         tip.Hash.String(),
				MergeMethod: core.GetGithubMergeMethod(),
			})
		if r != nil && r.StatusCode == http.StatusConflict {
			fmt.Println("(❌ - wrong remote tip)")
			return fmt.Errorf("did not merge %s", pr.LocalBranch())
		}
		if err == nil {
			fmt.Printf("(✅)\n")
			m.Repo.CleanupAfterMerge(ctx, pr)
		} else {
			fmt.Printf("(❌ - %s)\n", err)
		}
	}
	return nil
}
