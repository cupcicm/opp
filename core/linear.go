package core

import (
	"context"
	"fmt"
	"net/http"

	"git.sr.ht/~emersion/gqlclient"
	"github.com/guillermo/linear/linear-api"
)

type Story struct {
	title      string
	identifier string
}

type StoryFetcher interface {
	FetchInProgressStories(context.Context) ([]Story, error)
}

type AuthHeader struct{ Token string }

func (h AuthHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", h.Token)
	return http.DefaultTransport.RoundTrip(req)
}

func newGqlClient() *gqlclient.Client {
	token := GetStoryToolToken()
	return gqlclient.New(
		"https://api.linear.app/graphql",
		&http.Client{Transport: AuthHeader{Token: token}},
	)
}

type linearStoryFetcher struct {
	gqlClient *gqlclient.Client
}

func NewLinearStoryFetcher() StoryFetcher {
	return &linearStoryFetcher{
		gqlClient: newGqlClient(),
	}
}

func (l *linearStoryFetcher) FetchInProgressStories(ctx context.Context) (stories []Story, err error) {
	user, err := linear.FetchMe(l.gqlClient, ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not get user: %w", err)
	}

	filter := &linear.IssueFilter{
		Assignee: &linear.NullableUserFilter{
			Id: &linear.IDComparator{
				Eq: user.Id,
			},
		},
	}
	var after string = ""
	var i int32 = 30

	issueConnection, err := linear.FetchIssues(l.gqlClient, ctx, filter, &i, &after)

	for _, issue := range issueConnection.Nodes {
		stories = append(stories, Story{
			title:      issue.Title,
			identifier: issue.Identifier,
		})
	}

	return stories, nil
}
