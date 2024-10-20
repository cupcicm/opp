package cmd

import (
	"fmt"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v2"
)

func BranchCliCommand(repo *core.Repo) *cli.Command {
	cmd := &cli.Command{
		Name:        "branch",
		Aliases:     []string{"b"},
		Description: "Allows you to manage your local branch",
		Usage:       "opp branch tag JIRA-1234",
		Subcommands: []*cli.Command{
			{
				Name:        "tag",
				Aliases:     []string{"t"},
				Description: "Tag the current branch with a JIRA ticket",
				Action: func(cCtx *cli.Context) error {
					return NewBranchCommand(repo).Tag(cCtx)
				},
			},
		},
	}
	return cmd
}

type BranchCommand struct {
	repo *core.Repo
}

func NewBranchCommand(repo *core.Repo) *BranchCommand {
	return &BranchCommand{repo}
}

func (c *BranchCommand) Tag(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 1 {
		return fmt.Errorf("usage: opp branch tag XXX")
	}
	var tag = cCtx.Args().First()
	current, err := c.repo.CurrentBranch()

	if err != nil {
		return err
	}
	var state = c.repo.StateStore().GetBranchState(current)
	state.Tag = tag
	c.repo.StateStore().SaveBranchState(current, state)
	return nil
}
