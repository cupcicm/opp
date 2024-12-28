package cmd

import (
	"context"
	"errors"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v3"
)

func CleanCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "clean",
		Aliases:     []string{"gc"},
		Description: "Deletes all local PRs that have been closed on github",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			repo.Fetch(ctx)
			localPrs := repo.AllPrs(ctx)
			for _, pr := range localPrs {
				pullRequests := gh(ctx).PullRequests()
				_, err := repo.GetRemoteTip(&pr)
				if errors.Is(err, plumbing.ErrReferenceNotFound) {
					// The remote tip does not exist anymore : it has been deleted on the github repo.
					// Probably because the PR is either abandonned or merged.
					repo.CleanupAfterMerge(ctx, &pr)
				} else {
					githubPr, _, err := pullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
					if err != nil {
						return err
					}
					if *githubPr.State == "closed" {
						repo.CleanupAfterMerge(ctx, &pr)
					}
				}
			}
			return nil
		},
	}
	return cmd
}
