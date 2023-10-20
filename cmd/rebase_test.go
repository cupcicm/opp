package cmd_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/go-git/go-git/v5"
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

func TestRebaseFindsPreviousTips(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)

	r.Checkout(pr2)

	os.WriteFile(path.Join(r.Path(), "3"), []byte("amended 3"), 0644)
	r.Worktree().Add("3")
	r.RewriteLastCommit("amended 3")
	assert.NoError(t, r.Run("push"))
	r.Checkout(pr3)

	// The status now is that pr/3 depends on an old version of pr/2
	// the commit in pr/2 has been amended to a new one.
	// However, pr/2 still remembers its old tip, so rebasing pr/3 will
	// see pr/3 point to commit "4" on top of the "amended 3" commit
	assert.NoError(t, r.Run("rebase"))
	commits := core.Must(r.Log(&git.LogOptions{}))
	expectedCommitMessages := []string{"4", "amended 3", "2", "1", "0"}
	for i := 0; i < 5; i++ {
		c := core.Must(commits.Next())
		assert.Equal(t, expectedCommitMessages[i], strings.TrimSpace(c.Message))
	}
	r.Checkout(pr2)
	pr2Commits := core.Must(r.Log(&git.LogOptions{}))
	for i := 0; i < 4; i++ {
		c := core.Must(pr2Commits.Next())
		assert.Equal(t, expectedCommitMessages[i+1], strings.TrimSpace(c.Message))
	}
}

func TestRebaseAbandonsGracefully(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)

	r.Checkout(pr2)

	os.WriteFile(path.Join(r.Path(), "4"), []byte("conflicts with 4"), 0644)
	r.Worktree().Add("4")
	r.RewriteLastCommit("conflicts with 4")
	r.MergePr(t, pr2)
	r.Checkout(pr3)
	fmt.Println(r.Path())

	// The new version of commit 3 modifies file 4, so it will conflict with commit 4
	// Assert the rebase command fails, and that a rebase is in progress.
	assert.Error(t, r.Run("rebase"))
	assert.DirExists(t, path.Join(r.Path(), ".git", "rebase-merge"))
	rebaseTodo := string(core.Must(os.ReadFile(path.Join(r.Path(), ".git", "rebase-merge", "git-rebase-todo.backup"))))

	// Also, the file should contain "pick commit 3" and "pick commit 4"
	assert.Regexp(t, "pick [0-9a-z]+ 3", rebaseTodo)
	assert.Regexp(t, "pick [0-9a-z]+ 4", rebaseTodo)
}

func TestRebaseFindsTipWhenMerged(t *testing.T) {
	r := tests.NewTestRepo(t)

	pr2 := r.CreatePr(t, "HEAD^", 2)
	pr3 := r.CreatePr(t, "HEAD", 3)

	r.Checkout(pr2)

	os.WriteFile(path.Join(r.Path(), "3"), []byte("amended 3"), 0644)
	r.Worktree().Add("3")
	r.RewriteLastCommit("amended 3")
	r.MergePr(t, pr2)
	r.Checkout(pr3)
	fmt.Println(r.Path())

	// The status now is that pr/3 depends on an old version of pr/2
	// the commit in pr/2 has been amended to a new one, then pr/2 was merged.
	// However, pr/3 remembers the old tip of pr/2, so rebasing pr/3 will
	// see pr/3 point to commit "4" on top of the "amended 3" commit
	assert.NoError(t, r.Run("rebase"))
	commits := core.Must(r.Log(&git.LogOptions{}))
	expectedCommitMessages := []string{"4", "amended 3", "2", "1", "0"}
	for i := 0; i < 5; i++ {
		c := core.Must(commits.Next())
		assert.Equal(t, expectedCommitMessages[i], strings.TrimSpace(c.Message))
	}
}
