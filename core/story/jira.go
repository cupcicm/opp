package story

import (
	"context"
	"fmt"

	v3 "github.com/ctreminiom/go-atlassian/v2/jira/v3"
)

type jiraStoryFetcher struct {
	client *v3.Client
}

func NewJiraStoryFetcher(host string, user_email string, token string) StoryFetcher {
	client, err := v3.New(nil, host)
	if err != nil {
		panic(fmt.Sprintf("failed to create jira client: %v", err))
	}

	client.Auth.SetBasicAuth(user_email, token)

	return &jiraStoryFetcher{
		client: client,
	}
}

func (j *jiraStoryFetcher) FetchInProgressStories(ctx context.Context) ([]Story, error) {
	// Using JQL to search for in-progress issues using the new /rest/api/3/search/jql endpoint
	jql := "status = \"In Progress\" ORDER BY created DESC"

	// Using the new JQL search endpoint
	result, _, err := j.client.Issue.Search.SearchJQL(ctx, jql, []string{"summary", "key"}, nil, 20, "")
	if err != nil {
		return nil, fmt.Errorf("failed to search for issues: %w", err)
	}

	stories := make([]Story, 0, len(result.Issues))
	for _, issue := range result.Issues {
		stories = append(stories, Story{
			Title:      issue.Fields.Summary,
			Identifier: issue.Key,
		})
	}

	return stories, nil
}
