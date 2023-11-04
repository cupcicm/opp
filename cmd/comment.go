package cmd

import (
	"context"
	"fmt"
	"github.com/cupcicm/opp/core"
	"github.com/google/go-github/v56/github"
	"github.com/urfave/cli/v2"
)

func CommentCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "comment",
		Description: "Add a comment to a PR",
		Usage:       "Adds a comment to a PR",
		Action: func(cCtx *cli.Context) error {

			var prParam string
			var comment string
			switch cCtx.NArg() {
			case 1:
				prParam = ""
				comment = cCtx.Args().First()
			case 2:
				prParam = cCtx.Args().First()
				comment = cCtx.Args().Get(1)
			default:
				return cli.Exit("Usage: opp comment [pr] $comment", 1)
			}

			pr, _, err := PrFromStringOrCurrentBranch(repo, prParam)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeoutCause(
				cCtx.Context, core.GetGithubTimeout(),
				fmt.Errorf("adding comment too slow, increase github.timeout"),
			)
			defer cancel()

			_, _, err = gh(cCtx.Context).Issues().CreateComment(ctx, core.GetGithubOwner(),
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
