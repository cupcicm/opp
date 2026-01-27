package cmd_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
)

func TestStatus(t *testing.T) {
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^^^^", 2)
	r.CreatePr(t, "HEAD^^^", 3)
	r.CreatePr(t, "HEAD^^", 4)
	r.CreatePr(t, "HEAD^", 5)

	pr4 := core.LocalBranchForPr(4)
	pr3 := core.LocalBranchForPr(3)
	// Create a symbolic reference using git command
	cmd := r.Repo.GitExec(context.Background(), "symbolic-ref refs/heads/%s refs/heads/%s", pr4, pr3)
	_ = cmd.Run()

	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(3, false)
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(4, true)
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(5, false)

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
