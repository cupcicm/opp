package cmd

import (
	"context"
	"errors"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v2"
)

func CleanCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "clean",
		Aliases:     []string{"gc"},
		Description: "Deletes all local PRs that have been closed on github",
		Action: func(cCtx *cli.Context) error {
			repo.Fetch(cCtx.Context)
			localPrs := repo.AllPrs(cCtx.Context)
			for _, pr := range localPrs {
				pullRequests := gh(cCtx.Context).PullRequests()
				_, err := repo.GetRemoteTip(&pr)
				if errors.Is(err, plumbing.ErrReferenceNotFound) {
					// The remote tip does not exist anymore : it has been deleted on the github repo.
					// Probably because the PR is either abandonned or merged.
					repo.CleanupAfterMerge(cCtx.Context, &pr)
				} else {
					githubPr, _, err := pullRequests.Get(cCtx.Context, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
					if err != nil {
						return err
					}
					if *githubPr.State == "closed" {
						repo.CleanupAfterMerge(cCtx.Context, &pr)
					}
				}
			}
			return nil
		},
	}
	return cmd
}
