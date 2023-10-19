package cmd_test

import (
	"context"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestRebaseCleansDependentBranches(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)

	// PR 2 gets merged into master.
	tip := core.Must(r.GetLocalTip(pr2))
	assert.Nil(t, r.Push(context.Background(), tip.Hash, "master"))

	r.Checkout(pr3)

	assert.Nil(t, r.Run("rebase"))

	_, err := r.Source.Reference(plumbing.NewBranchReferenceName(pr2.LocalBranch()), true)
	assert.Equal(t, plumbing.ErrReferenceNotFound, err)

	// Reload PR3 state.
	pr3 = core.NewLocalPr(r.Repo, 3)
	assert.True(t, pr3.HasState)

	assert.Equal(t, "master", core.Must(pr3.GetAncestor()).LocalName())
}
