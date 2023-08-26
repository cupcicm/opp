package cmd

import (
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/spf13/cobra"
)

func PushCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "push",
		Aliases: []string{"up", "p"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var repo = core.Current()
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
			ancestor, err := pr.GetAncestor()
			if err != nil {
				return err
			}
			if ancestor.IsPr() {
				ancestorRemoteTip := core.Must(repo.GetRemoteTip(ancestor))
				prLocalTip := core.Must(repo.GetLocalTip(pr))
				isAncestor, _ := ancestorRemoteTip.IsAncestor(prLocalTip)
				if !isAncestor {
					fmt.Printf(
						"Branch %s does not have branch %s/%s in its history",
						core.GetRemoteName(), pr.LocalBranch(), ancestor.RemoteName(),
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
