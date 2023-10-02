package cmd

import (
	"context"
	"io"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v2"
)

func MakeApp(out io.Writer, repo *core.Repo, gh func(context.Context) core.GhPullRequest) *cli.App {
	return &cli.App{
		Name:  "opp",
		Usage: "Create, update and merge Github pull requests from the command line.",
		Commands: []*cli.Command{
			InitCommand(repo),
			CleanCommand(repo, gh),
			PrCommand(repo, gh),
			MergeCommand(repo, gh),
			StatusCommand(out, repo, gh),
			RebaseCommand(repo),
			PushCommand(repo),
		},
	}
}
