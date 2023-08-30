package core

import (
	"context"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type GhPullRequest interface {
	List(ctx context.Context, owner string, repo string, opt *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error)
	Create(ctx context.Context, owner string, repo string, pull *github.NewPullRequest) (*github.PullRequest, *github.Response, error)
	Get(ctx context.Context, owner string, repo string, number int) (*github.PullRequest, *github.Response, error)
	Merge(ctx context.Context, owner string, repo string, number int, commitMessage string, options *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error)
}

type GithubClient struct {
	*github.Client
}

func (c *GithubClient) PullRequests() GhPullRequest {
	return c.Client.PullRequests
}

func NewClient(ctx context.Context) *GithubClient {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: GetGithubToken()},
	)
	tc := oauth2.NewClient(ctx, ts)
	return &GithubClient{Client: github.NewClient(tc)}
}
