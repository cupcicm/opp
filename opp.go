package main

import (
	"context"
	"os"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func CommandContext() context.Context {
	return context.Background()
}

func main() {
	repo := core.Current()
	viper.AddConfigPath(repo.DotOpDir())
	viper.AddConfigPath("$HOME/.config/opp")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.ReadInConfig()
	root := cobra.Command{
		Use:          "opp",
		SilenceUsage: true,
	}
	ctx := CommandContext()
	root.AddCommand(cmd.InitCommand(repo))
	root.AddCommand(cmd.PrCommand(repo, core.NewClient(ctx).PullRequests()))
	root.AddCommand(cmd.MergeCommand(repo, core.NewClient(ctx).PullRequests()))
	root.AddCommand(cmd.StatusCommand(os.Stdout, repo, core.NewClient(ctx).PullRequests()))
	root.AddCommand(cmd.RebaseCommand(repo))
	root.AddCommand(cmd.PushCommand(repo))

	if !repo.OppEnabled() {
		cmd.InitCommand(repo).ExecuteContext(ctx)
		return
	}

	if err := root.ExecuteContext(ctx); err != nil {
		panic(err)
	}
}
