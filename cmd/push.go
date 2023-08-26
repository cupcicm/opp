package cmd

import (
	"fmt"

	"github.com/cupcicm/opp/git"
	"github.com/spf13/cobra"
)

func PushCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "push",
		Aliases: []string{"up", "p"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var repo = git.Current()
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
			pr := branch.(*git.LocalPr)
			ancestor, err := pr.GetAncestor()
			if err != nil {
				return err
			}
			if ancestor.IsPr() {
				ancestorRemoteTip := git.Must(repo.GetRemoteTip(ancestor))
				prLocalTip := git.Must(repo.GetLocalTip(pr))
				isAncestor, _ := ancestorRemoteTip.IsAncestor(prLocalTip)
				if !isAncestor {
					fmt.Printf(
						"Branch %s does not have branch %s/%s in its history",
						git.GetRemoteName(), pr.LocalBranch(), ancestor.RemoteName(),
					)
					fmt.Print("Please run opp rebase")
					return nil
				}
			}
			return pr.Push(cmd.Context())
		},
	}

	return cmd
}
