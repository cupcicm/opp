package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitCommand(repo *core.Repo) *cobra.Command {
	return &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			config := repo.Config()
			if _, err := os.Stat(config); err == nil {
				panic("Config file already exists")
			}
			os.Mkdir(path.Dir(config), 0755)

			reader := bufio.NewReader(os.Stdin)
			if viper.GetString("github.token") == "" {
				fmt.Println("Please enter a personal github token.")
				fmt.Println("You can create one at https://github.com/settings/tokens.")
				fmt.Print("Your github token: ")
				token := strings.TrimSpace(core.Must(reader.ReadString('\n')))
				viper.Set("github.token", token)
			}

			fmt.Println()
			fmt.Println("What is the name of the base branch for your PRs in this repository z? ")
			base := strings.TrimSpace(core.Must(reader.ReadString('\n')))
			viper.Set("repo.branch", base)
			client := core.NewClient(cmd.Context())

			user, _, err := client.Users.Get(cmd.Context(), "")
			if err != nil {
				panic(err)
			}
			viper.Set("github.login", user.Login)

			remoteName, githubRepo := extractGithubRepo(repo)
			viper.Set("repo.github", githubRepo)
			viper.Set("repo.remote", remoteName)

			if err := viper.SafeWriteConfig(); err != nil {
				panic(err)
			}
		},
	}
}

func extractGithubRepo(r *core.Repo) (string, string) {
	found := false
	var result string
	var remoteName string
	for _, remote := range core.Must(r.Remotes()) {
		urls := remote.Config().URLs
		if len(urls) == 0 {
			continue
		}
		url := urls[0]
		index := strings.Index(url, "github.com")
		dotGit := strings.LastIndex(url, ".git")
		if index > -1 {
			if found {
				// Second time we find a remote, not good.
				panic("two github remotes in this repo.")
			}
			found = true
			result = url[index+len("github.com")+1 : dotGit]
			remoteName = remote.Config().Name
		}
	}
	return remoteName, result
}
