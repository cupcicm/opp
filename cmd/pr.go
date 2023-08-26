package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

var ErrLostPCreationRaceCondition error = errors.New("lost race condition when creating PR")
var ErrLostPrCreationRaceConditionMultipleTimes error = errors.New("lost race condition when creating PR too many times, aborting")

func PrCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Aliases: []string{"pull-request", "new"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var repo = core.Current()
			var headCommit plumbing.Hash
			if len(args) == 0 {
				var head = core.Must(repo.Head())
				if !head.Name().IsBranch() {
					return errors.New("works only when on a branch")
				}
				headCommit = head.Hash()
			} else {
				hash, err := repo.ResolveRevision(plumbing.Revision(args[0]))
				if err != nil {
					return err
				}
				headCommit = *hash
			}
			ancestor, commits, err := repo.FindBranchingPoint(headCommit)
			if err != nil {
				return err
			}
			tip := core.Must(repo.GetLocalTip(ancestor))
			if ancestor.IsPr() && tip.Hash == headCommit {
				fmt.Println("You are on a branch that has already been pushed as a PR")
				fmt.Println("Use opp up to update that PR instead.")
				//return nil
			}
			// Create a new PR then.
			pr := createPr{Repo: repo, Github: core.NewClient(cmd.Context())}
			return pr.Create(cmd.Context(), headCommit, commits, ancestor)
		},
	}

	return cmd
}

type createPr struct {
	Repo   *core.Repo
	Github *github.Client
}

func RemoteBranch(branch string) string {
	return fmt.Sprintf("%s/%s", core.GetGithubUsername(), branch)
}

func (c *createPr) Create(ctx context.Context, hash plumbing.Hash, commits []*object.Commit, ancestor core.Branch) error {

	title, body := c.GetBodyAndTitle(commits)

	pr, err := c.create(ctx, hash, ancestor, title, body)
	if err != nil {
		return err
	}
	c.createLocalBranchForPr(*pr.Number, hash, ancestor)
	localPr := core.NewLocalPr(c.Repo, *pr.Number)
	localPr.SetAncestor(ancestor)
	c.Repo.SetTrackingBranch(localPr, ancestor)
	fmt.Printf("https://github.com/%s/pull/%d\n", core.GetGithubRepo(), *pr.Number)
	return err
}

func (c *createPr) GetBodyAndTitle(commits []*object.Commit) (string, string) {
	sort.Slice(commits, func(i, j int) bool {
		return len(commits[i].Message) > len(commits[j].Message)
	})
	title, body, _ := strings.Cut(commits[0].Message, "\n")
	return strings.TrimSpace(title), strings.TrimSpace(body)
}

func (c *createPr) createLocalBranchForPr(number int, hash plumbing.Hash, ancestor core.Branch) {
	c.Repo.CreateBranch(&config.Branch{
		Name:   core.LocalBranchForPr(number),
		Remote: core.RemoteBranchForPr(number),
		Merge:  plumbing.NewBranchReferenceName(ancestor.RemoteName()),
		Rebase: "true",
	})

	ref := plumbing.NewBranchReferenceName(core.LocalBranchForPr(number))
	c.Repo.Storer.SetReference(plumbing.NewHashReference(ref, hash))
}

func (c *createPr) create(ctx context.Context, hash plumbing.Hash, ancestor core.Branch, title string, body string) (*github.PullRequest, error) {
	for attempts := 0; attempts < 3; attempts++ {
		pr, err := c.createOnce(ctx, hash, ancestor, title, body)
		if err == nil {
			return pr, nil
		}
		if err != ErrLostPCreationRaceCondition {
			return nil, err
		}
	}
	return nil, ErrLostPrCreationRaceConditionMultipleTimes
}

func (c *createPr) createOnce(ctx context.Context, hash plumbing.Hash, ancestor core.Branch, title string, body string) (*github.PullRequest, error) {
	lastPr, err := c.getLastPrNumber(ctx)
	if err != nil {
		return nil, err
	}
	remote := core.RemoteBranchForPr(lastPr + 1)
	base := ancestor.RemoteName()
	err = c.Repo.Push(ctx, hash, remote)
	if err != nil {
		return nil, err
	}
	pull := github.NewPullRequest{
		Title: &title,
		Head:  &remote,
		Base:  &base,
		Body:  &body,
	}
	pr, _, err := c.Github.PullRequests.Create(
		ctx,
		core.GetGithubUsername(),
		core.GetGithubRepoName(),
		&pull,
	)
	if err != nil {
		return nil, err
	}
	if *pr.Number != lastPr+1 {
		return nil, ErrLostPCreationRaceCondition
	}
	return pr, nil
}

func (c *createPr) getLastPrNumber(ctx context.Context) (int, error) {
	pr, _, err := c.Github.PullRequests.List(
		ctx,
		core.GetGithubOwner(),
		core.GetGithubRepoName(),
		&github.PullRequestListOptions{
			State:     "all",
			Sort:      "created",
			Direction: "desc",
			ListOptions: github.ListOptions{
				Page:    0,
				PerPage: 1,
			},
		},
	)
	if err != nil {
		return 0, err
	}
	if len(pr) == 0 {
		return 0, nil
	}
	return *pr[0].Number, nil
}
