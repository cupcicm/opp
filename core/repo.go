package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
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

func branchpush(hash plumbing.Hash, branch string) []config.RefSpec {
	return []config.RefSpec{
		config.RefSpec(fmt.Sprintf("%s:refs/heads/%s", hash.String(), branch)),
	}
}

func (r *Repo) Push(ctx context.Context, hash plumbing.Hash, branch string) error {
	err := r.Repository.PushContext(ctx, &git.PushOptions{
		RemoteName: GetRemoteName(),
		RefSpecs:   branchpush(hash, branch),
		Force:      true,
	})
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
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
	cmd := r.GitExec("checkout %s", branch.LocalName())
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) GitExec(format string, args ...any) *exec.Cmd {
	cmd := exec.Command("bash", "-c", "git "+fmt.Sprintf(format, args...))
	cmd.Dir = r.Path()
	return cmd
}

func (r *Repo) Fetch(ctx context.Context) error {
	cmd := r.GitExec("fetch -p %s", GetRemoteName())
	return cmd.Run()
}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) SetTrackingBranch(localBranch Branch, remoteBranch Branch) error {
	cmd := r.GitExec(
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
func (r *Repo) NoLocalChanges() bool {
	for _, status := range Must(r.Worktree().Status()) {
		if !(status.Worktree == git.Unmodified || status.Worktree == git.Untracked) || !(status.Staging == git.Unmodified || status.Staging == git.Untracked) {
			return false
		}
	}

	return true
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
				r.SetTrackingBranch(&possibleDependentPR, r.BaseBranch())
			}
		}
	}
	for _, deleting := range toclean {
		r.DeleteLocalAndRemoteBranch(ctx, deleting)
		deleting.DeleteState()
	}
}

func (r *Repo) DeleteLocalAndRemoteBranch(ctx context.Context, branch Branch) {
	r.Repository.DeleteBranch(branch.LocalName())
	r.Storer.RemoveReference(plumbing.NewBranchReferenceName(branch.LocalName()))
	r.Repository.PushContext(ctx, &git.PushOptions{
		RemoteName: GetRemoteName(),
		RefSpecs:   []config.RefSpec{config.RefSpec(":" + branch.RemoteName())},
	})
}
