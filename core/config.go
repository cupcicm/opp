package core

import (
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

func GetGithubTokenCmd() string {
	return viper.GetString("github.token-cmd")
}

func HasGithubTokenConfigured() bool {
	return viper.GetString("github.token") != "" || GetGithubTokenCmd() != ""
}

func GetGithubToken() string {
	tokenCmd := GetGithubTokenCmd()
	if tokenCmd != "" {
		cmd := exec.Command("bash", "-c", tokenCmd)
		output, err := cmd.Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
				panic(fmt.Errorf("failed to get github token from command %q: %w (stderr: %s)", tokenCmd, err, strings.TrimSpace(string(exitErr.Stderr))))
			}
			panic(fmt.Errorf("failed to get github token from command %q: %w", tokenCmd, err))
		}
		return strings.TrimSpace(string(output))
	}
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
