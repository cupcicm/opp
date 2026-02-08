package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

var ErrReferenceNotFound = errors.New("reference not found")

type Repo struct {
	*git.Repository
	s    *StateStore
	path string
}

func Current() *Repo {
	cwd, err := os.Getwd()
	if err != nil {
		panic("could not get current directory")
	}
	return NewRepo(cwd)
}

// NewRepo creates a Repo rooted at the given folder.
// It walks up the directory tree to find the .git folder.
func NewRepo(folder string) *Repo {
	dir := folder
	for {
		if _, err := os.Stat(path.Join(dir, ".git")); err == nil {
			repo, err := git.PlainOpen(dir)
			if err != nil {
				panic(fmt.Sprintf("could not open git repository at %s: %v", dir, err))
			}
			r := &Repo{
				Repository: repo,
				path:       dir,
			}
			r.s = NewStateStore(r)
			return r
		}
		parent := path.Dir(dir)
		if parent == dir {
			panic("not inside a git repository")
		}
		dir = parent
	}
}

// NewRepoFromGitRepo creates a Repo from an existing go-git Repository.
// Used in tests.
func NewRepoFromGitRepo(repo *git.Repository) *Repo {
	w, err := repo.Worktree()
	if err != nil {
		panic("could not get worktree")
	}
	r := &Repo{
		Repository: repo,
		path:       w.Filesystem.Root(),
	}
	r.s = NewStateStore(r)
	return r
}

func (r *Repo) OppEnabled() bool {
	return FileExists(r.Config())
}

func (r *Repo) StateStore() *StateStore {
	return r.s
}

func (r *Repo) Path() string {
	return r.path
}

func (r *Repo) DotOpDir() string {
	return path.Join(r.Path(), ".opp")
}

func (r *Repo) Config() string {
	return path.Join(r.DotOpDir(), "config.yaml")
}

func (r *Repo) AllPrs(ctx context.Context) []LocalPr {
	var prNumbers = r.StateStore().AllLocalPrNumbers(ctx)
	var toclean []*LocalPr
	var prs = make([]LocalPr, 0, len(prNumbers))

	for _, prNum := range prNumbers {
		pr := NewLocalPr(r, prNum)
		// Check that the branch exists.
		_, err := r.GetLocalTip(pr)
		if err != nil {
			// This PR does not exist locally: clean it.
			toclean = append(toclean, pr)
		} else {
			prs = append(prs, *pr)
		}
	}
	r.CleanupMultiple(ctx, toclean, prs)
	return prs
}

func (r *Repo) Push(ctx context.Context, hash string, branch string) error {
	ctx, cancel := context.WithTimeoutCause(
		ctx, GetGithubTimeout(),
		fmt.Errorf("push to %s too slow, increase github.timeout", GetRemoteName()),
	)
	defer cancel()
	cmd := r.GitExec(ctx, "%s --force %s %s:refs/heads/%s", GetPushCommand(), GetRemoteName(), hash, branch)
	return cmd.Run()
}

func (r *Repo) AllLocalPrs() (map[int]string, error) {
	cmd := r.GitExec(context.Background(), "for-each-ref '--format=%%(refname:short) %%(objectname)' refs/heads/pr/")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list branches: %w", err)
	}
	result := make(map[int]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		pr, err := ExtractPrNumber(parts[0])
		if err == nil {
			result[pr] = parts[1]
		}
	}
	return result, nil
}

// Returns all of the commits between the given hash
// and its merge-base with the base branch of the repository
func (r *Repo) GetCommitsNotInBaseBranch(hash string) ([]*object.Commit, error) {
	commit, err := r.Repository.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return nil, err
	}
	baseHash, err := r.GetRefHash(
		context.Background(),
		fmt.Sprintf("refs/remotes/%s/%s", GetRemoteName(), GetBaseBranch()),
	)
	if err != nil {
		return nil, fmt.Errorf("could not find the tip of the base branch: %w", err)
	}
	base, err := r.Repository.CommitObject(plumbing.NewHash(baseHash))
	if err != nil {
		return nil, fmt.Errorf("could not find the commit for the tip of the base branch: %w", err)
	}
	mergeBase, err := commit.MergeBase(base)
	if err != nil {
		return nil, fmt.Errorf("no common ancestor between %s and %s", commit.Hash.String(), baseHash)
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
func (r *Repo) FindBranchingPoint(headCommit string) (Branch, []*object.Commit, error) {
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
			if commit.Hash.String() == tip {
				return NewLocalPr(r, number), branchedCommits, nil
			}
		}
		branchedCommits = append(branchedCommits, commit)
	}
	return r.BaseBranch(), commits, nil
}

func (r *Repo) PrForHead() (*LocalPr, bool) {
	branchName, err := r.GetCurrentBranchName(context.Background())
	if err != nil {
		// HEAD is detached, not on a branch
		return nil, false
	}
	number, errNotAPr := ExtractPrNumber(branchName)
	if errNotAPr != nil {
		return nil, false
	}
	return NewLocalPr(r, number), true
}

func (r *Repo) BaseBranch() Branch {
	return NewBranch(r, GetBaseBranch())
}

func (r *Repo) Checkout(ctx context.Context, branch Branch) error {
	return r.CheckoutRef(ctx, branch.LocalName())
}

// CheckoutRef checks out the given ref (branch name or commit hash).
func (r *Repo) CheckoutRef(ctx context.Context, ref string) error {
	cmd := r.GitExec(ctx, "checkout %s", ref)
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// GetHeadRef returns a string representing the current HEAD.
// If on a branch, returns the branch name. If detached, returns the commit hash.
func (r *Repo) GetHeadRef(ctx context.Context) (string, error) {
	branchName, err := r.GetCurrentBranchName(ctx)
	if err == nil {
		return branchName, nil
	}
	hash, err := r.GetHeadHash(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD ref: %w", err)
	}
	return hash, nil
}

func (r *Repo) GitExec(ctx context.Context, format string, args ...any) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "bash", "-c", "git "+fmt.Sprintf(format, args...))
	cmd.Dir = r.Path()
	return cmd
}

// GetHeadHash returns the SHA of the current HEAD commit.
// This replaces the pattern: Repository.Head().Hash()
func (r *Repo) GetHeadHash(ctx context.Context) (string, error) {
	cmd := r.GitExec(ctx, "rev-parse HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD hash: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentBranchName returns the name of the current branch.
// Returns an error if HEAD is detached (not on a branch).
// This replaces the pattern: head.Name().Short() and checking head.Name().IsBranch()
func (r *Repo) GetCurrentBranchName(ctx context.Context) (string, error) {
	cmd := r.GitExec(ctx, "symbolic-ref --short HEAD")
	output, err := cmd.Output()
	if err != nil {
		// symbolic-ref fails when HEAD is detached
		return "", fmt.Errorf("HEAD is detached, not on a branch")
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRefHash returns the commit hash that a reference points to.
// refName should be a full reference name like "refs/heads/main" or "refs/remotes/origin/main".
// Returns an error if the reference doesn't exist.
// This replaces the pattern: Repository.Reference(name, true).Hash()
func (r *Repo) GetRefHash(ctx context.Context, refName string) (string, error) {
	cmd := r.GitExec(ctx, "rev-parse --verify %s", refName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrReferenceNotFound, refName)
	}
	return strings.TrimSpace(string(output)), nil
}

func (r *Repo) GetMainBranch(ctx context.Context, remoteName string) (string, error) {
	cmd := r.GitExec(ctx, "symbolic-ref refs/remotes/%s/HEAD", remoteName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not determine main branch for remote %s: %w", remoteName, err)
	}
	ref := strings.TrimSpace(string(output))
	prefix := fmt.Sprintf("refs/remotes/%s/", remoteName)
	return strings.TrimPrefix(ref, prefix), nil
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

func (r *Repo) TryRebaseOntoSilently(ctx context.Context, first string, onto Branch, interactive bool) bool {
	interactiveString := ""
	if interactive {
		interactiveString = "--interactive"
	}
	cmd := r.GitExec(ctx, "rebase %s --onto %s/%s %s^", interactiveString, GetRemoteName(), onto.RemoteName(), first)
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

func (r *Repo) TryLocalRebaseOntoSilently(
	ctx context.Context,
	first string,
	onto string,
) bool {
	cmd := r.GitExec(ctx, "rebase --onto %s^ %s", onto, first)
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

func (r *Repo) TryRebaseBranchOnto(ctx context.Context, parent string, onto Branch) bool {
	ontoName := onto.LocalName()
	if !onto.IsPr() {
		ontoName = fmt.Sprintf("%s/%s", GetRemoteName(), onto.RemoteName())
	}
	cmd := r.GitExec(ctx, "rebase --onto %s %s", ontoName, parent)
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
	branchName, err := r.GetCurrentBranchName(context.Background())
	if err != nil {
		return nil, errors.New("works only when on a branch")
	}
	pr, err := ExtractPrNumber(branchName)
	if err == nil {
		return NewLocalPr(r, pr), nil
	}
	return NewBranch(r, branchName), nil
}

func (r *Repo) GetBranch(name string) (Branch, error) {
	pr, err := ExtractPrNumber(name)
	if err == nil {
		return NewLocalPr(r, pr), nil
	}
	// Check if branch exists
	_, err = r.GetRefHash(context.Background(), fmt.Sprintf("refs/heads/%s", name))
	if err != nil {
		return nil, err
	}
	return NewBranch(r, name), nil
}

func (r *Repo) GetLocalTip(b Branch) (string, error) {
	return r.GetRefHash(context.Background(), fmt.Sprintf("refs/heads/%s", b.LocalName()))
}

func (r *Repo) GetRemoteTip(b Branch) (string, error) {
	return r.GetRefHash(context.Background(), fmt.Sprintf("refs/remotes/%s/%s", GetRemoteName(), b.RemoteName()))
}

// IsAncestor returns true if ancestor is an ancestor of descendant.
func (r *Repo) IsAncestor(ctx context.Context, ancestor, descendant string) bool {
	cmd := r.GitExec(ctx, "merge-base --is-ancestor %s %s", ancestor, descendant)
	return cmd.Run() == nil
}

func (r *Repo) CleanupAfterMerge(ctx context.Context, pr *LocalPr) {
	tip, err := r.GetLocalTip(pr)
	if err != nil {
		fmt.Printf("could not find the tip of branch %s.\n", pr.LocalBranch())
		return
	}
	fmt.Printf("Removing local branch %s. Tip was %s\n", pr.LocalBranch(), tip[0:7])
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
	currentBranch, _ := r.GetCurrentBranchName(ctx)
	for _, deleting := range toclean {
		if deleting.LocalName() == currentBranch {
			r.Checkout(ctx, r.BaseBranch())
		}
		r.DeleteLocalAndRemoteBranch(ctx, deleting)
		deleting.DeleteState()
	}
}

func (r *Repo) DeleteLocalAndRemoteBranch(ctx context.Context, branch Branch) error {
	r.GitExec(ctx, "branch -D %s", branch.LocalName()).Run()
	return r.DeleteRemoteBranch(ctx, branch)
}

func (r *Repo) DeleteRemoteBranch(ctx context.Context, branch Branch) error {
	ctx, cancel := context.WithTimeoutCause(
		ctx, GetGithubTimeout(),
		fmt.Errorf("push to %s too slow, increase github.timeout", GetRemoteName()),
	)
	defer cancel()
	cmd := r.GitExec(ctx, "%s %s :%s", GetPushCommand(), GetRemoteName(), branch.RemoteName())
	return cmd.Run()
}

func (r *Repo) DetachHead(ctx context.Context) error {
	cmd := r.GitExec(ctx, "checkout --detach HEAD")
	return cmd.Run()
}
