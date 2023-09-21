package cmd_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
)

func TestCanCreatePR(t *testing.T) {
	r := tests.NewTestRepo(t)

	localPr := r.CreatePr(t, "HEAD^", 2)

	assert.True(t, localPr.HasState)
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

	assert.True(t, localPr.HasState)
	ancestor, err := localPr.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "master", ancestor.LocalName())
	}
	prTip := core.Must(r.Repo.GetLocalTip(localPr))
	commits := core.Must(r.Repo.GetCommitsNotInBaseBranch(prTip.Hash))
	fmt.Println("pr")
	fmt.Println(prTip.Hash.String())
	// There are 5 commits prepared in the test repo. We removed 2 by detaching to HEAD^^.
	// There should be 3 left.
	assert.Equal(t, 3, len(commits))
}

func TestCanChangePrAncestor(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^", 2)
	rebasedOnMaster := r.CreatePr(t, "HEAD", 3, "--base", "master")

	assert.True(t, rebasedOnMaster.HasState)
	ancestor, err := rebasedOnMaster.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "master", ancestor.LocalName())
	}
	prTip := core.Must(r.Repo.GetLocalTip(rebasedOnMaster))
	commits := core.Must(r.Repo.GetCommitsNotInBaseBranch(prTip.Hash))
	// There are 5 commits prepared in the test repo. We removed 2 by detaching to HEAD^^.
	// There should be 3 left.
	assert.Equal(t, 1, len(commits))
}
