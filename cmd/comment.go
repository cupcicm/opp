package cmd

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/google/go-github/v56/github"
	"github.com/urfave/cli/v3"
)

func CommentCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "comment",
		Description: "Add a comment to a PR",
		Usage:       "Adds a comment to a PR",
		Action: func(ctx context.Context, cmd *cli.Command) error {

			var prParam string
			var comment string
			switch cmd.NArg() {
			case 1:
				prParam = ""
				comment = cmd.Args().First()
			case 2:
				prParam = cmd.Args().First()
				comment = cmd.Args().Get(1)
			default:
				return cli.Exit("Usage: opp comment [pr] $comment", 1)
			}

			pr, _, err := PrFromStringOrCurrentBranch(repo, prParam)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeoutCause(
				ctx, core.GetGithubTimeout(),
				fmt.Errorf("adding comment too slow, increase github.timeout"),
			)
			defer cancel()

			_, _, err = gh(ctx).Issues().CreateComment(ctx, core.GetGithubOwner(),
				core.GetGithubRepoName(), pr.PrNumber, &github.IssueComment{Body: &comment})
			if err != nil {
				PrintFailure(nil)
				return err
			}
			fmt.Printf("Comment added to %s ", pr.Url())
			PrintSuccess()
			return nil
		},
	}
	return cmd
}
