package cmd

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v2"
)

func PushCommand(repo *core.Repo) *cli.Command {
	cmd := &cli.Command{
		Name:    "push",
		Aliases: []string{"up", "p", "push"},
		Action: func(cCtx *cli.Context) error {
			repo.Fetch(cCtx.Context)
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
			return push(cCtx.Context, repo, pr)
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
		ancestorRemoteTip := core.Must(repo.GetLocalTip(ancestor))
		prLocalTip := core.Must(repo.GetLocalTip(pr))
		isAncestor, _ := ancestorRemoteTip.IsAncestor(prLocalTip)
		if !isAncestor {
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
