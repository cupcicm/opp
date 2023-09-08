package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitCommand(repo *core.Repo) *cobra.Command {
	return &cobra.Command{
		Use: "init",
		Run: func(cmd *cobra.Command, args []string) {
			config := repo.Config()
			if _, err := os.Stat(config); err == nil {
				panic("Config file already exists")
			}
			os.Mkdir(path.Dir(config), 0755)

			i := initializer{repo}
			i.AskGithubToken()
			i.GuessRepoValues()
			i.GetGithubValues(cmd.Context())

			if err := viper.SafeWriteConfig(); err != nil {
				panic(err)
			}
		},
	}
}

type initializer struct {
	Repo *core.Repo
}

func (i *initializer) AskGithubToken() {
	reader := bufio.NewReader(os.Stdin)
	if viper.GetString("github.token") == "" {
		fmt.Println("Please enter a personal github token.")
		fmt.Println("You can create one at https://github.com/settings/tokens.")
		fmt.Println(`It needs to have all of the "repo" permissions checked,`)
		fmt.Println(`and the "discussion:write" permission.`)
		fmt.Print("Your github token: ")
		token := strings.TrimSpace(core.Must(reader.ReadString('\n')))
		viper.Set("github.token", token)
	}
}

func (i *initializer) GuessRepoValues() {
	remoteName, githubRepo := i.extractGithubRepo()
	viper.Set("repo.github", githubRepo)
	viper.Set("repo.remote", remoteName)

	githubHead := core.Must(i.Repo.Reference(plumbing.NewRemoteHEADReferenceName(remoteName), false))
	mainRef := githubHead.Target().Short()
	mainBranch := mainRef[strings.Index(mainRef, "/")+1:]
	viper.Set("repo.branch", mainBranch)
}

func (i *initializer) extractGithubRepo() (string, string) {
	found := false
	var result string
	var remoteName string
	for _, remote := range core.Must(i.Repo.Remotes()) {
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
			if dotGit == -1 {
				result = url[index+len("github.com")+1:]
			} else {
				result = url[index+len("github.com")+1 : dotGit]
			}
			remoteName = remote.Config().Name
		}
	}
	return remoteName, result
}

func (i *initializer) GetGithubValues(ctx context.Context) {
	client := core.NewClient(ctx)

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		panic(err)
	}
	viper.Set("github.login", user.Login)
}
