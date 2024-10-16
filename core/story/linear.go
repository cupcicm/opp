package story

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/cupcicm/opp/core"
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

func newClient() *graphql.Client {
	opt := graphql.WithHTTPClient(&http.Client{Transport: AuthHeader{Token: core.GetStoryToolToken()}})

	return graphql.NewClient(linearGraphqlEndpoint, opt)
}

func NewLinearStoryFetcher() StoryFetcher {
	return &linearStoryFetcher{
		client: newClient(),
	}
}

// {
// 	"filter": {
// 	  "state": {
// 		"type": {
// 		  "eq": "started"
// 		}
// 	  },
// 	  "assignee": {
// 		"isMe": {
// 		  "eq": true
// 		}
// 	  }
// 	},
// 	"sort": [
// 	  {
// 		"createdAt": {
// 		  "order": "Descending"
// 		}
// 	  }
// 	],
//   }

// Issue filtering options.
type IssueFilter struct {
	// Filters that the issues assignee must satisfy.
	Assignee *UserFilter `json:"assignee,omitempty"`
	// Filters that the issues state must satisfy.
	State *WorkflowStateFilter `json:"state,omitempty"`
}

type UserFilter struct {
	// Filter based on the currently authenticated user. Set to true to filter for the authenticated user, false for any other user.
	IsMe *BooleanComparator `json:"isMe,omitempty"`
}

type BooleanComparator struct {
	// Equals constraint.
	Eq *bool `json:"eq,omitempty"`
}

type WorkflowStateFilter struct {
	// Comparator for the workflow state type.
	Type *StringComparator `json:"type,omitempty"`
}

type StringComparator struct {
	// Equals constraint.
	Eq *string `json:"eq,omitempty"`
}

type IssueSortInput struct {
	SortOptions []SortOption
}

type SortOption struct {
	CreatedAt *OrderComparator `json:"createdAt,omitempty"`
}

type OrderComparator struct {
	Order *string `json:"order,omitempty"`
}

// {
// 	"data": {
// 	  "issues": {
// 		"pageInfo": {
// 		  "endCursor": "eyJrZXkiOiIwYWVkOWIzYS0xOWU4LTQ3MWYtYjg4Ny1mYmE2NWQ2YTFmZDAiLCJncm91cCI6IiJ9",
// 		  "hasNextPage": false,
// 		  "hasPreviousPage": false,
// 		  "startCursor": "eyJrZXkiOiIwYWVkOWIzYS0xOWU4LTQ3MWYtYjg4Ny1mYmE2NWQ2YTFmZDAiLCJncm91cCI6IiJ9"
// 		},
// 		"nodes": [
// 		  {
// 			"title": "Welcome to Linear 👋",
// 			"identifier": "CUP-1"
// 		  }
// 		]
// 	  }
// 	}
//   }

type R struct {
	Issues Resp `json:"issues"`
}

type Resp struct {
	Nodes    []Issue   `json:"nodes"`
	PageInfo *PageInfo `json:"pageInfo"`
}

type PageInfo struct {
	// Indicates if there are more results when paginating backward.
	HasPreviousPage bool `json:"hasPreviousPage"`
	// Indicates if there are more results when paginating forward.
	HasNextPage bool `json:"hasNextPage"`
	// Cursor representing the first result in the paginated results.
	StartCursor *string `json:"startCursor,omitempty"`
	// Cursor representing the last result in the paginated results.
	EndCursor *string `json:"endCursor,omitempty"`
}

// An issue.
type Issue struct {
	// The issue's title.
	Title string `json:"title"`
	// Issue's human readable identifier (e.g. ENG-123).
	Identifier string `json:"identifier"`
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
	var respData R
	if err := l.client.Run(ctx, req, &respData); err != nil {
		log.Fatal(err)
	}
	for _, issue := range respData.Issues.Nodes {
		stories = append(stories, Story{
			title:      issue.Title,
			identifier: issue.Identifier,
		})
	}
	fmt.Println(stories)

	return stories, nil
}

func Toto(ctx context.Context) {

	stories := make([]Story, 0)
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
	var respData R
	if err := client.Run(ctx, req, &respData); err != nil {
		log.Fatal(err)
	}
	for _, issue := range respData.Issues.Nodes {
		stories = append(stories, Story{
			title:      issue.Title,
			identifier: issue.Identifier,
		})
	}
	fmt.Println(stories)
	log.Fatal(nil)

}
