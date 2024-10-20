package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/story"
	"github.com/urfave/cli/v2"
)

func MakeApp(out io.Writer, in io.Reader, repo *core.Repo, gh func(context.Context) core.Gh, sf func(string, string) story.StoryFetcher) *cli.App {
	return &cli.App{
		Name:  "opp",
		Usage: "Create, update and merge Github pull requests from the command line.",
		Commands: []*cli.Command{
			InitCommand(repo),
			CleanCommand(repo, gh),
			CloseCommand(repo, gh),
			PrCommand(in, repo, gh, sf),
			MergeCommand(repo, gh),
			StatusCommand(out, repo, gh),
			RebaseCommand(repo),
			PushCommand(repo),
			CommentCommand(repo, gh),
		},
		Action: func(ctx *cli.Context) error {
			// Called only if no subcommand match.
			args := ctx.Args()
			if !args.Present() {
				return errors.New("no subcommand provided")
			}
			subcommand := args.First()
			return runCustomScript(ctx.Context, subcommand, args.Slice()[1:])
		},
	}
}

// Much like git, if no valid subcommand match, run `opp-XXX.sh` instead.
// This allows the user to create new opp commands.
func runCustomScript(ctx context.Context, subcommand string, args []string) error {
	subcommand = fmt.Sprintf("opp-%s.sh %s", subcommand, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "bash", "-c", subcommand)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
