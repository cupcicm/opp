package story

import (
	"context"
	"fmt"

	"github.com/cupcicm/opp/core"
)

type Story struct {
	title      string
	identifier string
}

type StoryFetcher interface {
	FetchInProgressStories(context.Context) ([]Story, error)
}

type StoryFetcherNoop struct{}

func (s *StoryFetcherNoop) FetchInProgressStories(_ context.Context) ([]Story, error) {
	return []Story{}, nil
}

func getStoryFetcher() StoryFetcher {
	if !core.FetchStoriesEnabled() {
		return &StoryFetcherNoop{}
	}

	storyTool, err := getStoryTool()
	if err != nil {
		fmt.Printf("Story tool not configured correctly, will not attempt to fetch stories: %s\n", err.Error())
		return &StoryFetcherNoop{}
	}

	switch t := storyTool.tool; t {
	case "linear":
		return NewLinearStoryFetcher()
	default:
		fmt.Printf("Story tool not supported, will not attempt to fetch stories: %s\n", t)
		return &StoryFetcherNoop{}
	}
}
