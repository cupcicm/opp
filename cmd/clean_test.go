package cmd_test

import (
	"context"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
)

func TestCleanupDoesntCleanupOtherPrs(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD^", 2)

	r.DeleteRemoteBranch(context.Background(), pr2)
	_, err := r.GetLocalTip(pr2)
	assert.True(t, core.FileExists(pr2.StateFile()))
	assert.Nil(t, err)

	assert.Nil(t, r.Run("clean"))

	_, err = r.GetLocalTip(pr2)
	assert.NotNil(t, err)
	assert.False(t, core.FileExists(pr2.StateFile()))
}

func TestCleanupCleansDependencies(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)

	r.DeleteRemoteBranch(context.Background(), pr2)
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(3, true)

	assert.Nil(t, r.Run("clean"))

	_, err := r.GetLocalTip(pr2)
	assert.NotNil(t, err)
	assert.False(t, core.FileExists(pr2.StateFile()))
	pr3.ReloadState()
	ancestor, _ := pr3.GetAncestor()
	assert.Equal(t, "master", ancestor.LocalName())
}
