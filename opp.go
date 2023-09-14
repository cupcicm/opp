package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/spf13/viper"
)

func CommandContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*12)
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
	ctx, cancel := CommandContext()
	if !repo.OppEnabled() {
		if err := root.RunContext(ctx, []string{"init"}); err != nil {
			log.Fatal(err)
		}
	}
	signalChan := make(chan os.Signal, 1)
	// Get a signal when the User Ctrl-C.
	signal.Notify(signalChan, os.Interrupt)
	defer func() {
		signal.Stop(signalChan)
		cancel()
	}()
	go func() {
		select {
		case <-signalChan: // first signal, cancel context
			cancel()
		case <-ctx.Done():
		}
		<-signalChan // second signal, hard exit
		os.Exit(2)
	}()
	if err := root.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
