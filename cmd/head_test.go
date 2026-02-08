package cmd_test

import (
	"context"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
)

func TestGetHeadHash(t *testing.T) {
	tmpDir := t.TempDir()

	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	testFile := path.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	repo := core.NewRepo(tmpDir)
	ctx := context.Background()

	hash, err := repo.GetHeadHash(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 40, "SHA should be 40 characters")

	// Create another commit and verify HEAD changes
	os.WriteFile(testFile, []byte("test2"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "second").Run()

	newHash, err := repo.GetHeadHash(ctx)
	assert.NoError(t, err)
	assert.NotEqual(t, hash, newHash, "HEAD hash should change after new commit")
}

func TestGetCurrentBranchName(t *testing.T) {
	tmpDir := t.TempDir()

	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	testFile := path.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	repo := core.NewRepo(tmpDir)
	ctx := context.Background()

	branchName, err := repo.GetCurrentBranchName(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, branchName)
	assert.Contains(t, []string{"master", "main"}, branchName)

	// Create and checkout a new branch
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature-branch").Run()

	branchName2, err := repo.GetCurrentBranchName(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "feature-branch", branchName2)

	// Detach HEAD
	exec.Command("git", "-C", tmpDir, "checkout", "--detach", "HEAD").Run()

	_, err = repo.GetCurrentBranchName(ctx)
	assert.Error(t, err, "Should return error when HEAD is detached")
	assert.Contains(t, err.Error(), "detached")
}

func TestGetRefHash(t *testing.T) {
	r := tests.NewTestRepo(t)
	ctx := context.Background()

	// Test getting HEAD hash via refs/heads/master
	masterHash, err := r.Repo.GetRefHash(ctx, "refs/heads/master")
	assert.NoError(t, err)
	assert.NotEmpty(t, masterHash)
	assert.Len(t, masterHash, 40, "SHA should be 40 characters")

	// Test getting remote ref hash
	remoteHash, err := r.Repo.GetRefHash(ctx, "refs/remotes/origin/master")
	assert.NoError(t, err)
	assert.NotEmpty(t, remoteHash)

	// Test non-existent reference
	_, err = r.Repo.GetRefHash(ctx, "refs/heads/nonexistent")
	assert.Error(t, err, "Should return error for non-existent ref")
	assert.Contains(t, err.Error(), "not found")

	// Create a PR branch and test it
	pr := r.CreatePr(t, "HEAD", 1)
	prHash, err := r.Repo.GetRefHash(ctx, "refs/heads/"+pr.LocalName())
	assert.NoError(t, err)
	assert.NotEmpty(t, prHash)
}

func TestGetHeadRef(t *testing.T) {
	ctx := context.Background()
	r := tests.NewTestRepo(t)

	// On a branch: returns branch name
	currentBranch, err := r.Repo.GetCurrentBranchName(ctx)
	assert.NoError(t, err)

	headRef, err := r.Repo.GetHeadRef(ctx)
	assert.NoError(t, err)
	assert.Equal(t, currentBranch, headRef)

	// Detached HEAD: returns commit hash
	hash := core.Must(r.Repo.GetHeadHash(ctx))
	exec.Command("git", "-C", r.Path(), "checkout", "--detach", "HEAD").Run()

	headRef, err = r.Repo.GetHeadRef(ctx)
	assert.NoError(t, err)
	assert.Equal(t, hash, headRef)
}

func TestCheckoutRef(t *testing.T) {
	ctx := context.Background()
	r := tests.NewTestRepo(t)

	// Checkout a new branch by name
	exec.Command("git", "-C", r.Path(), "branch", "test-branch").Run()
	err := r.Repo.CheckoutRef(ctx, "test-branch")
	assert.NoError(t, err)
	branch, err := r.Repo.GetCurrentBranchName(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "test-branch", branch)

	// Checkout by commit hash (detached)
	hash := core.Must(r.Repo.GetHeadHash(ctx))
	r.Repo.CheckoutRef(ctx, "master")
	err = r.Repo.CheckoutRef(ctx, hash)
	assert.NoError(t, err)
	_, err = r.Repo.GetCurrentBranchName(ctx)
	assert.Error(t, err, "Should be in detached HEAD")
}

func TestDeleteLocalAndRemoteBranch(t *testing.T) {
	ctx := context.Background()
	r := tests.NewTestRepo(t)

	pr := r.CreatePr(t, "HEAD", 1)

	_, err := r.Repo.GetRefHash(ctx, "refs/heads/"+pr.LocalBranch())
	assert.NoError(t, err)

	r.Repo.DeleteLocalAndRemoteBranch(ctx, pr)

	_, err = r.Repo.GetRefHash(ctx, "refs/heads/"+pr.LocalBranch())
	assert.ErrorIs(t, err, core.ErrReferenceNotFound)
}

func TestAllLocalPrs(t *testing.T) {
	r := tests.NewTestRepo(t)

	prs, err := r.Repo.AllLocalPrs()
	assert.NoError(t, err)
	assert.Empty(t, prs)

	pr1 := r.CreatePr(t, "HEAD^^", 1)
	pr2 := r.CreatePr(t, "HEAD^", 2)
	r.CreatePr(t, "HEAD", 3)

	prs, err = r.Repo.AllLocalPrs()
	assert.NoError(t, err)
	assert.Len(t, prs, 3)

	pr1Hash := core.Must(r.Repo.GetRefHash(context.Background(), "refs/heads/"+pr1.LocalBranch()))
	pr2Hash := core.Must(r.Repo.GetRefHash(context.Background(), "refs/heads/"+pr2.LocalBranch()))
	assert.Equal(t, pr1Hash, prs[1])
	assert.Equal(t, pr2Hash, prs[2])
}

func TestIsAncestor(t *testing.T) {
	ctx := context.Background()
	r := tests.NewTestRepo(t)

	head := core.Must(r.Repo.GetHeadHash(ctx))
	parent := core.Must(r.Repo.GetRefHash(ctx, "HEAD^"))

	assert.True(t, r.Repo.IsAncestor(ctx, parent, head))
	assert.False(t, r.Repo.IsAncestor(ctx, head, parent))

	assert.True(t, r.Repo.IsAncestor(ctx, head, head))

	// Test with merge commits
	exec.Command("git", "-C", r.Path(), "checkout", "-b", "side-branch").Run()
	os.WriteFile(path.Join(r.Path(), "side.txt"), []byte("side"), 0644)
	exec.Command("git", "-C", r.Path(), "add", "side.txt").Run()
	exec.Command("git", "-C", r.Path(), "commit", "-m", "side commit").Run()
	exec.Command("git", "-C", r.Path(), "checkout", "master").Run()
	exec.Command("git", "-C", r.Path(), "merge", "--no-ff", "side-branch", "-m", "merge side").Run()

	newHead := core.Must(r.Repo.GetHeadHash(ctx))
	assert.True(t, r.Repo.IsAncestor(ctx, head, newHead), "commit before merge should be ancestor of merge commit")
	assert.False(t, r.Repo.IsAncestor(ctx, newHead, head), "merge commit should not be ancestor of earlier commit")

	sideHash := core.Must(r.Repo.GetRefHash(ctx, "side-branch"))
	assert.True(t, r.Repo.IsAncestor(ctx, sideHash, newHead), "side branch commit should be ancestor of merge commit")
}

func TestGetMainBranch(t *testing.T) {
	ctx := context.Background()
	r := tests.NewTestRepo(t)

	branch, err := r.Repo.GetMainBranch(ctx, "origin")
	assert.NoError(t, err)
	assert.Equal(t, "master", branch)
}
