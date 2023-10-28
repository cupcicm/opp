package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cupcicm/opp/core"
	"github.com/google/go-github/v56/github"
	"github.com/urfave/cli/v2"
)

func CommentCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "comment",
		Description: "Add a comment to a PR",
		Usage:       "Adds a comment to a PR",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "pr",
				Usage: "number of the PR on which to add a comment (defaults to current PR branch number)",
			},
		},
		Action: func(cCtx *cli.Context) error {

			if cCtx.NArg() == 0 {
				return cli.Exit("No comment body supplied", 1)
			}
			commentBody := cCtx.Args().Get(0)
			var prNumber int
			var err error
			if cCtx.IsSet("pr") {
				prStr := cCtx.String("pr")
				prNumber, err = strconv.Atoi(prStr)
				if err != nil {
					return cli.Exit(fmt.Sprintf("Invalid pr numnber '%s'", prStr), 1)
				}
			} else {
				pr, found := repo.PrForHead()
				if !found {
					return cli.Exit("Can't determine pr number on which to add comment", 1)
				}
				prNumber = pr.PrNumber
			}
			ctx, cancel := context.WithTimeoutCause(
				cCtx.Context, core.GetGithubTimeout(),
				fmt.Errorf("adding comment too slow, increase github.timeout"),
			)
			defer cancel()

			_, _, err = gh(cCtx.Context).Issues().CreateComment(ctx, core.GetGithubOwner(),
				core.GetGithubRepoName(), prNumber, &github.IssueComment{Body: &commentBody})
			if err != nil {
				PrintFailure(nil)
				return err
			}
			PrintSuccess()
			return nil
		},
	}
	return cmd
}
