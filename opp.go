package main

import (
	"context"
	"log"
	"os"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/spf13/viper"
)

func CommandContext() context.Context {
	return context.Background()
}

func gh(ctx context.Context) core.GhPullRequest {
	return core.NewClient(ctx).PullRequests()
}

func main() {
	repo := core.Current()
	viper.AddConfigPath(repo.DotOpDir())
	viper.AddConfigPath("$HOME/.config/opp")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.ReadInConfig()

	root := cmd.MakeApp(os.Stdout, repo, gh)
	ctx := CommandContext()
	if !repo.OppEnabled() {
		if err := root.RunContext(ctx, []string{"init"}); err != nil {
			log.Fatal(err)
		}
	}

	if err := root.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
