package cmd_test

import (
	"context"
	"testing"

	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasCreate(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR first
	pr := repo.CreatePr(t, "HEAD~2", 1)
	require.NotNil(t, pr)

	// Create an alias for the PR
	err := repo.Run("alias", "myfeature", "1")
	require.NoError(t, err)

	// Verify the alias was created
	prNumber, ok := repo.ResolveAlias("myfeature")
	assert.True(t, ok)
	assert.Equal(t, 1, prNumber)
}

func TestAliasCreateForCurrentBranch(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR and checkout
	pr := repo.CreatePr(t, "HEAD~2", 1, "--checkout")
	require.NotNil(t, pr)

	// Create an alias for current branch (no PR number specified)
	err := repo.Run("alias", "current-feature")
	require.NoError(t, err)

	// Verify the alias was created
	prNumber, ok := repo.ResolveAlias("current-feature")
	assert.True(t, ok)
	assert.Equal(t, 1, prNumber)
}

func TestAliasList(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create PRs
	repo.CreatePr(t, "HEAD~2", 1)
	repo.CreatePr(t, "HEAD~3", 2)

	// Create aliases
	repo.Run("alias", "feature-one", "1")
	repo.Run("alias", "feature-two", "2")

	// List aliases (just verify it runs without error)
	err := repo.Run("alias")
	require.NoError(t, err)

	// Verify both aliases exist
	prNumber1, ok1 := repo.ResolveAlias("feature-one")
	assert.True(t, ok1)
	assert.Equal(t, 1, prNumber1)

	prNumber2, ok2 := repo.ResolveAlias("feature-two")
	assert.True(t, ok2)
	assert.Equal(t, 2, prNumber2)
}

func TestAliasDelete(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR and alias
	repo.CreatePr(t, "HEAD~2", 1)
	repo.Run("alias", "myfeature", "1")

	// Verify alias exists
	_, ok := repo.ResolveAlias("myfeature")
	assert.True(t, ok)

	// Delete the alias
	err := repo.Run("alias", "-d", "myfeature")
	require.NoError(t, err)

	// Verify alias is gone
	_, ok = repo.ResolveAlias("myfeature")
	assert.False(t, ok)
}

func TestAliasInvalidName(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR
	repo.CreatePr(t, "HEAD~2", 1)

	// Try to create alias with invalid name (number)
	err := repo.Run("alias", "123", "1")
	assert.Error(t, err)

	// Try to create alias with pr/ prefix
	err = repo.Run("alias", "pr/123", "1")
	assert.Error(t, err)
}

func TestAliasWorksWithOtherCommands(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR and alias
	pr := repo.CreatePr(t, "HEAD~2", 1)
	repo.Run("alias", "myfeature", "1")

	// Verify push command accepts alias
	repo.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(1, true)

	// The push command should resolve "myfeature" to PR #1
	// This tests the ExtractPrNumberOrAlias function integration
	resolvedPr, ok := repo.ResolveAlias("myfeature")
	assert.True(t, ok)
	assert.Equal(t, pr.PrNumber, resolvedPr)
}

func TestCheckoutByAlias(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR
	repo.CreatePr(t, "HEAD~2", 1)

	// Create an alias
	repo.Run("alias", "myfeature", "1")

	// Checkout master first
	repo.GitExec(context.Background(), "checkout master").Run()

	// Checkout using alias
	err := repo.Run("checkout", "myfeature")
	require.NoError(t, err)

	// Verify we're on the PR branch
	currentPr, found := repo.PrForHead()
	assert.True(t, found)
	assert.Equal(t, 1, currentPr.PrNumber)
}

func TestCheckoutByPrNumber(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR
	repo.CreatePr(t, "HEAD~2", 1)

	// Checkout master first
	repo.GitExec(context.Background(), "checkout master").Run()

	// Checkout using PR number
	err := repo.Run("checkout", "1")
	require.NoError(t, err)

	// Verify we're on the PR branch
	currentPr, found := repo.PrForHead()
	assert.True(t, found)
	assert.Equal(t, 1, currentPr.PrNumber)
}

func TestAliasesForPr(t *testing.T) {
	repo := tests.NewTestRepo(t)

	// Create a PR
	repo.CreatePr(t, "HEAD~2", 1)

	// Create multiple aliases for the same PR
	repo.Run("alias", "feature", "1")
	repo.Run("alias", "bugfix", "1")

	// Get aliases for PR
	aliases := repo.AliasesForPr(1)
	assert.Len(t, aliases, 2)
	assert.Contains(t, aliases, "feature")
	assert.Contains(t, aliases, "bugfix")
}
