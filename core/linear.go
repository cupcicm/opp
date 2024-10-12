package core

import (
	"context"
	"net/http"

	"git.sr.ht/~emersion/gqlclient"
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
	// linear.FetchIssues()
	var isMe bool = true
	var state string = "started"
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

	var order string = "Descending"
	sort := []SortOption{
		{
			CreatedAt: &OrderComparator{
				Order: &order,
			},
		},
	}

	// op := gqlclient.NewOperation("query issues ($filter: IssueFilter, $first: Int, $after: String) {\n\tissues(filter: $filter, first: $first, after: $after) {\n\t\tnodes {\n\t\t\tidentifier\n\t\t\tsortOrder\n\t\t\ttitle\n\t\t\tdescription\n\t\t\tbranchName\n\t\t\tcycle {\n\t\t\t\tid\n\t\t\t\tname\n\t\t\t}\n\t\t\tlabels {\n\t\t\t\tnodes {\n\t\t\t\t\tname\n\t\t\t\t\tcolor\n\t\t\t\t}\n\t\t\t}\n\t\t\tproject {\n\t\t\t\tname\n\t\t\t\tid\n\t\t\t\tcolor\n\t\t\t}\n\t\t\tassignee {\n\t\t\t\tname\n\t\t\t\tisMe\n\t\t\t}\n\t\t\tstate {\n\t\t\t\tname\n\t\t\t\tcolor\n\t\t\t\tposition\n\t\t\t\ttype\n\t\t\t}\n\t\t}\n\t\tpageInfo {\n\t\t\tendCursor\n\t\t\thasNextPage\n\t\t\thasPreviousPage\n\t\t\tstartCursor\n\t\t}\n\t}\n}\n")

	query := `
query issues ($filter: IssueFilter, $sort: [IssueSortInput!]) {
	issues(filter: $filter, sort: $sort) {
		pageInfo {
			endCursor
			hasNextPage
			hasPreviousPage
			startCursor
		}
		nodes {
			title
			identifier
		}
	}
}
`
	op := gqlclient.NewOperation(query)
	op.Var("filter", filter)
	op.Var("sort", sort)

	var resp R

	err = l.gqlClient.Execute(ctx, op, &resp)
	if err != nil {
		return nil, err
	}

	for _, issue := range resp.Issues.Nodes {
		stories = append(stories, Story{
			title:      issue.Title,
			identifier: issue.Identifier,
		})
	}

	return stories, nil
}
