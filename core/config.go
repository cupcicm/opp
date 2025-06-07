package core

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

func init() {
	viper.SetDefault("github.merge.method", "rebase")
	viper.SetDefault("github.timeout", 30*time.Second)
	viper.SetDefault("story.enrich", true)
}

func GetGithubToken() string {
	return viper.GetString("github.token")
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

func GetGithubMergeMethod() string {
	return viper.GetString("github.merge.method")
}

func GetGithubTimeout() time.Duration {
	return viper.GetDuration("github.timeout")
}

func GetStoryTool() string {
	// Check new config format first
	if viper.IsSet("story.jira.token") {
		return "jira"
	}
	if viper.IsSet("story.linear.token") {
		return "linear"
	}

	// Fallback to old config format
	return viper.GetString("story.tool")
}

func GetStoryToolUrl() string {
	return viper.GetString("story.url")
}

func GetStoryToolToken() string {
	// Check new config format first
	if viper.IsSet("story.jira.token") {
		return viper.GetString("story.jira.token")
	}
	if viper.IsSet("story.linear.token") {
		return viper.GetString("story.linear.token")
	}

	// Fallback to old config format
	return viper.GetString("story.token")
}

func EnrichPrBodyWithStoryEnabled() bool {
	return viper.GetBool("story.enrich")
}

// New Jira-specific getters
func GetJiraEmail() string {
	return viper.GetString("story.jira.email")
}

func GetJiraHost() string {
	return viper.GetString("story.jira.host")
}
