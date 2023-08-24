package git

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
		DetectDotGit:          true,
		EnableDotGitCommonDir: false,
	})
	if err != nil {
		panic(err)
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

func (r *Repo) Config() string {
	return path.Join(r.DotOpDir(), "config.yaml")
}

func (r *Repo) DotOpDir() string {
	return path.Join(r.Path(), ".opp")
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

func (r *Repo) TrackedBranches() (map[int]plumbing.Hash, error) {
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

func (r *Repo) Checkout(pr *LocalPr) error {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("git checkout %s", pr.LocalBranch()))
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) Fetch(ctx context.Context) error {
	err := r.FetchContext(ctx, &git.FetchOptions{
		RemoteName: GetRemoteName(),
	})

	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

func (r *Repo) Rebase(ctx context.Context, branch Branch) error {
	cmd := exec.Command(
		"bash", "-c",
		fmt.Sprintf("git rebase %s/%s", GetRemoteName(), branch.RemoteName()),
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) CanRebase() bool {
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

// Sometimes your ancestor has been merged.
func (r *Repo) GetFirstValidAncestor(pr *LocalPr) (Branch, error) {
	ancestor, err := pr.GetAncestor()
	if err != nil {
		return nil, err
	}
	if !ancestor.IsPr() {
		// Main branches don't disappear
		return ancestor, nil
	}
	_, err = r.GetLocalTip(ancestor)
	if err == plumbing.ErrReferenceNotFound {
		// This branch has been deleted.
		ancestor, err = r.GetFirstValidAncestor(ancestor.(*LocalPr))
		if err != nil {
			return nil, err
		}
		pr.SetAncestor(ancestor)
	}
	return ancestor, nil
}
