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

var (
	ErrLostPCreationRaceCondition               = errors.New("lost race condition when creating PR")
	ErrLostPrCreationRaceConditionMultipleTimes = errors.New("lost race condition when creating PR too many times, aborting")
	ErrAlreadyAPrBranch                         = errors.New(strings.TrimSpace(`
You are on a branch that has already been pushed as a PR
Use opp up to update that PR instead.`))
	BaseFlagUsage = strings.TrimSpace(`
Specify what is the base for your PR (either the base branch or another PR).
If you leave this blank, opp is going to detect what is the right base for your PR
by walking back the history until it finds either the main branch or the tip of another PR.
`)
	CheckoutFlagUsage = strings.TrimSpace(`
By default, opp pr tries to leave you on the branch you were. Use --checkout if you want to
checkout the PR branch.
There are some cases where opp pr still checkouts the PR branch after creation: when you
specified a different base for example.
`)
	Description = strings.TrimSpace(`
Starting from either HEAD, or the provided reference (HEAD~1, a74c9e, a_branch, ...) and walking
back, gathers commits until it finds either the tip of a PR branch (e.g. pr/xxx) or the base branch
and creates a PR that contains these commits.

If you provide a different --base, these commits will be rebased on that new base, and if this is
a clean rebase it will create a PR targetting that base.

If you don't, it will create a PR targetting either the base branch or another PR, depending on
whether it reached the base branch or the tip of another PR first when walking back.
`)
)

func PrCommand(repo *core.Repo, gh func(context.Context) core.GhPullRequest) *cli.Command {
	cmd := &cli.Command{
		Name:        "pr",
		Aliases:     []string{"pull-request", "new"},
		ArgsUsage:   "[reference]",
		Usage:       "Creates a new PR branch locally, pushes it, and creates a github PR",
		Description: Description,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "base",
				Aliases: []string{"b"},
				Usage:   BaseFlagUsage,
			},
			&cli.BoolFlag{
				Name:    "checkout",
				Aliases: []string{"c"},
				Usage:   CheckoutFlagUsage,
			},
		},
		Action: func(cCtx *cli.Context) error {
			pr := create{Repo: repo, PullRequests: gh(cCtx.Context)}
			args, err := pr.SanitizeArgs(cCtx)
			if err != nil {
				return err
			}
			if args.NeedsRebase {
				newArgs, err := pr.RebasePrCommits(cCtx.Context, args)
				if err != nil {
					return err
				}
				args = newArgs
			}

			err = nil
			localPr, createErr := pr.Create(cCtx.Context, args)
			if args.CheckoutPr {
				err = repo.Checkout(localPr)
			}
			if createErr != nil {
				return createErr
			}
			return err
		},
	}

	return cmd
}

type create struct {
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

type args struct {
	AncestorBranch core.Branch
	Commits        []*object.Commit
	NeedsRebase    bool
	CheckoutPr     bool
}

func (c *create) SanitizeArgs(cCtx *cli.Context) (*args, error) {
	forHead := cCtx.Args().Present()
	var overrideAncestorBranch core.Branch = nil
	needsRebase := false
	headCommit, err := HeadCommit(c.Repo, cCtx.Args())
	if err != nil {
		return nil, err
	}
	overrideAncestor := cCtx.String("base")
	localChanges := !c.Repo.NoLocalChanges()
	if overrideAncestor != "" {
		overrideAncestorBranch, err = c.Repo.GetBranch(overrideAncestor)
		needsRebase = true
		if err != nil {
			return nil, cli.Exit(fmt.Errorf("%s is not a valid branch", overrideAncestor), 1)
		}
	}
	// If there are local changes, we need to be very careful before we accept to create this PR
	if localChanges {
		if needsRebase {
			// If you have provided a different ancestor, the commits need to be
			// rebased on top of it, the user needs to stash their changes.
			return nil, cli.Exit("You have provided --base but have local changes, please stash them", 1)
		}
		if cCtx.Bool("checkout") && !forHead {
			// If the user wants to checkout the PR at the end, it's OK but it needs to be a PR that will end
			// up exactly with the same HEAD as before.
			return nil, cli.Exit("Cannot checkout the PR since there are local changes. Please stash them", 1)
		}
	}
	ancestor, commits, err := c.Repo.FindBranchingPoint(headCommit)
	if err != nil {
		return nil, cli.Exit(fmt.Sprintf(
			"%s does not descend from %s/%s",
			headCommit, core.GetRemoteName(), core.GetBaseBranch(),
		), 1)
	}
	tip := core.Must(c.Repo.GetLocalTip(ancestor))
	if ancestor.IsPr() && tip.Hash == headCommit {
		return nil, cli.Exit(ErrAlreadyAPrBranch, 1)
	}
	args := args{
		Commits:     commits,
		NeedsRebase: needsRebase,
		CheckoutPr:  cCtx.Bool("checkout"),
	}
	if needsRebase {
		args.AncestorBranch = overrideAncestorBranch
		args.CheckoutPr = true
	} else {
		args.AncestorBranch = ancestor
	}
	return &args, nil
}

func HeadCommit(repo *core.Repo, args cli.Args) (plumbing.Hash, error) {
	var headCommit plumbing.Hash
	if !args.Present() {
		var head = core.Must(repo.Head())
		headCommit = head.Hash()
	} else {
		hash, err := repo.ResolveRevision(plumbing.Revision(args.First()))
		if err != nil {
			return plumbing.ZeroHash, cli.Exit(fmt.Sprintf("invalid revision %s", args.First()), 1)
		}
		headCommit = *hash
	}
	return headCommit, nil
}

// If the user wants to rebase the commits in this PR on another branch, we try to run the rebase
// (rebase --onto new_branch first_commit^ last commit).
// If that works, then create the PR with those new commits and then checkout the PR
// since HEAD will be detached after the rebase.
func (c *create) RebasePrCommits(ctx context.Context, previousArgs *args) (*args, error) {
	fmt.Printf("Rebasing %d commits on top of %s/%s... ", len(previousArgs.Commits), core.GetRemoteName(), previousArgs.AncestorBranch.RemoteName())
	// Careful ! The first commit is the child-most one, not the other way around.
	if !c.Repo.TryRebaseOntoSilently(ctx, previousArgs.Commits[len(previousArgs.Commits)-1].Hash, previousArgs.Commits[0].Hash, previousArgs.AncestorBranch) {
		hashes := make([]string, len(previousArgs.Commits))
		for i := range previousArgs.Commits {
			hashes[i] = previousArgs.Commits[i].Hash.String()[0:6]
		}
		fmt.Println("❌")
		return nil, cli.Exit(fmt.Errorf(
			"one of these commits cannot be replayed cleanly on %s/%s:\n  - %s",
			core.GetRemoteName(), previousArgs.AncestorBranch.RemoteName(), strings.Join(hashes, "\n  - "),
		), 1)
	}
	fmt.Println("✅")
	head, err := c.Repo.Head()
	if err != nil {
		return nil, err
	}
	headCommit := head.Hash()
	ancestor, commits, err := c.Repo.FindBranchingPoint(headCommit)

	if err != nil {
		return nil, cli.Exit(fmt.Sprintf(
			"%s does not descend from %s/%s",
			headCommit, core.GetRemoteName(), core.GetBaseBranch(),
		), 1)
	}
	return &args{
		AncestorBranch: ancestor,
		Commits:        commits,
		NeedsRebase:    true,
		CheckoutPr:     true,
	}, nil
}

func (c *create) Create(ctx context.Context, args *args) (*core.LocalPr, error) {

	// The first commit is the child-most one.
	lastCommit := args.Commits[0].Hash
	title, body := c.GetBodyAndTitle(args.Commits)

	pr, err := c.create(ctx, lastCommit, args.AncestorBranch, title, body)
	if err != nil {
		return nil, fmt.Errorf("could not create pull request : %w", err)
	}
	c.createLocalBranchForPr(*pr.Number, lastCommit, args.AncestorBranch)
	localPr := core.NewLocalPr(c.Repo, *pr.Number)
	localPr.SetAncestor(args.AncestorBranch)
	err = c.Repo.SetTrackingBranch(localPr, args.AncestorBranch)
	if err != nil {
		err = fmt.Errorf("pr has been created but could not set tracking branch")
	}
	fmt.Println(localPr.Url())
	return localPr, err
}

func (c *create) GetBodyAndTitle(commits []*object.Commit) (string, string) {
	sort.Slice(commits, func(i, j int) bool {
		return len(commits[i].Message) > len(commits[j].Message)
	})
	title, body, _ := strings.Cut(strings.TrimSpace(commits[0].Message), "\n")
	return strings.TrimSpace(title), strings.TrimSpace(body)
}

func (c *create) createLocalBranchForPr(number int, hash plumbing.Hash, ancestor core.Branch) {
	c.Repo.CreateBranch(&config.Branch{
		Name:   core.LocalBranchForPr(number),
		Remote: core.RemoteBranchForPr(number),
		Merge:  plumbing.NewBranchReferenceName(ancestor.RemoteName()),
		Rebase: "true",
	})

	ref := plumbing.NewBranchReferenceName(core.LocalBranchForPr(number))
	c.Repo.Storer.SetReference(plumbing.NewHashReference(ref, hash))
}

func (c *create) create(ctx context.Context, hash plumbing.Hash, ancestor core.Branch, title string, body string) (*github.PullRequest, error) {
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

func (c *create) createOnce(ctx context.Context, hash plumbing.Hash, ancestor core.Branch, title string, body string) (*github.PullRequest, error) {
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

func (c *create) getLastPrNumber(ctx context.Context) (int, error) {
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
