package cmd

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v3"
)

func LinkCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	return &cli.Command{
		Name:      "link",
		Usage:     "Copy a PR link to the clipboard",
		ArgsUsage: "[pr]",
		Description: `Copies a rich link (URL + title) for the given PR to the clipboard.
If no PR is specified and you're on a PR branch, uses the current branch's PR.`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			pr, _, err := PrFromFirstArgument(repo, cmd)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeoutCause(
				ctx, core.GetGithubTimeout(),
				fmt.Errorf("fetching PR too slow, increase github.timeout"),
			)
			defer cancel()
			githubPr, _, err := gh(ctx).PullRequests().Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
			if err != nil {
				return fmt.Errorf("could not fetch PR #%d: %w", pr.PrNumber, err)
			}
			title := ""
			if githubPr.Title != nil {
				title = *githubPr.Title
			}
			core.ClipboardWrite(pr, title)
			fmt.Println(pr.Url())
			return nil
		},
	}
}
