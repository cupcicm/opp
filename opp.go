package main

import (
	"context"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func CommandContext() context.Context {
	return context.Background()
}

func main() {
	viper.AddConfigPath(core.Current().DotOpDir())
	viper.AddConfigPath("$HOME/.config/opp")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.ReadInConfig()
	root := cobra.Command{
		Use: "opp",
	}
	ctx := CommandContext()
	root.AddCommand(cmd.InitCommand())
	root.AddCommand(cmd.PrCommand())
	root.AddCommand(cmd.RebaseCommand())
	root.AddCommand(cmd.PushCommand())

	if !core.Current().OppEnabled() {
		cmd.InitCommand().ExecuteContext(ctx)
		return
	}

	if err := root.ExecuteContext(ctx); err != nil {
		panic(err)
	}
}
