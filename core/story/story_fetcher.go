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
