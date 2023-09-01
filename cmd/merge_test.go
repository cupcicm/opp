package cmd_test

import (
	"fmt"
	"testing"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
)

func TestMergeability(t *testing.T) {
	a := []int{1, 2, 3}
	b := append(a, 4)
	b[1] = 0
	fmt.Printf("%v", a)
	fmt.Printf("%v", b)
	r := tests.NewTestRepo(t)

	r.CreatePr(t, "HEAD^^^", 2)
	r.CreatePr(t, "HEAD^^", 3)
	r.CreatePr(t, "HEAD^", 4)
	pr5 := r.CreatePr(t, "HEAD", 5)
	r.Repo.Checkout(pr5)

	r.GithubMock.CallGetAndReturnMergeable(2, true)
	r.GithubMock.CallGetAndReturnMergeable(3, false)
	r.GithubMock.CallGetAndReturnMergeable(4, true)
	r.GithubMock.CallGetAndReturnMergeable(5, true)

	assert.NotNil(t, cmd.MergeCommand(r.Repo, r.GithubMock).Execute())
}
