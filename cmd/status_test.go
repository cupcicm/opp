package cmd_test

import (
	"strings"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestStatus(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^^^^", 2)
	r.CreatePr(t, "HEAD^^^", 3)
	r.CreatePr(t, "HEAD^^", 4)
	r.CreatePr(t, "HEAD^", 5)

	pr4 := plumbing.NewBranchReferenceName(core.LocalBranchForPr(4))
	pr3 := plumbing.NewBranchReferenceName(core.LocalBranchForPr(3))
	r.Repo.Storer.SetReference(plumbing.NewSymbolicReference(pr4, pr3))

	r.GithubMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.CallGetAndReturnMergeable(3, false)
	r.GithubMock.CallGetAndReturnMergeable(4, true)
	r.GithubMock.CallGetAndReturnMergeable(5, false)

	assert.Nil(t, r.Run("status"))
	assert.Equal(t, strings.TrimSpace(`
	PR chain #2
  1. https://github.com/cupcicm/opp/pull/2
     mergeable  ✅
     up-to-date ✅
  2. https://github.com/cupcicm/opp/pull/3
     mergeable  ❌ - cannot be merged cleanly into master
     up-to-date ✅
  3. https://github.com/cupcicm/opp/pull/4
     mergeable  ✅
     up-to-date ❌
  4. https://github.com/cupcicm/opp/pull/5
     mergeable  ❌ - cannot be merged cleanly into master
     up-to-date ✅`), strings.TrimSpace(r.Out.String()))
}
