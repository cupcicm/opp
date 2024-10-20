package story

import (
	"context"
	"fmt"
	"net/http"

	"github.com/machinebox/graphql"
)

const linearGraphqlEndpoint = "https://api.linear.app/graphql"

type linearStoryFetcher struct {
	client *graphql.Client
}

type AuthHeader struct{ Token string }

func (h AuthHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", h.Token)
	return http.DefaultTransport.RoundTrip(req)
}

func newClient(token string) *graphql.Client {
	opt := graphql.WithHTTPClient(&http.Client{Transport: AuthHeader{Token: token}})

	return graphql.NewClient(linearGraphqlEndpoint, opt)
}

func NewLinearStoryFetcher(token string) StoryFetcher {
	return &linearStoryFetcher{
		client: newClient(token),
	}
}

func (l *linearStoryFetcher) FetchInProgressStories(ctx context.Context) (stories []Story, err error) {
	var isMe bool = true
	var state string = "started"
	var order string = "Descending"

	filter := &IssueFilter{
		Assignee: &UserFilter{
			IsMe: &BooleanComparator{
				Eq: &isMe,
			},
		},
		State: &WorkflowStateFilter{
			Type: &StringComparator{
				Eq: &state,
			},
		},
	}

	sort := []SortOption{
		{
			CreatedAt: &OrderComparator{
				Order: &order,
			},
		},
	}

	// make a request
	req := graphql.NewRequest(linearQuery)

	// set any variables
	req.Var("filter", filter)
	req.Var("sort", sort)

	// run it and capture the response
	var respData ResponseData
	if err := l.client.Run(ctx, req, &respData); err != nil {
		// TODO(ClairePhi): add retry with avast retry-go
		return nil, fmt.Errorf("failed to fetch data from the linear graphql API: %w", err)
	}
	for _, issue := range respData.Issues.Nodes {
		stories = append(stories, Story{
			title:      issue.Title,
			identifier: issue.Identifier,
		})
	}

	return stories, nil
}
