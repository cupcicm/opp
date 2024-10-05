package cmd_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/google/go-github/v56/github"
)

func TestPrTitle_StoryFromTitle(t *testing.T) {
	remote := "cupcicm/pr/2"
	base := "master"
	body := ""
	draft := false

	testCases := []struct {
		name           string
		commitMessages []string
		expectedTitle  string
	}{
		{
			name:           "at the beginning of the title",
			commitMessages: []string{"[ABC-456] c", "[ABC-123] fix that"},
			expectedTitle:  "[ABC-123] fix that",
		},
		{
			name:           "in the middle of the title",
			commitMessages: []string{"[ABC-456] c", "fix [ABC-123] that"},
			expectedTitle:  "fix [ABC-123] that",
		},
		{
			name:           "at the end of the title",
			commitMessages: []string{"[ABC-456] c", "fix that [ABC-123]"},
			expectedTitle:  "fix that [ABC-123]",
		},
		{
			name:           "not in the title",
			commitMessages: []string{"[ABC-456] c", "fix that"},
			expectedTitle:  "[ABC-456] c",
		},
		{
			name:           "twice in the title",
			commitMessages: []string{"[ABC-456] c", "[ABC-123] fix that [DEF-678]"},
			expectedTitle:  "[ABC-123] fix that [DEF-678]",
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
				Body:  &body,
				Draft: &draft,
			}

			r.CreatePrAssertPrDetails(t, "HEAD", 2, prDetails)
		})
	}
}

func TestPrTitle_StoryFromCommitMessages(t *testing.T) {
	remote := "cupcicm/pr/2"
	base := "master"
	body := ""
	draft := false

	rawTitle := "my super long title" //Story not in rawTitle

	testCases := []struct {
		name           string
		commitMessages []string
		expectedTitle  string
	}{
		{
			name:           "single other commit with story",
			commitMessages: []string{"[ABC-345] fix that", rawTitle},
			expectedTitle:  fmt.Sprintf("[ABC-345] %s", rawTitle),
		},
		{
			name:           "single other commit without story",
			commitMessages: []string{"fix that", rawTitle},
			expectedTitle:  rawTitle,
		},
		{
			name:           "several other commits with one story",
			commitMessages: []string{"[ABC-345] fix that", "do that", rawTitle},
			expectedTitle:  fmt.Sprintf("[ABC-345] %s", rawTitle),
		},
		{
			name:           "several other commits without any story",
			commitMessages: []string{"fix that", "do that", rawTitle},
			expectedTitle:  rawTitle,
		},
		{
			name:           "several other commits with several stories",
			commitMessages: []string{"[ABC-345] fix that", "[DEF-678] do that", rawTitle},
			expectedTitle:  fmt.Sprintf("[ABC-345] %s", rawTitle), // Take first occurence
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
				Body:  &body,
				Draft: &draft,
			}

			r.CreatePrAssertPrDetails(t, "HEAD", 2, prDetails)
		})
	}
}
