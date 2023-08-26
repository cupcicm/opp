package git

import (
	"context"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func NewClient(ctx context.Context) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: GetGithubToken()},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
