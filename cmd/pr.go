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
	"github.com/urfave/cli/v2"
)

var ErrLostPCreationRaceCondition error = errors.New("lost race condition when creating PR")
var ErrLostPrCreationRaceConditionMultipleTimes error = errors.New("lost race condition when creating PR too many times, aborting")
var ErrAlreadyAPrBranch = errors.New(
	`You are on a branch that has already been pushed as a PR
Use opp up to update that PR instead.`,
)

func PrCommand(repo *core.Repo, gh func(context.Context) core.GhPullRequest) *cli.Command {
	cmd := &cli.Command{
		Name:      "pr",
		Aliases:   []string{"pull-request", "new"},
		ArgsUsage: "[pr number]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "ancestor",
				Aliases: []string{"a"},
				Usage:   "Specify the PR you want the new one to depend on, rather than it being detected automatically",
			},
			&cli.BoolFlag{
				Name:    "checkout",
				Aliases: []string{"c"},
				Usage:   "By default, opp pr does not checkout the pr branch after creation. Set this flag so that it does.",
			},
		},
		Action: func(cCtx *cli.Context) error {
			forHead := false
			var headCommit plumbing.Hash
			var overrideAncestorBranch core.Branch
			previousHead, err := repo.GetBranch(core.Must(repo.Head()).Name().Short())
			if err != nil {
				previousHead = nil
			}
			if !cCtx.Args().Present() {
				var head = core.Must(repo.Head())
				headCommit = head.Hash()
				forHead = true
			} else {
				hash, err := repo.ResolveRevision(plumbing.Revision(cCtx.Args().First()))
				if err != nil {
					return cli.Exit(fmt.Sprintf("invalid revision %s", cCtx.Args().First()), 1)
				}
				headCommit = *hash
			}
			ancestor, commits, err := repo.FindBranchingPoint(headCommit)
			if err != nil {
				return cli.Exit(fmt.Sprintf(
					"%s does not descend from %s/%s",
					headCommit, core.GetRemoteName(), core.GetBaseBranch(),
				), 1)
			}
			overrideAncestor := cCtx.String("ancestor")
			localChanges := !repo.NoLocalChanges()
			if localChanges && overrideAncestor != "" {
				// If you have provided a different ancestor, the commits need to be
				// rebased on top of it, stash the changes.
				return cli.Exit("You have provided --ancestor but have local changes, please stash them", 1)
			}
			if cCtx.Bool("checkout") && (localChanges && !forHead) {
				return cli.Exit("Cannot checkout the PR since there are local changes. Please stash them", 1)
			}
			tip := core.Must(repo.GetLocalTip(ancestor))
			if ancestor.IsPr() && tip.Hash == headCommit {
				return cli.Exit(ErrAlreadyAPrBranch, 1)
			}

			if overrideAncestor != "" {
				fmt.Printf("Rebasing %d commits on top of %s... ", len(commits), overrideAncestor)
				overrideAncestorBranch, err = repo.GetBranch(overrideAncestor)
				if err != nil {
					fmt.Println("❌")
					return cli.Exit(fmt.Errorf("%s is not a valid branch", overrideAncestor), 1)
				}
				if !repo.TryRebaseOntoSilently(cCtx.Context, commits[0].Hash, headCommit, overrideAncestorBranch) {
					hashes := make([]string, len(commits))
					for i := range commits {
						hashes[i] = commits[i].Hash.String()
					}
					fmt.Println("❌")
					return cli.Exit(fmt.Errorf(
						"one of these commits %s cannot be replayed cleanly on %s",
						strings.Join(hashes, ", "), overrideAncestor,
					), 1)
				}
				fmt.Printf("✅\n")
				head, err := repo.Head()
				if err != nil {
					repo.Checkout(previousHead)
					return err
				}
				headCommit = head.Hash()
				ancestor, commits, err = repo.FindBranchingPoint(headCommit)
				if err != nil {
					return cli.Exit(fmt.Sprintf(
						"%s does not descend from %s/%s",
						headCommit, core.GetRemoteName(), core.GetBaseBranch(),
					), 1)
				}
			}

			pr := createPr{Repo: repo, PullRequests: gh(cCtx.Context)}
			localPr, err := pr.Create(cCtx.Context, headCommit, commits, ancestor)
			if err != nil {
				return err
			}
			if cCtx.Bool("checkout") {
				return repo.Checkout(localPr)
			}
			if overrideAncestor != "" && previousHead != nil {
				return repo.Checkout(previousHead)
			}
			return nil
		},
	}

	return cmd
}

type createPr struct {
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

func RemoteBranch(branch string) string {
	return fmt.Sprintf("%s/%s", core.GetGithubUsername(), branch)
}

func (c *createPr) Create(ctx context.Context, hash plumbing.Hash, commits []*object.Commit, ancestor core.Branch) (*core.LocalPr, error) {

	title, body := c.GetBodyAndTitle(commits)

	pr, err := c.create(ctx, hash, ancestor, title, body)
	if err != nil {
		return nil, fmt.Errorf("could not create pull request : %w", err)
	}
	c.createLocalBranchForPr(*pr.Number, hash, ancestor)
	localPr := core.NewLocalPr(c.Repo, *pr.Number)
	localPr.SetAncestor(ancestor)
	err = c.Repo.SetTrackingBranch(localPr, ancestor)
	if err != nil {
		err = fmt.Errorf("pr has been created but could not set tracking branch")
	}
	fmt.Println(localPr.Url())
	return localPr, err
}

func (c *createPr) GetBodyAndTitle(commits []*object.Commit) (string, string) {
	sort.Slice(commits, func(i, j int) bool {
		return len(commits[i].Message) > len(commits[j].Message)
	})
	title, body, _ := strings.Cut(strings.TrimSpace(commits[0].Message), "\n")
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
	pr, _, err := c.PullRequests.Create(
		ctx,
		core.GetGithubOwner(),
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
	pr, _, err := c.PullRequests.List(
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
