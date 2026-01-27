package cmd_test

import (
	"testing"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
)

func TestCannotMergeIfDependentPRs(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)
	r.Repo.Checkout(pr3)

	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(3, true)

	assert.NotNil(t, r.Run("merge"))
}

func TestMergeKeepsTrackOfAncestorTips(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)
	r.Repo.Checkout(pr2)

	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.PullRequestsMock.CallMerge(2, "8f4ca5d979bc19b7c836655a6432d690f78316af")

	// Check that pr3 knows about the tip of its ancestor (pr2)
	assert.Len(t, pr3.AncestorTips(), 1)
	pr2Tip := pr3.AncestorTips()[0]
	assert.Equal(t, pr2Tip, core.Must(r.GetLocalTip(pr2)))

	assert.Nil(t, r.Run("merge"))

	// Assert that we have cleaned the local PR
	assert.Len(t, core.Must(r.AllLocalPrs()), 1)

	pr3.ReloadState()
	assert.Len(t, pr3.AncestorTips(), 2)
	assert.Contains(t, pr3.AncestorTips(), "8f4ca5d979bc19b7c836655a6432d690f78316af", pr2Tip)
}

func TestMergeWaitsForMergeability(t *testing.T) {
	r := tests.NewTestRepo(t)
	reset := cmd.SetShortMergeabilityIntervalForTests()
	defer reset()

	pr2 := r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)
	r.Repo.Checkout(pr2)

	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeabilityBeingEvaluated(2)
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeabilityBeingEvaluated(2)
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.PullRequestsMock.CallMerge(2, "8f4ca5d979bc19b7c836655a6432d690f78316af")

	// Check that pr3 knows about the tip of its ancestor (pr2)
	assert.Len(t, pr3.AncestorTips(), 1)
	pr2Tip := pr3.AncestorTips()[0]
	assert.Equal(t, pr2Tip, core.Must(r.GetLocalTip(pr2)))

	assert.Nil(t, r.Run("merge"))

	// Assert that we have cleaned the local PR
	assert.Len(t, core.Must(r.AllLocalPrs()), 1)

	pr3.ReloadState()
	assert.Len(t, pr3.AncestorTips(), 2)
	assert.Contains(t, pr3.AncestorTips(), "8f4ca5d979bc19b7c836655a6432d690f78316af", pr2Tip)
}
