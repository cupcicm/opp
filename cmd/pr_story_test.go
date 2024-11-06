package cmd_test

import (
	"context"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/google/go-github/v56/github"
)

func TestPrStory(t *testing.T) {
	remote := "cupcicm/pr/2"
	base := "master"
	draft := false

	testCases := []struct {
		name           string
		commitMessages []string
		expectedTitle  string
		expectedBody   string
	}{
		{
			name:           "story a the beginning of the title added to body",
			commitMessages: []string{"[ABC-123] my title\nmy body", "a\nb"},
			expectedTitle:  "[ABC-123] my title",
			expectedBody:   "- Linear [ABC-123](https://my.base.url/browse/ABC-123)\n\nmy body",
		},
		{
			name:           "story a the middle of the title added to body",
			commitMessages: []string{"my [ABC-123] title\nmy body", "a\nb"},
			expectedTitle:  "my [ABC-123] title",
			expectedBody:   "- Linear [ABC-123](https://my.base.url/browse/ABC-123)\n\nmy body",
		},
		{
			name:           "story a the end of the title added to body",
			commitMessages: []string{"my title [ABC-123]\nmy body", "a\nb"},
			expectedTitle:  "my title [ABC-123]",
			expectedBody:   "- Linear [ABC-123](https://my.base.url/browse/ABC-123)\n\nmy body",
		},
		{
			name:           "no story",
			commitMessages: []string{"longest commit message\nlongest commit message body", "a\nb", "c\nd"},
			expectedTitle:  "longest commit message",
			expectedBody:   "longest commit message body",
		},
		{
			name:           "empty body",
			commitMessages: []string{"[ABC-123] my commit title", "a\nb"},
			expectedTitle:  "[ABC-123] my commit title",
			expectedBody:   "- Linear [ABC-123](https://my.base.url/browse/ABC-123)",
		},
		{
			name:           "story twice in the title",
			commitMessages: []string{"[ABC-123] [DEF-456] my title\nmy body", "a\nb"},
			expectedTitle:  "[ABC-123] [DEF-456] my title",
			expectedBody:   "- Linear [ABC-123](https://my.base.url/browse/ABC-123)\n\nmy body",
		},
		{
			name:           "story extracted from other commit",
			commitMessages: []string{"my title without story\nmy body without story", "a\nhere [ABC-123]", "c\nd"},
			expectedTitle:  "[ABC-123] my title without story",
			expectedBody:   "- Linear [ABC-123](https://my.base.url/browse/ABC-123)\n\nmy body without story",
		},
		{
			name:           "story extracted from other commit with several stories",
			commitMessages: []string{"my title without story\nmy body without story", "a\nhere [ABC-123]", "c\nand here [DEF-456]"},
			expectedTitle:  "[DEF-456] my title without story",
			expectedBody:   "- Linear [DEF-456](https://my.base.url/browse/DEF-456)\n\nmy body without story",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := tests.NewTestRepo(t)

			r.Repo.GitExec(context.Background(), "checkout origin/master").Run()
			r.Repo.GitExec(context.Background(), "checkout -b test_branch").Run()

			wt := core.Must(r.Source.Worktree())

			for _, commitMessage := range tc.commitMessages {
				wt.Add("README.md")
				r.Commit(commitMessage)
			}

			prDetails := github.NewPullRequest{
				Title: &tc.expectedTitle,
				Head:  &remote,
				Base:  &base,
				Body:  &tc.expectedBody,
				Draft: &draft,
			}

			r.CreatePrAssertPrDetails(t, "HEAD", 2, prDetails)
		})
	}
}
