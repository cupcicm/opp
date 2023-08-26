package core

import (
	"strings"

	"github.com/spf13/viper"
)

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
