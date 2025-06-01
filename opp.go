package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/story"
	"github.com/spf13/viper"
)

func CommandContext() (context.Context, context.CancelCauseFunc) {
	return context.WithCancelCause(context.Background())
}

func gh(ctx context.Context) core.Gh {
	return core.NewClient(ctx)
}

func sf(config story.StoryFetcherConfig) story.StoryFetcher {
	return story.NewStoryFetcher(config)
}

func main() {
	repo := core.Current()
	viper.AddConfigPath(repo.DotOpDir())
	viper.AddConfigPath("$HOME/.config/opp")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.ReadInConfig()

	root := cmd.MakeApp(os.Stdout, os.Stdin, repo, gh, sf)
	ctx, cancel := CommandContext()
	if !repo.OppEnabled() && (len(os.Args) != 2 || os.Args[1] != "init") {
		fmt.Println("Please run opp init first")
		os.Exit(1)
	}
	signalChan := make(chan os.Signal, 1)
	// Get a signal when the User Ctrl-C.
	signal.Notify(signalChan, os.Interrupt)
	defer func() {
		signal.Stop(signalChan)
		cancel(nil)
	}()
	go func() {
		select {
		case <-signalChan: // first signal, cancel context
			cancel(errors.New("interrupted"))
		case <-ctx.Done():
		}
		<-signalChan // second signal, hard exit
		os.Exit(2)
	}()
	if err := root.Run(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}
