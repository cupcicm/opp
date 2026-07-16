package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("repo.git-executable", "git")
	viper.SetDefault("github.merge.method", "rebase")
	viper.SetDefault("github.timeout", 30*time.Second)
	viper.SetDefault("repo.push-command", "push")
	viper.SetDefault("story.enrich", true)
}

func GetGitExecutable() string {
	return viper.GetString("repo.git-executable")
}

// GithubTokenNeedsRefresh returns a boolean indicating whether the
// token needs to be refreshed and a boolean indicating whether the token
// needs to be saved to the configuration after a refresh.
// If expires_after is not set, the token always needs to be refreshed,
// and never needs to be saved.
// If expires_after is set, the token needs to be refreshed and saved
// if it was fetched more than expires_after ago.
func GithubTokenNeedsRefresh() (bool, bool, error) {
	if viper.GetString("github.token_fetch.command") == "" {
		return false, false, nil
	}
	expiresAfterStr := viper.GetString("github.token_fetch.expires_after")
	if expiresAfterStr == "" {
		return true, false, nil
	}
	fetchedAtStr := viper.GetString("github.token_fetch.fetched_at")
	if fetchedAtStr == "" {
		return true, true, nil
	}

	fetchedAt, err := time.Parse(time.RFC3339, fetchedAtStr)
	if err != nil {
		return false, false, fmt.Errorf("failed to parse github.token_fetch.fetched_at: %w", err)
	}

	expiresAfter, err := time.ParseDuration(expiresAfterStr)
	if err != nil {
		return false, false, fmt.Errorf("failed to parse github.token_fetch.expires_after: %w", err)
	}
	return time.Since(fetchedAt) > expiresAfter, true, nil
}

func GetGithubToken(ctx context.Context) (string, error) {
	token := viper.GetString("github.token")
	needsRefresh, needsSave, err := GithubTokenNeedsRefresh()
	if err != nil {
		return "", err
	}
	if needsRefresh {
		if needsSave {
			fmt.Println("Refreshing Github token...")
		}
		fetchToken := viper.GetString("github.token_fetch.command")
		if fetchToken == "" {
			return "", fmt.Errorf("no github.token or github.token.fetch configured")
		}
		out, err := exec.CommandContext(ctx, "bash", "-c", fetchToken).Output()
		if err != nil {
			return "", fmt.Errorf("github.token.fetch command failed: %w", err)
		}
		token = strings.TrimSpace(string(out))
		if needsSave {
			viper.Set("github.token_fetch.fetched_at", time.Now().Format(time.RFC3339))
			viper.Set("github.token", token)
			if err := viper.WriteConfig(); err != nil {
				return "", fmt.Errorf("failed to write refreshed github.token to config: %w", err)
			}
		}
		return token, nil
	}
	return token, nil
}

func GetGithubUsername() string {
	return viper.GetString("github.login")
}

func GetGithubRepo() string {
	return viper.GetString("repo.github")
}

// The first part of the repo, before the slash.
func GetGithubOwner() string {
	repo := viper.GetString("repo.github")
	slash := strings.LastIndex(repo, "/")
	return repo[:slash]
}

// The second part of the repo, after the slash.
func GetGithubRepoName() string {
	repo := viper.GetString("repo.github")
	slash := strings.LastIndex(repo, "/")
	return repo[slash+1:]
}

func GetRemoteName() string {
	return viper.GetString("repo.remote")
}

func GetBaseBranch() string {
	return viper.GetString("repo.branch")
}

func GetPushCommand() string {
	return viper.GetString("repo.push-command")
}

func GetGithubMergeMethod() string {
	return viper.GetString("github.merge.method")
}

func GetGithubTimeout() time.Duration {
	return viper.GetDuration("github.timeout")
}

func GetStoryTool() string {
	return viper.GetString("story.tool")
}

func GetStoryToolUrl() string {
	return viper.GetString("story.url")
}

func EnrichPrBodyWithStoryEnabled() bool {
	return viper.GetBool("story.enrich")
}

func GetStoryToolToken() string {
	return viper.GetString("story.token")
}
