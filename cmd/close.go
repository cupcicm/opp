package cmd

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v2"
)

func CloseCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "close",
		Aliases:     []string{"abandon"},
		Description: "Closes an open PR without merging it. Also deletes its local branch",
		Action: func(cCtx *cli.Context) error {
			pr, currentBranch, err := PrFromFirstArgument(repo, cCtx)
			if err != nil {
				return err
			}
			if currentBranch {
				repo.Checkout(repo.BaseBranch())
			}
			// Deleting the remote branch closes the PR.
			fmt.Printf("Closing %s... ", pr.LocalBranch())
			err = repo.DeleteLocalAndRemoteBranch(cCtx.Context, pr)
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
