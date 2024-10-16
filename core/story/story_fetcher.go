package story

import "context"

type Story struct {
	title      string
	identifier string
}

type StoryFetcher interface {
	FetchInProgressStories(context.Context) ([]Story, error)
}
