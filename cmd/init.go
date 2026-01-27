package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v3"
)

const ErrorPattern = "could not %s a global gitignore file, please add .opp to your .gitignore file manually"

func InitCommand(repo *core.Repo) *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "indicate you are going to use opp in this git repo",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			config := repo.Config()
			if _, err := os.Stat(config); err == nil {
				return cli.Exit("Config file already exists", 1)
			}
			os.Mkdir(path.Dir(config), 0755)

			i := initializer{repo}
			i.AskGithubToken()
			i.GuessRepoValues()
			i.GetGithubValues(ctx)
			err := i.AddOppInGlobalGitignore(ctx)
			if err != nil {
				fmt.Printf("%v\n", err)
			}

			if err := viper.SafeWriteConfig(); err != nil {
				return cli.Exit(fmt.Errorf("could not write config file: %w", err), 1)
			}
			return nil
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
		fmt.Println(`and the "write:discussion" permission.`)
		fmt.Print("Your github token: ")
		token := strings.TrimSpace(core.Must(reader.ReadString('\n')))
		viper.Set("github.token", token)
	}
}

func (i *initializer) GuessRepoValues() {
	remoteName, githubRepo := i.extractGithubRepo()
	viper.Set("repo.github", githubRepo)
	viper.Set("repo.remote", remoteName)

	// Get the remote HEAD to determine default branch
	cmd := i.Repo.GitExec(context.Background(), "symbolic-ref refs/remotes/%s/HEAD", remoteName)
	output, err := cmd.Output()
	if err != nil {
		// If we can't determine, default to main
		viper.Set("repo.branch", "main")
		return
	}
	refName := strings.TrimSpace(string(output))
	// refName will be like "refs/remotes/origin/main"
	parts := strings.Split(refName, "/")
	if len(parts) > 0 {
		mainBranch := parts[len(parts)-1]
		viper.Set("repo.branch", mainBranch)
	}
}

func (i *initializer) extractGithubRepo() (string, string) {
	found := false
	var result string
	var remoteName string

	// Get list of remotes
	cmd := i.Repo.GitExec(context.Background(), "remote")
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	remotes := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, remote := range remotes {
		if remote == "" {
			continue
		}
		// Get URL for this remote
		cmd := i.Repo.GitExec(context.Background(), "remote get-url %s", remote)
		urlBytes, err := cmd.Output()
		if err != nil {
			continue
		}
		url := strings.TrimSpace(string(urlBytes))
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
			remoteName = remote
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

func (i *initializer) GlobalGitignorePath(ctx context.Context) (string, error) {
	cmd := i.Repo.GitExec(ctx, "config --get core.excludesfile")
	output, err := cmd.Output()
	if err != nil {
		cmd := i.Repo.GitExec(ctx, "config core.excludesfile ~/.gitignore_global")
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf(ErrorPattern, "create")
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("could not find user's home directory")
		}
		return path.Join(home, ".gitignore_global"), nil
	}
	return strings.TrimSpace(string(output)), nil
}

func (i *initializer) AddOppInGlobalGitignore(ctx context.Context) error {
	gitignore, err := i.GlobalGitignorePath(ctx)
	if err != nil {
		return err
	}
	file, err := os.ReadFile(gitignore)

	if err != nil {
		if os.IsNotExist(err) {
			// File does not exist, it's OK
			file = []byte{}
		} else {
			return fmt.Errorf(ErrorPattern, "read")
		}
	}
	lines := strings.Split(string(file), "\n")
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, ".opp") {
			found = true
		}
	}
	if !found {
		lines = append(lines,
			"# Ignore the opp folder, see https://github.com/cupcicm/opp",
			".opp/",
		)
	}
	if !strings.HasSuffix(lines[len(lines)-1], "\n") {
		lines[len(lines)-1] = lines[len(lines)-1] + "\n"
	}
	if os.WriteFile(gitignore, []byte(strings.Join(lines, "\n")), 0644) != nil {
		return fmt.Errorf(ErrorPattern, "write")
	}
	return nil
}
