package cmd

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v3"
)

func PushCommand(repo *core.Repo) *cli.Command {
	cmd := &cli.Command{
		Name:    "push",
		Aliases: []string{"up", "p"},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			err := repo.Fetch(ctx)
			if err != nil {
				return fmt.Errorf("on fetch: %w", err)
			}
			branch, err := repo.CurrentBranch()
			if err != nil {
				return err
			}
			if !branch.IsPr() {
				fmt.Println("Not a PR branch.")
				fmt.Println("Please use opp new instead")
				return nil
			}
			pr := branch.(*core.LocalPr)
			return push(ctx, repo, pr)
		},
	}

	return cmd
}

func push(ctx context.Context, repo *core.Repo, pr *core.LocalPr) error {
	ancestor, err := pr.GetAncestor()
	if err != nil {
		// Assume the ancestor is the base branch
		ancestor = repo.BaseBranch()
	}
	if ancestor.IsPr() {
		ancestorTip := core.Must(repo.GetLocalTip(ancestor))
		prLocalTip := core.Must(repo.GetLocalTip(pr))
		if !repo.IsAncestor(ctx, ancestorTip, prLocalTip) {
			return cli.Exit(fmt.Errorf(
				"branch %s does not have branch %s in its history\nPlease run opp rebase",
				pr.LocalBranch(), ancestor.LocalName(),
			), 1)
		}
		err := push(ctx, repo, ancestor.(*core.LocalPr))
		if err != nil {
			return err
		}
	}
	fmt.Printf("Pushing local changes to %s ... ", pr.Url())
	err = pr.Push(ctx)
	if err == nil {
		PrintSuccess()
		pr.RememberCurrentTip()
		return nil
	}

	PrintFailure(nil)
	return cli.Exit(fmt.Errorf("could not push : %w", err), 1)
}
