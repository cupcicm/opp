package story

// query

const linearQuery = `
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

// variables
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

//response
// {
// 	"data": {
// 	  "issues": {
// 		"pageInfo": {
// 		  "endCursor": "xxx",
// 		  "hasNextPage": false,
// 		  "hasPreviousPage": false,
// 		  "startCursor": "yyy"
// 		},
// 		"nodes": [
// 		  {
// 			"title": "my title",
// 			"identifier": "ABC-123"
// 		  }
// 		]
// 	  }
// 	}
//   }

type ResponseData struct {
	Issues Issues `json:"issues"`
}

type Issues struct {
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
	// Issue's human readable identifier (e.g. ABC-123).
	Identifier string `json:"identifier"`
}
