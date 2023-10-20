package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v56/github"
	"github.com/urfave/cli/v2"
)

type merger struct {
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

func MergeCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:    "merge",
		Aliases: []string{"m"},
		Action: func(cCtx *cli.Context) error {
			pr, mergingCurrentBranch, err := PrFromFirstArgument(repo, cCtx)
			if err != nil {
				return err
			}
			merger := merger{Repo: repo, PullRequests: gh(cCtx.Context).PullRequests()}
			ancestors := pr.AllAncestors()
			mergeContext, cancel := context.WithTimeoutCause(
				cCtx.Context, core.GetGithubTimeout(),
				fmt.Errorf("merging too slow, increase github.timeout"),
			)
			defer cancel()
			if len(ancestors) >= 1 {
				fmt.Printf("%s is not mergeable because it has unmerged dependent PRs.\n", pr.Url())
				return fmt.Errorf("please merge %s first", ancestors[0].LocalBranch())
			}
			isMergeable, err := merger.IsMergeable(mergeContext, pr)
			if !isMergeable {
				return cli.Exit(err, 1)
			}
			if err := merger.Merge(mergeContext, pr); err != nil {
				return cli.Exit("could not merge", 1)
			}
			if mergingCurrentBranch {
				repo.Checkout(repo.BaseBranch())
			}
			repo.CleanupAfterMerge(mergeContext, pr)
			return nil
		},
	}
	return cmd
}

// Is this PR, separately from its ancestor, mergeable in itself ?
func (m *merger) IsMergeable(ctx context.Context, pr *core.LocalPr) (bool, error) {
	githubPr, _, err := m.PullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
	if err != nil {
		return false, err
	}
	if githubPr.Merged != nil && *githubPr.Merged {
		return false, errors.New("already merged")
	}
	if githubPr.Mergeable == nil {
		return false, errors.New("still being checked by github")
	}
	switch *githubPr.MergeableState {
	case "dirty":
		return false, fmt.Errorf("cannot be merged cleanly into %s", m.Repo.BaseBranch().RemoteName())
	case "blocked":
		return false, errors.New("not authorized to merge")
	case "unstable":
		return false, errors.New("has some failing checks")
	case "draft":
		return false, errors.New("draft PR")
	case "clean":
		return true, nil
	default:
		return false, errors.New("not mergeable")
	}
}

func (m *merger) Merge(ctx context.Context, pr *core.LocalPr) error {
	tip, err := m.Repo.GetLocalTip(pr)
	if err != nil {
		return err
	}
	fmt.Printf("Merging %s... ", pr.LocalBranch())
	merge, r, err := m.PullRequests.Merge(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber, "",
		&github.PullRequestOptions{
			SHA:         tip.Hash.String(),
			MergeMethod: core.GetGithubMergeMethod(),
		})
	if r != nil && r.StatusCode == http.StatusConflict {
		PrintFailure("wrong remote tip")
		return fmt.Errorf("did not merge %s", pr.LocalBranch())
	}
	if err == nil {
		PrintSuccess()
	} else {
		PrintFailure(err)
		return err
	}
	pr.AddKnownTip(plumbing.NewHash(merge.GetSHA()))
	return nil
}
