package story

import (
	"context"
	"fmt"
)

type Story struct {
	Title      string
	Identifier string
}

type StoryFetcher interface {
	FetchInProgressStories(context.Context) ([]Story, error)
}

// StoryFetcherConfig holds all possible configuration options for different story fetchers
type StoryFetcherConfig struct {
	// Common fields
	Tool  string
	Token string

	// Jira specific fields
	Host  string
	Email string
}

func NewStoryFetcher(config StoryFetcherConfig) StoryFetcher {
	switch config.Tool {
	case "linear":
		return NewLinearStoryFetcher(config.Token)
	case "jira":
		if config.Email == "" || config.Host == "" {
			panic("Jira story fetcher requires email and host to be configured")
		}
		return NewJiraStoryFetcher(config.Host, config.Email, config.Token)
	default:
		panic(fmt.Sprintf("Story tool %q not supported to fetch stories", config.Tool))
	}
}
