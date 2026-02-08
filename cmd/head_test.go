package cmd_test

import (
	"context"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/tests"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestGetHeadHash(t *testing.T) {
	// Create a minimal test repo without the full PrepareSource setup
	tmpDir := t.TempDir()

	// Initialize a git repo
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create an initial commit
	testFile := path.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Open with go-git
	gitRepo, err := git.PlainOpen(tmpDir)
	assert.NoError(t, err)

	repo := core.NewRepoFromGitRepo(gitRepo)

	// Get HEAD hash using our new method
	ctx := context.Background()
	hash, err := repo.GetHeadHash(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 40, "SHA should be 40 characters")

	// Verify it matches what go-git's Head() returns
	head := core.Must(repo.Head())
	assert.Equal(t, head.Hash().String(), hash, "GetHeadHash should return the same hash as Repository.Head().Hash()")

	// Create another commit
	os.WriteFile(testFile, []byte("test2"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "second").Run()

	// Re-open repo to get new HEAD
	gitRepo2, _ := git.PlainOpen(tmpDir)
	repo2 := core.NewRepoFromGitRepo(gitRepo2)

	newHash, err := repo2.GetHeadHash(ctx)
	assert.NoError(t, err)
	assert.NotEqual(t, hash, newHash, "HEAD hash should change after new commit")
}

func TestGetCurrentBranchName(t *testing.T) {
	// Create a minimal test repo
	tmpDir := t.TempDir()

	// Initialize a git repo
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create an initial commit
	testFile := path.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "initial").Run()

	// Open with go-git
	gitRepo, err := git.PlainOpen(tmpDir)
	assert.NoError(t, err)
	repo := core.NewRepoFromGitRepo(gitRepo)
	ctx := context.Background()

	// Should be on 'main' or 'master' branch by default
	branchName, err := repo.GetCurrentBranchName(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, branchName)
	// Git uses either 'master' or 'main' depending on version/config
	assert.Contains(t, []string{"master", "main"}, branchName)

	// Verify it matches go-git
	head := core.Must(repo.Head())
	assert.True(t, head.Name().IsBranch(), "HEAD should be on a branch")
	assert.Equal(t, head.Name().Short(), branchName)

	// Create and checkout a new branch
	exec.Command("git", "-C", tmpDir, "checkout", "-b", "feature-branch").Run()
	gitRepo2, _ := git.PlainOpen(tmpDir)
	repo2 := core.NewRepoFromGitRepo(gitRepo2)

	branchName2, err := repo2.GetCurrentBranchName(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "feature-branch", branchName2)

	// Detach HEAD
	exec.Command("git", "-C", tmpDir, "checkout", "--detach", "HEAD").Run()
	gitRepo3, _ := git.PlainOpen(tmpDir)
	repo3 := core.NewRepoFromGitRepo(gitRepo3)

	_, err = repo3.GetCurrentBranchName(ctx)
	assert.Error(t, err, "Should return error when HEAD is detached")
	assert.Contains(t, err.Error(), "detached")

	// Verify it matches go-git behavior
	head3 := core.Must(repo3.Head())
	assert.False(t, head3.Name().IsBranch(), "HEAD should not be on a branch when detached")
}

func TestGetRefHash(t *testing.T) {
	r := tests.NewTestRepo(t)
	ctx := context.Background()

	// Test getting HEAD hash via refs/heads/master
	masterHash, err := r.Repo.GetRefHash(ctx, "refs/heads/master")
	assert.NoError(t, err)
	assert.NotEmpty(t, masterHash)
	assert.Len(t, masterHash, 40, "SHA should be 40 characters")

	// Verify it matches go-git's Reference()
	ref := core.Must(r.Repo.Reference(plumbing.NewBranchReferenceName("master"), true))
	assert.Equal(t, ref.Hash().String(), masterHash, "GetRefHash should return same hash as Repository.Reference()")

	// Test getting remote ref hash
	remoteHash, err := r.Repo.GetRefHash(ctx, "refs/remotes/origin/master")
	assert.NoError(t, err)
	assert.NotEmpty(t, remoteHash)

	// Verify it matches go-git's Reference()
	remoteRef := core.Must(r.Repo.Reference(plumbing.NewRemoteReferenceName("origin", "master"), true))
	assert.Equal(t, remoteRef.Hash().String(), remoteHash)

	// Test non-existent reference
	_, err = r.Repo.GetRefHash(ctx, "refs/heads/nonexistent")
	assert.Error(t, err, "Should return error for non-existent ref")
	assert.Contains(t, err.Error(), "not found")

	// Create a PR branch and test it
	pr := r.CreatePr(t, "HEAD", 1)
	prHash, err := r.Repo.GetRefHash(ctx, "refs/heads/"+pr.LocalName())
	assert.NoError(t, err)
	assert.NotEmpty(t, prHash)

	// Verify matches go-git
	prRef := core.Must(r.Repo.Reference(plumbing.NewBranchReferenceName(pr.LocalName()), true))
	assert.Equal(t, prRef.Hash().String(), prHash)
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

	// Create a PR branch
	pr := r.CreatePr(t, "HEAD", 1)

	// Verify the branch exists
	_, err := r.Repo.GetRefHash(ctx, "refs/heads/"+pr.LocalBranch())
	assert.NoError(t, err)

	// Delete it
	r.Repo.DeleteLocalAndRemoteBranch(ctx, pr)

	// Verify the local branch is gone
	_, err = r.Repo.GetRefHash(ctx, "refs/heads/"+pr.LocalBranch())
	assert.ErrorIs(t, err, core.ErrReferenceNotFound)
}

func TestAllLocalPrs(t *testing.T) {
	r := tests.NewTestRepo(t)

	// No PRs initially
	prs, err := r.Repo.AllLocalPrs()
	assert.NoError(t, err)
	assert.Empty(t, prs)

	// Create some PRs
	pr1 := r.CreatePr(t, "HEAD^^", 1)
	pr2 := r.CreatePr(t, "HEAD^", 2)
	r.CreatePr(t, "HEAD", 3)

	prs, err = r.Repo.AllLocalPrs()
	assert.NoError(t, err)
	assert.Len(t, prs, 3)

	// Verify hashes match
	pr1Hash := core.Must(r.Repo.GetRefHash(context.Background(), "refs/heads/"+pr1.LocalBranch()))
	pr2Hash := core.Must(r.Repo.GetRefHash(context.Background(), "refs/heads/"+pr2.LocalBranch()))
	assert.Equal(t, pr1Hash, prs[1])
	assert.Equal(t, pr2Hash, prs[2])
}

func TestIsAncestor(t *testing.T) {
	ctx := context.Background()
	r := tests.NewTestRepo(t)

	// HEAD^ is an ancestor of HEAD
	head := core.Must(r.Repo.GetHeadHash(ctx))
	parent := core.Must(r.Repo.GetRefHash(ctx, "HEAD^"))

	assert.True(t, r.Repo.IsAncestor(ctx, parent, head))
	assert.False(t, r.Repo.IsAncestor(ctx, head, parent))

	// A commit is an ancestor of itself
	assert.True(t, r.Repo.IsAncestor(ctx, head, head))

	// Test with merge commits: create a branch, add a commit, merge it back
	exec.Command("git", "-C", r.Path(), "checkout", "-b", "side-branch").Run()
	os.WriteFile(path.Join(r.Path(), "side.txt"), []byte("side"), 0644)
	exec.Command("git", "-C", r.Path(), "add", "side.txt").Run()
	exec.Command("git", "-C", r.Path(), "commit", "-m", "side commit").Run()
	exec.Command("git", "-C", r.Path(), "checkout", "master").Run()
	exec.Command("git", "-C", r.Path(), "merge", "--no-ff", "side-branch", "-m", "merge side").Run()

	// Now we have: head -> ... -> merge commit -> side commit + original head
	// The original head (before merge) should be an ancestor of the new head (after merge)
	newHead := core.Must(r.Repo.GetHeadHash(ctx))
	assert.True(t, r.Repo.IsAncestor(ctx, head, newHead), "commit before merge should be ancestor of merge commit")
	assert.False(t, r.Repo.IsAncestor(ctx, newHead, head), "merge commit should not be ancestor of earlier commit")

	// The side branch commit should also be an ancestor of the merge
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
