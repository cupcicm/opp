package cmd

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v2"
)

func CleanCommand(repo *core.Repo, gh func(context.Context) core.GhPullRequest) *cli.Command {
	cmd := &cli.Command{
		Name:        "clean",
		Aliases:     []string{"gc"},
		Description: "Deletes all local PRs that have been closed on github",
		Action: func(cCtx *cli.Context) error {
			localPrs := repo.AllPrs(cCtx.Context)
			for _, pr := range localPrs {
				pullRequests := gh(cCtx.Context)

				githubPr, _, err := pullRequests.Get(cCtx.Context, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
				if err != nil {
					return err
				}
				if *githubPr.State == "closed" {
					clean(cCtx.Context, repo, &pr)
				}
			}
			return nil
		},
	}
	return cmd
}

func clean(ctx context.Context, repo *core.Repo, pr *core.LocalPr) error {
	tip, err := repo.GetLocalTip(pr)
	if err != nil {
		return err
	}
	repo.DeleteLocalAndRemoteBranch(ctx, pr)
	fmt.Printf("Deleted branch %s. Tip was %s\n", pr.LocalBranch(), tip.Hash.String()[0:7])
	return nil
}
