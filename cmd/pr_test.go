package cmd_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/story"
	"github.com/cupcicm/opp/core/tests"
	"github.com/google/go-github/v56/github"
	"github.com/stretchr/testify/assert"
)

func TestCanCreatePR(t *testing.T) {
	r := tests.NewTestRepo(t)

	localPr := r.CreatePr(t, "HEAD^", 2)

	assert.True(t, localPr.StateIsLoaded())
	ancestor, err := localPr.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "master", ancestor.LocalName())
	}
}

func TestCanCreatePRWhenNotOnBranch(t *testing.T) {
	r := tests.NewTestRepo(t)

	// Go in detached HEAD mode.
	r.Repo.GitExec(context.Background(), "checkout %s", "HEAD^^").Run()
	fmt.Println("after")

	localPr := r.CreatePr(t, "HEAD", 2)

	assert.True(t, localPr.StateIsLoaded())
	ancestor, err := localPr.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "master", ancestor.LocalName())
	}
	prTip := core.Must(r.Repo.GetLocalTip(localPr))
	commits := core.Must(r.Repo.GetCommitsNotInBaseBranch(prTip))
	fmt.Println("pr")
	fmt.Println(prTip)
	// There are 5 commits prepared in the test repo. We removed 2 by detaching to HEAD^^.
	// There should be 3 left.
	assert.Equal(t, 3, len(commits))
}

func TestCanChangePrAncestor(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^", 2)
	rebasedOnMaster := r.CreatePr(t, "HEAD", 3, "--base", "master")

	assert.True(t, rebasedOnMaster.StateIsLoaded())
	ancestor, err := rebasedOnMaster.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "master", ancestor.LocalName())
	}
	prTip := core.Must(r.Repo.GetLocalTip(rebasedOnMaster))
	commits := core.Must(r.Repo.GetCommitsNotInBaseBranch(prTip))
	// From HEAD^ to HEAD there is only 1 commit.
	assert.Equal(t, 1, len(commits))
}

func TestCanSetAncestor(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^", 2)
	rebasedOnMaster := r.CreatePr(t, "HEAD", 3, "--base", "2")

	assert.True(t, rebasedOnMaster.StateIsLoaded())
	ancestor, err := rebasedOnMaster.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "pr/2", ancestor.LocalName())
	}
}

func TestCanCreatePRFromPRBranchWithNewCommits(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^", 2)
	r.Repo.GitExec(context.Background(), "checkout pr/2").Run()

	// Add a commit on pr/2 without pushing - local pr/2 is now ahead of remote
	wt := core.Must(r.Source.Worktree())
	wt.Add("0")
	r.Commit("extra commit on pr/2")

	r.GithubMock.IssuesMock.CallListAndReturnPr(2)
	r.GithubMock.PullRequestsMock.CallCreate(3)
	r.StoryFetcherMock.CallFetchInProgressStories([]story.Story{}, false)
	err := r.Run("pr", "-i", "HEAD")
	assert.NoError(t, err)

	pr3 := r.AssertHasPr(t, 3)
	assert.True(t, pr3.StateIsLoaded())
	ancestor, err := pr3.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "pr/2", ancestor.LocalName())
	}
}

func TestErrorsWhenOnPRBranchWithNoNewCommits(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^", 2)
	r.Repo.GitExec(context.Background(), "checkout pr/2").Run()

	// No new commits - we're at remote tip; should error with ErrNoNewCommitsOnPrBranch
	err := r.Run("pr", "-i", "HEAD")
	assert.Error(t, err, "expected error when on PR branch with no new commits")
	if err != nil {
		assert.Contains(t, err.Error(), "no new local commits", "error should mention no new commits")
	}
}

func TestCanSetAncestorWithDraft(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^", 2)

	draft := true
	remote := "cupcicm/pr/3"
	base := "cupcicm/pr/2"
	title := "4"
	body := ""

	prDetails := github.NewPullRequest{
		Title: &title,
		Head:  &remote,
		Base:  &base,
		Body:  &body,
		Draft: &draft,
	}

	rebasedOnMaster := r.CreatePrAssertPrDetails(t, "HEAD", 3, prDetails, "--base", "2", "--draft")

	assert.True(t, rebasedOnMaster.StateIsLoaded())
	ancestor, err := rebasedOnMaster.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "pr/2", ancestor.LocalName())
	}
}
