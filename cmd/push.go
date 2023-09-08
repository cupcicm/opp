package cmd

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/spf13/cobra"
)

func PushCommand(repo *core.Repo) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "push",
		Aliases: []string{"up", "p"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo.Fetch(cmd.Context())
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
			return push(cmd.Context(), repo, pr)
		},
	}

	return cmd
}

func push(ctx context.Context, repo *core.Repo, pr *core.LocalPr) error {
	ancestor, err := pr.GetAncestor()
	if err != nil {
		return err
	}
	if ancestor.IsPr() {
		ancestorRemoteTip := core.Must(repo.GetLocalTip(ancestor))
		prLocalTip := core.Must(repo.GetLocalTip(pr))
		isAncestor, _ := ancestorRemoteTip.IsAncestor(prLocalTip)
		if !isAncestor {
			fmt.Printf(
				"Branch %s does not have branch %s in its history\n",
				pr.LocalBranch(), ancestor.LocalName(),
			)
			fmt.Println("Please run opp rebase")
			return nil
		}
		err := push(ctx, repo, ancestor.(*core.LocalPr))
		if err != nil {
			return err
		}
	}
	fmt.Printf("Pushing local changes to %s...", pr.Url())
	err = pr.Push(ctx)
	if err == nil {
		fmt.Println("  ✅")
	} else {
		fmt.Println("  ❌")
	}
	return err
}
