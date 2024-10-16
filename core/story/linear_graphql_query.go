package story

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
