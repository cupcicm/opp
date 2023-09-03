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

	r.GithubMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.CallGetAndReturnMergeable(3, true)

	assert.NotNil(t, cmd.MergeCommand(r.Repo, r.GithubMock).Execute())
}

func TestMerges(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD", 2)
	r.Repo.Checkout(pr2)

	r.GithubMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.CallMerge(2)

	assert.Nil(t, cmd.MergeCommand(r.Repo, r.GithubMock).Execute())

	// Assert that we have cleaned the local PR
	assert.Empty(t, core.Must(r.AllLocalPrs()))
}
