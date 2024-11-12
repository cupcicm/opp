package story

import (
	"context"
	"fmt"
	"net/http"

	"github.com/avast/retry-go"
	"github.com/machinebox/graphql"
)

const (
	linearGraphqlEndpoint = "https://api.linear.app/graphql"
	maxRetries            = 3
)

type graphqlRetryClient struct {
	*graphql.Client
}

func (c *graphqlRetryClient) Run(ctx context.Context, req *graphql.Request, resp interface{}) error {
	return retry.Do(
		func() error {
			return c.Client.Run(ctx, req, resp)
		},
		retry.Attempts(maxRetries),
	)
}

type linearStoryFetcher struct {
	client *graphqlRetryClient
}

type AuthHeader struct{ Token string }

func (h AuthHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", h.Token)
	return http.DefaultTransport.RoundTrip(req)
}

func newClient(token string) *graphqlRetryClient {
	opt := graphql.WithHTTPClient(&http.Client{Transport: AuthHeader{Token: token}})
	return &graphqlRetryClient{
		Client: graphql.NewClient(linearGraphqlEndpoint, opt),
	}
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
		return nil, fmt.Errorf("failed to fetch data from the linear graphql API: %w", err)
	}
	for _, issue := range respData.Issues.Nodes {
		stories = append(stories, Story(issue))
	}

	return stories, nil
}
