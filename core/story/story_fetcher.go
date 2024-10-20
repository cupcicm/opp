package story

import (
	"context"
)

type Story struct {
	Title      string
	Identifier string
}

type StoryFetcher interface {
	FetchInProgressStories(context.Context) ([]Story, error)
}

func NewStoryFetcher(tool, token string) StoryFetcher {
	switch tool {
	case "linear":
		return NewLinearStoryFetcher(token)
	default:
		panic("Story tool not supported to fetch stories")
	}
}
