package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

type Repo struct {
	*git.Repository
}

func Current() *Repo {
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{
		// Walk back the directories to find the .git folder.
		DetectDotGit:          true,
		EnableDotGitCommonDir: false,
	})
	if err != nil {
		panic("You are not inside a git repository")
	}
	return &Repo{
		Repository: repo,
	}
}

func (r *Repo) OppEnabled() bool {
	return FileExists(r.Config())
}

func (r *Repo) Worktree() *git.Worktree {
	w, err := r.Repository.Worktree()
	if err == git.ErrIsBareRepository {
		panic("cannot work on bare repos")
	}
	return w
}

func (r *Repo) Path() string {
	return r.Worktree().Filesystem.Root()
}

func (r *Repo) DotOpDir() string {
	return path.Join(r.Path(), ".opp")
}

func (r *Repo) Config() string {
	return path.Join(r.DotOpDir(), "config.yaml")
}

func (r *Repo) AllPrs(ctx context.Context) []LocalPr {
	return PrStates{r}.AllPrs(ctx)
}

func (r *Repo) Push(ctx context.Context, hash plumbing.Hash, branch string) error {
	ctx, cancel := context.WithTimeoutCause(
		ctx, GetGithubTimeout(),
		fmt.Errorf("push to %s too slow, increase github.timeout", GetRemoteName()),
	)
	defer cancel()
	cmd := r.GitExec(ctx, "push --force %s %s:refs/heads/%s", GetRemoteName(), hash.String(), branch)
	return cmd.Run()
}

func (r *Repo) AllLocalPrs() (map[int]plumbing.Hash, error) {
	iter, err := r.Repository.Branches()
	if err != nil {
		return nil, fmt.Errorf("could not iter branches: %w", err)
	}
	result := make(map[int]plumbing.Hash)
	iter.ForEach(func(ref *plumbing.Reference) error {
		pr, err := ExtractPrNumber(ref.Name().Short())
		if err == nil {
			result[pr] = ref.Hash()
		}
		return nil
	})
	return result, nil
}

// Returns all of the commits between the given hash
// and its merge-base with the base branch of the repository
func (r *Repo) GetCommitsNotInBaseBranch(hash plumbing.Hash) ([]*object.Commit, error) {
	commit, err := r.Repository.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	ref, err := r.Reference(
		plumbing.NewRemoteReferenceName(GetRemoteName(), GetBaseBranch()),
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("could not find the tip of the base branch: %w", err)
	}
	base, err := r.Repository.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("could not find the commit for the tip of the base branch: %w", err)
	}
	mergeBase, err := commit.MergeBase(base)
	if err != nil {
		return nil, fmt.Errorf("no common ancestor between %s and %s", commit.Hash.String(), ref.Hash().String())
	}
	if len(mergeBase) != 1 {
		return nil, fmt.Errorf("do not know how to deal with more than one merge base")
	}
	from := commit
	to := mergeBase[0]

	iterCommits := object.NewCommitPreorderIter(from, nil, nil)
	commits := make([]*object.Commit, 0)

	iterCommits.ForEach(func(c *object.Commit) error {
		if c.Hash == to.Hash {
			return storer.ErrStop
		}
		commits = append(commits, c)
		return nil
	})
	return commits, nil
}

// Takes all commit that are ancestors of headCommit and not in the base branch
// and walks them until it finds one that is the tip of an exisiting pr/XXX branch.
// Returns all the commits that were touched during the walk, in git children -> parent order.
// (e.g. the first commit is always headCommit)
func (r *Repo) FindBranchingPoint(headCommit plumbing.Hash) (Branch, []*object.Commit, error) {
	commits, err := r.GetCommitsNotInBaseBranch(headCommit)
	branchedCommits := make([]*object.Commit, 0)
	if err != nil {
		return nil, nil, err
	}
	tracked, err := r.AllLocalPrs()
	if err != nil {
		return nil, nil, err
	}

	for _, commit := range commits {
		for number, tip := range tracked {
			if commit.Hash == tip {
				return NewLocalPr(r, number), branchedCommits, nil
			}
		}
		branchedCommits = append(branchedCommits, commit)
	}
	return r.BaseBranch(), commits, nil
}

func (r *Repo) PrForHead() (*LocalPr, bool) {
	head := Must(r.Repository.Head())
	branchName := head.Name().Short()
	number, errNotAPr := ExtractPrNumber(branchName)
	if errNotAPr != nil {
		return nil, false
	}
	return NewLocalPr(r, number), true
}

func (r *Repo) BaseBranch() Branch {
	return NewBranch(r, GetBaseBranch())
}

func (r *Repo) Checkout(branch Branch) error {
	cmd := r.GitExec(context.Background(), "checkout %s", branch.LocalName())
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) GitExec(ctx context.Context, format string, args ...any) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "bash", "-c", "git "+fmt.Sprintf(format, args...))
	cmd.Dir = r.Path()
	return cmd
}

func (r *Repo) Fetch(ctx context.Context) error {
	ctx, cancel := context.WithTimeoutCause(
		ctx, GetGithubTimeout(),
		fmt.Errorf("fetch from %s too slow, increase github.timeout", GetRemoteName()),
	)
	defer cancel()
	// The --prune here is important : it removes the branches that have been deleted on github.
	cmd := r.GitExec(ctx, "fetch --prune %s", GetRemoteName())
	return cmd.Run()
}

// When remote is true, rebase on the distant version of the branch. When false,
// rebase on the local version.
func (r *Repo) Rebase(ctx context.Context, branch Branch) error {
	cmd := r.GitExec(ctx, "rebase %s/%s", GetRemoteName(), branch.RemoteName())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) TryRebaseCurrentBranchSilently(ctx context.Context, branch Branch) bool {
	cmd := r.GitExec(ctx, "rebase %s/%s", GetRemoteName(), branch.RemoteName())
	err := cmd.Run()
	if err == nil {
		return true
	}
	abort := r.GitExec(ctx, "rebase --abort")
	if err := abort.Run(); err != nil {
		panic(fmt.Errorf("tried to abort the rebase but failed: %w", err))
	}
	return false
}

func (r *Repo) TryRebaseOntoSilently(ctx context.Context, first plumbing.Hash, last plumbing.Hash, onto Branch, interactive bool) bool {
	interactiveString := ""
	if interactive {
		interactiveString = "--interactive"
	}
	cmd := r.GitExec(ctx, "rebase %s --onto %s/%s %s^ %s", interactiveString, GetRemoteName(), onto.RemoteName(), first.String(), last.String())
	err := cmd.Run()
	if err == nil {
		return true
	}
	abort := r.GitExec(ctx, "rebase --abort")
	if err := abort.Run(); err != nil {
		panic(fmt.Errorf("tried to abort the rebase but failed: %w", err))
	}
	return false
}

func (r *Repo) TryRebaseBranchOnto(ctx context.Context, parent plumbing.Hash, onto Branch) bool {
	ontoName := onto.LocalName()
	if !onto.IsPr() {
		ontoName = fmt.Sprintf("%s/%s", GetRemoteName(), onto.RemoteName())
	}
	cmd := r.GitExec(ctx, "rebase --onto %s %s", ontoName, parent.String())
	err := cmd.Run()
	if err == nil {
		return true
	}
	abort := r.GitExec(ctx, "rebase --abort")
	if err := abort.Run(); err != nil {
		panic(fmt.Errorf("tried to abort the rebase but failed: %w", err))
	}
	return false
}

// When remote is true, rebase on the distant version of the branch. When false,
// rebase on the local version.
func (r *Repo) InteractiveRebase(ctx context.Context, branch Branch) error {
	cmd := r.GitExec(ctx, "rebase --no-fork-point -i %s/%s", GetRemoteName(), branch.RemoteName())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) SetTrackingBranch(localBranch Branch, remoteBranch Branch) error {
	cmd := r.GitExec(
		context.Background(),
		"branch -u %s/%s %s",
		GetRemoteName(),
		remoteBranch.RemoteName(),
		localBranch.LocalName())
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.Stdin = nil
	return cmd.Run()
}

// NoLocalChanges returns true when all files are either
// unmodified or untracked.
func (r *Repo) NoLocalChanges(ctx context.Context) bool {
	ctx, cancel := context.WithTimeoutCause(
		ctx, GetGithubTimeout(),
		fmt.Errorf("git status too slow, increase github.timeout"),
	)
	defer cancel()
	cmd := r.GitExec(ctx, "status --untracked-files=no --short")
	cmd.Stderr = nil
	cmd.Stdin = nil
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// if git status has no output, then there are no local changes.
	return len(out) == 0
}

func (r *Repo) CurrentBranch() (Branch, error) {
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	if !head.Name().IsBranch() {
		return nil, errors.New("works only when on a branch")
	}
	pr, err := ExtractPrNumber(head.Name().Short())
	if err == nil {
		return NewLocalPr(r, pr), nil
	}
	return NewBranch(r, head.Name().Short()), nil
}

func (r *Repo) GetBranch(name string) (Branch, error) {
	pr, err := ExtractPrNumber(name)
	if err == nil {
		return NewLocalPr(r, pr), nil
	}
	_, err = r.Repository.Reference(plumbing.NewBranchReferenceName(name), true)
	if err != nil {
		return nil, err
	}
	return NewBranch(r, name), nil
}

func (r *Repo) GetLocalTip(b Branch) (*object.Commit, error) {
	ref, err := r.Reference(plumbing.NewBranchReferenceName(b.LocalName()), true)
	if err != nil {
		return nil, err
	}
	return r.CommitObject(ref.Hash())
}

func (r *Repo) GetRemoteTip(b Branch) (*object.Commit, error) {
	ref, err := r.Reference(plumbing.NewRemoteReferenceName(GetRemoteName(), b.RemoteName()), true)
	if err != nil {
		return nil, err
	}
	return r.CommitObject(ref.Hash())
}

func (r *Repo) CleanupAfterMerge(ctx context.Context, pr *LocalPr) {
	tip, err := r.GetLocalTip(pr)
	if err != nil {
		fmt.Printf("could not find the tip of branch %s.\n", pr.LocalBranch())
		return
	}
	fmt.Printf("Removing local branch %s. Tip was %s\n", pr.LocalBranch(), tip.Hash.String()[0:7])
	r.CleanupMultiple(ctx, []*LocalPr{pr}, r.AllPrs(ctx))
}

func (r *Repo) CleanupMultiple(ctx context.Context, toclean []*LocalPr, others []LocalPr) {
	for _, possibleDependentPR := range others {
		ancestor, _ := possibleDependentPR.GetAncestor()
		for _, deleting := range toclean {
			if ancestor.LocalName() == deleting.LocalName() {
				// This is a PR that depends on the PR we are currently cleaning.
				// Make it point to the master branch
				possibleDependentPR.SetAncestor(r.BaseBranch())
				possibleDependentPR.SetKnownTipsFromAncestor(deleting)
				r.SetTrackingBranch(&possibleDependentPR, r.BaseBranch())
			}
		}
	}
	for _, deleting := range toclean {
		r.DeleteLocalAndRemoteBranch(ctx, deleting)
		deleting.DeleteState()
	}
}

func (r *Repo) DeleteLocalAndRemoteBranch(ctx context.Context, branch Branch) error {
	r.Repository.DeleteBranch(branch.LocalName())
	r.Storer.RemoveReference(plumbing.NewBranchReferenceName(branch.LocalName()))
	return r.DeleteRemoteBranch(ctx, branch)
}

func (r *Repo) DeleteRemoteBranch(ctx context.Context, branch Branch) error {
	ctx, cancel := context.WithTimeoutCause(
		ctx, GetGithubTimeout(),
		fmt.Errorf("push to %s too slow, increase github.timeout", GetRemoteName()),
	)
	defer cancel()
	cmd := r.GitExec(ctx, "push %s :%s", GetRemoteName(), branch.RemoteName())
	return cmd.Run()
}
