package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/story"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v56/github"
	"github.com/urfave/cli/v3"
)

var (
	ErrLostPrCreationRaceCondition              = errors.New("lost race condition when creating PR")
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
	DraftFlagUsage   = "Create a draft PR."
	ExtractFlagUsage = strings.TrimSpace(`
When set, tries to extract the commits used to create the PR from the current branch.
This means that the current branch will not retain the commits you used to create the PR, they
will be "moved" to the PR branch, and will not stay in your main branch.
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

func PrCommand(in io.Reader, repo *core.Repo, gh func(context.Context) core.Gh, sf func(string, string) story.StoryFetcher) *cli.Command {
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
				Name:    "interactive",
				Aliases: []string{"i"},
			},
			&cli.BoolFlag{
				Name:    "checkout",
				Aliases: []string{"c"},
				Usage:   CheckoutFlagUsage,
			},
			&cli.BoolFlag{
				Name:    "draft",
				Aliases: []string{"d"},
				Usage:   DraftFlagUsage,
			},
			&cli.BoolFlag{
				Name:    "extract",
				Aliases: []string{"x"},
				Usage:   ExtractFlagUsage,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			initialRef, err := repo.Head()
			if err != nil {
				return err
			}
			pr := create{Repo: repo, Github: gh(ctx), StoryFetcher: sf}
			args, err := pr.SanitizeArgs(ctx, cmd)
			if err != nil {
				return err
			}
			if args.NeedsRebase {
				newArgs, err := pr.RebasePrCommits(ctx, args)
				if err != nil {
					repo.CheckoutRef(initialRef)
					return err
				}
				args = newArgs
			}
			localPr, err := pr.Create(ctx, in, args)
			if err != nil {
				return err
			}
			if args.Extract && !args.Detached {
				// We need to remove the commits we used to make the PR from the branch
				// we were on before.
				err := repo.Checkout(args.InitialBranch)
				if err != nil {
					return err
				}
				if !repo.TryRebaseCurrentBranchSilently(ctx, localPr) {
					return fmt.Errorf("problem while rebasing %s on %s\n", args.InitialBranch.LocalName(), localPr.LocalName())
				}
				if !repo.TryLocalRebaseOntoSilently(ctx, args.Commits[0].Hash, args.Commits[len(args.Commits)-1].Hash) {
					return fmt.Errorf("problem while extracting PR commits from %s\n", args.InitialBranch.LocalName())
				}
			}

			if args.CheckoutPr {
				return repo.Checkout(localPr)
			}
			return repo.CheckoutRef(initialRef)
		},
	}

	return cmd
}

type create struct {
	Repo         *core.Repo
	Github       core.Gh
	StoryFetcher func(string, string) story.StoryFetcher
}

type args struct {
	AncestorBranch core.Branch
	Commits        []*object.Commit
	NeedsRebase    bool
	CheckoutPr     bool
	Interactive    bool
	DraftPr        bool
	Detached       bool
	InitialBranch  core.Branch
	Extract        bool
	BranchTag      string
}

func (c *create) SanitizeArgs(ctx context.Context, cmd *cli.Command) (*args, error) {
	forHead := cmd.Args().Present()
	var (
		overrideAncestorBranch core.Branch
		needsRebase            = false
		headCommit, err        = HeadCommit(c.Repo, cmd.Args())
	)
	if err != nil {
		return nil, err
	}
	overrideAncestor := cmd.String("base")
	localChanges := !c.Repo.NoLocalChanges(ctx)
	extract := cmd.Bool("extract")
	if overrideAncestor != "" {
		overrideAncestorBranch, err = c.Repo.GetBranch(overrideAncestor)
		needsRebase = true
		if err != nil {
			return nil, cli.Exit(fmt.Errorf("%s is not a valid branch", overrideAncestor), 1)
		}
	}
	if cmd.Bool("interactive") {
		needsRebase = true
	}
	// If there are local changes, we need to be very careful before we accept to create this PR
	if localChanges {
		if needsRebase {
			// If you have provided a different ancestor, the commits need to be
			// rebased on top of it, the user needs to stash their changes.
			return nil, cli.Exit("You have provided --base but have local changes, please stash them", 1)
		}
		if cmd.Bool("checkout") && !forHead {
			// If the user wants to checkout the PR at the end, it's OK but it needs to be a PR that will end
			// up exactly with the same HEAD as before.
			return nil, cli.Exit("Cannot checkout the PR since there are local changes. Please stash them", 1)
		}
		if extract {
			return nil, cli.Exit("You have provided --extract but have local changes, please stash them", 1)
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
	shouldCheckout := cmd.Bool("checkout")
	head := core.Must(c.Repo.Head())

	if !head.Name().IsBranch() {
		shouldCheckout = true
	}
	args := args{
		Commits:     commits,
		NeedsRebase: needsRebase,
		CheckoutPr:  shouldCheckout,
		Interactive: cmd.Bool("interactive"),
		DraftPr:     cmd.Bool("draft"),
		Extract:     extract,
	}
	if head.Name().IsBranch() {
		args.Detached = false
		args.InitialBranch = core.NewBranch(c.Repo, head.Name().Short())
	} else {
		args.Detached = true
	}
	if overrideAncestor != "" {
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
	// Careful ! The first commit is the child-most one, not the other way around.
	err := c.Repo.DetachHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not detach head: %w", err)
	}
	if previousArgs.Interactive {
		fmt.Println("Choose what commits you want to include in this PR")
		if !c.Repo.TryRebaseOntoSilently(
			ctx,
			previousArgs.Commits[len(previousArgs.Commits)-1].Hash,
			previousArgs.AncestorBranch,
			true,
		) {
			return nil, cli.Exit(fmt.Errorf(
				"one of the commits you chose cannot be replayed cleanly on %s/%s",
				core.GetRemoteName(), previousArgs.AncestorBranch.RemoteName(),
			), 1)
		}
	} else {
		fmt.Printf("Rebasing %d commits on top of %s/%s... ", len(previousArgs.Commits), core.GetRemoteName(), previousArgs.AncestorBranch.RemoteName())
		if !c.Repo.TryRebaseOntoSilently(
			ctx,
			previousArgs.Commits[len(previousArgs.Commits)-1].Hash,
			previousArgs.AncestorBranch,
			false,
		) {
			hashes := make([]string, len(previousArgs.Commits))
			for i := range previousArgs.Commits {
				hashes[i] = previousArgs.Commits[i].Hash.String()[0:6]
			}
			PrintFailure(nil)
			return nil, cli.Exit(fmt.Errorf(
				"one of these commits cannot be replayed cleanly on %s/%s:\n  - %s",
				core.GetRemoteName(), previousArgs.AncestorBranch.RemoteName(), strings.Join(hashes, "\n  - "),
			), 1)
		}
		PrintSuccess()
	}
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

	// make a deep copy of previous args
	newArgs := *previousArgs
	newArgs.AncestorBranch = ancestor
	newArgs.Commits = commits
	newArgs.NeedsRebase = false
	newArgs.CheckoutPr = true

	return &newArgs, nil
}

func (c *create) Create(ctx context.Context, in io.Reader, args *args) (*core.LocalPr, error) {

	// The first commit is the child-most one.
	lastCommit := args.Commits[0].Hash
	title, body, err := c.GetBodyAndTitle(ctx, in, args.Commits)
	if err != nil {
		return nil, fmt.Errorf("could not get the pull request body and title: %w", err)
	}

	pr, err := c.create(ctx, lastCommit, args.AncestorBranch, title, body, args.DraftPr)
	if err != nil {
		return nil, fmt.Errorf("could not create pull request : %w", err)
	}
	c.createLocalBranchForPr(pr, lastCommit, args.AncestorBranch)
	localPr := core.NewLocalPr(c.Repo, pr)
	localPr.SetAncestor(args.AncestorBranch)
	localPr.RememberCurrentTip()
	err = c.Repo.SetTrackingBranch(localPr, args.AncestorBranch)
	if err != nil {
		err = fmt.Errorf("pr has been created but could not set tracking branch")
	}
	fmt.Println(localPr.Url())
	return localPr, err
}

func (c *create) GetBodyAndTitle(ctx context.Context, in io.Reader, commits []*object.Commit) (string, string, error) {
	rawTitle, rawBody := c.getRawBodyAndTitle(commits)
	commitMessages := make([]string, len(commits))
	for i, c := range commits {
		commitMessages[i] = c.Message
	}
	storyService := story.NewStoryService(c.StoryFetcher, in)
	title, body, err := storyService.EnrichBodyAndTitle(ctx, commitMessages, rawTitle, rawBody)
	if err != nil {
		return "", "", fmt.Errorf("could not enrich the PR with the Story: %w", err)
	}
	return title, body, nil
}

func (c *create) getRawBodyAndTitle(commits []*object.Commit) (string, string) {
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

func (c *create) create(
	ctx context.Context,
	hash plumbing.Hash,
	ancestor core.Branch,
	title string,
	body string,
	draft bool,
) (int, error) {
	for attempts := 0; attempts < 3; attempts++ {
		pr, err := c.createOnce(ctx, hash, ancestor, title, body, draft)
		if err == nil {
			return pr, nil
		}
		if err == ErrLostPrCreationRaceCondition {
			c.undoCreatePr(ctx, pr)
		}
		if err != ErrLostPrCreationRaceCondition {
			return 0, err
		}
	}
	return 0, ErrLostPrCreationRaceConditionMultipleTimes
}

func (c *create) createOnce(
	ctx context.Context,
	hash plumbing.Hash,
	ancestor core.Branch,
	title string,
	body string,
	draft bool,
) (int, error) {
	ctx, cancel := context.WithTimeoutCause(
		ctx, core.GetGithubTimeout(),
		fmt.Errorf("creating PR too slow, increase github.timeout"),
	)
	defer cancel()
	lastPr, err := c.getLastPrNumber(ctx)
	if err != nil {
		return 0, err
	}
	remote := core.RemoteBranchForPr(lastPr + 1)
	base := ancestor.RemoteName()
	err = c.Repo.Push(ctx, hash, remote)
	if err != nil {
		return 0, err
	}
	pull := github.NewPullRequest{
		Title: &title,
		Head:  &remote,
		Base:  &base,
		Body:  &body,
		Draft: &draft,
	}
	pr, _, err := c.Github.PullRequests().Create(
		ctx,
		core.GetGithubOwner(),
		core.GetGithubRepoName(),
		&pull,
	)
	if err != nil {
		return 0, err
	}
	if *pr.Number != lastPr+1 {
		return lastPr + 1, ErrLostPrCreationRaceCondition
	}
	return *pr.Number, nil
}

func (c *create) undoCreatePr(ctx context.Context, prNumber int) {
	pr := core.NewLocalPr(c.Repo, prNumber)
	fmt.Println("oops, deleting pr ", pr.LocalBranch())
	// The local branch has not been created yet, no need to delete it.
	c.Repo.DeleteRemoteBranch(ctx, pr)
}

// The Github API list pull requests and issues under "issues".
func (c *create) getLastPrNumber(ctx context.Context) (int, error) {
	issues, _, err := c.Github.Issues().ListByRepo(
		ctx,
		core.GetGithubOwner(),
		core.GetGithubRepoName(),
		&github.IssueListByRepoOptions{
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

	if len(issues) == 0 {
		return 0, nil
	}
	return *issues[0].Number, nil
}
