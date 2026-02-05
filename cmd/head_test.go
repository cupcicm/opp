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
	assert.NotEmpty(t, hash.String())
	assert.Len(t, hash.String(), 40, "SHA should be 40 characters")

	// Verify it matches what go-git's Head() returns
	head := core.Must(repo.Head())
	assert.Equal(t, head.Hash(), hash, "GetHeadHash should return the same hash as Repository.Head().Hash()")

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
	assert.NotEmpty(t, masterHash.String())
	assert.Len(t, masterHash.String(), 40, "SHA should be 40 characters")

	// Verify it matches go-git's Reference()
	ref := core.Must(r.Repo.Reference(plumbing.NewBranchReferenceName("master"), true))
	assert.Equal(t, ref.Hash(), masterHash, "GetRefHash should return same hash as Repository.Reference()")

	// Test getting remote ref hash
	remoteHash, err := r.Repo.GetRefHash(ctx, "refs/remotes/origin/master")
	assert.NoError(t, err)
	assert.NotEmpty(t, remoteHash.String())

	// Verify it matches go-git's Reference()
	remoteRef := core.Must(r.Repo.Reference(plumbing.NewRemoteReferenceName("origin", "master"), true))
	assert.Equal(t, remoteRef.Hash(), remoteHash)

	// Test non-existent reference
	_, err = r.Repo.GetRefHash(ctx, "refs/heads/nonexistent")
	assert.Error(t, err, "Should return error for non-existent ref")
	assert.Contains(t, err.Error(), "not found")

	// Create a PR branch and test it
	pr := r.CreatePr(t, "HEAD", 1)
	prHash, err := r.Repo.GetRefHash(ctx, "refs/heads/"+pr.LocalName())
	assert.NoError(t, err)
	assert.NotEmpty(t, prHash.String())

	// Verify matches go-git
	prRef := core.Must(r.Repo.Reference(plumbing.NewBranchReferenceName(pr.LocalName()), true))
	assert.Equal(t, prRef.Hash(), prHash)
}

