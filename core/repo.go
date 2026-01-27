package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Repo struct {
	gitDir   string
	workTree string
	s        *StateStore
}

func Current() *Repo {
	// Get git directory
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = "."
	gitDirBytes, err := cmd.Output()
	if err != nil {
		panic("You are not inside a git repository")
	}
	gitDir := strings.TrimSpace(string(gitDirBytes))
	if !path.IsAbs(gitDir) {
		gitDir = path.Join(".", gitDir)
	}

	// Get worktree directory
	cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = "."
	workTreeBytes, err := cmd.Output()
	if err != nil {
		panic("Could not determine worktree")
	}
	workTree := strings.TrimSpace(string(workTreeBytes))

	r := &Repo{
		gitDir:   gitDir,
		workTree: workTree,
	}
	r.s = NewStateStore(r)
	return r
}

func NewRepoFromGitRepo(repo *git.Repository) *Repo {
	// This function is kept for backward compatibility with tests
	// Tests still use go-git, so we need to extract the worktree path
	wt, err := repo.Worktree()
	if err != nil {
		panic("cannot work on bare repos")
	}
	workTree := wt.Filesystem.Root()

	// Get git dir
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = workTree
	gitDirBytes, _ := cmd.Output()
	gitDir := strings.TrimSpace(string(gitDirBytes))
	if !path.IsAbs(gitDir) {
		gitDir = path.Join(workTree, gitDir)
	}

	r := &Repo{
		gitDir:   gitDir,
		workTree: workTree,
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

func (r *Repo) Worktree() *git.Worktree {
	// This method is kept for backward compatibility with tests that use go-git
	// Production code should use Path() instead
	panic("Worktree() is deprecated, use Path() instead")
}

func (r *Repo) Path() string {
	return r.workTree
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
	cmd := r.GitExec(ctx, "push --force %s %s:refs/heads/%s", GetRemoteName(), hash, branch)
	return cmd.Run()
}

func (r *Repo) AllLocalPrs() (map[int]string, error) {
	cmd := r.GitExec(context.Background(),
		"for-each-ref --format=\"%%(refname:short) %%(objectname)\" refs/heads/")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list branches: %w", err)
	}

	result := make(map[int]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		branchName := parts[0]
		hash := parts[1]

		pr, err := ExtractPrNumber(branchName)
		if err == nil {
			result[pr] = hash
		}
	}
	return result, nil
}

// Returns all of the commits between the given hash
// and its merge-base with the base branch of the repository
func (r *Repo) GetCommitsNotInBaseBranch(hash string) ([]string, error) {
	// Get base branch tip
	baseRef := fmt.Sprintf("refs/remotes/%s/%s", GetRemoteName(), GetBaseBranch())
	baseTip, err := r.GetRef(context.Background(), baseRef)
	if err != nil {
		return nil, fmt.Errorf("could not find base branch: %w", err)
	}

	// Find merge base
	cmd := r.GitExec(context.Background(), "merge-base %s %s", hash, baseTip)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("no common ancestor: %w", err)
	}
	mergeBase := strings.TrimSpace(string(output))

	// Get commits between merge base and hash (excluding merge base)
	cmd = r.GitExec(context.Background(), "rev-list %s..%s", mergeBase, hash)
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	commits := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Reverse to match old order (child → parent)
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

// Takes all commit that are ancestors of headCommit and not in the base branch
// and walks them until it finds one that is the tip of an exisiting pr/XXX branch.
// Returns all the commits that were touched during the walk, in git children -> parent order.
// (e.g. the first commit is always headCommit)
func (r *Repo) FindBranchingPoint(headCommit string) (Branch, []string, error) {
	commits, err := r.GetCommitsNotInBaseBranch(headCommit)
	branchedCommits := make([]string, 0)
	if err != nil {
		return nil, nil, err
	}
	tracked, err := r.AllLocalPrs()
	if err != nil {
		return nil, nil, err
	}

	for _, commit := range commits {
		for number, tip := range tracked {
			if commit == tip {
				return NewLocalPr(r, number), branchedCommits, nil
			}
		}
		branchedCommits = append(branchedCommits, commit)
	}
	return r.BaseBranch(), commits, nil
}

func (r *Repo) PrForHead() (*LocalPr, bool) {
	branchName, err := r.GetCurrentBranchName()
	if err != nil {
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

func (r *Repo) Checkout(branch Branch) error {
	cmd := r.GitExec(context.Background(), "checkout %s", branch.LocalName())
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (r *Repo) CheckoutRef(ref string) error {
	// ref is now just a hash string
	cmd := r.GitExec(context.Background(), "checkout %s", ref)
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

// GetRef gets a reference hash by name (e.g., "refs/heads/main")
func (r *Repo) GetRef(ctx context.Context, refName string) (string, error) {
	cmd := r.GitExec(ctx, "rev-parse --verify %s", refName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ref not found: %s", refName)
	}
	return strings.TrimSpace(string(output)), nil
}

// SetRef sets a reference to point to a hash
func (r *Repo) SetRef(ctx context.Context, refName string, hash string) error {
	cmd := r.GitExec(ctx, "update-ref %s %s", refName, hash)
	return cmd.Run()
}

// DeleteRef deletes a reference
func (r *Repo) DeleteRef(ctx context.Context, refName string) error {
	cmd := r.GitExec(ctx, "update-ref -d %s", refName)
	return cmd.Run()
}

// GetCurrentBranchName returns the current branch name
func (r *Repo) GetCurrentBranchName() (string, error) {
	cmd := r.GitExec(context.Background(), "symbolic-ref --short HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not on a branch")
	}
	return strings.TrimSpace(string(output)), nil
}

// IsAncestor checks if ancestor is an ancestor of descendant
func (r *Repo) IsAncestor(ctx context.Context, ancestor, descendant string) bool {
	cmd := r.GitExec(ctx, "merge-base --is-ancestor %s %s", ancestor, descendant)
	err := cmd.Run()
	return err == nil // Exit code 0 = true, 1 = false
}

// ResolveRevision resolves any revision to a commit hash
func (r *Repo) ResolveRevision(ctx context.Context, rev string) (string, error) {
	cmd := r.GitExec(ctx, "rev-parse %s", rev)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("invalid revision: %s", rev)
	}
	return strings.TrimSpace(string(output)), nil
}

// Reference is for backward compatibility - returns just the hash as a string
func (r *Repo) Reference(refName plumbing.ReferenceName, resolved bool) (string, error) {
	return r.GetRef(context.Background(), refName.String())
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
	branchName, err := r.GetCurrentBranchName()
	if err != nil {
		return nil, err
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
	_, err = r.GetRef(context.Background(), "refs/heads/"+name)
	if err != nil {
		return nil, err
	}
	return NewBranch(r, name), nil
}

func (r *Repo) GetLocalTip(b Branch) (string, error) {
	refName := "refs/heads/" + b.LocalName()
	return r.GetRef(context.Background(), refName)
}

func (r *Repo) GetRemoteTip(b Branch) (string, error) {
	refName := fmt.Sprintf("refs/remotes/%s/%s", GetRemoteName(), b.RemoteName())
	return r.GetRef(context.Background(), refName)
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
	for _, deleting := range toclean {
		r.DeleteLocalAndRemoteBranch(ctx, deleting)
		deleting.DeleteState()
	}
}

func (r *Repo) DeleteLocalAndRemoteBranch(ctx context.Context, branch Branch) error {
	// Delete local branch using git command
	cmd := r.GitExec(ctx, "branch -D %s", branch.LocalName())
	_ = cmd.Run() // Ignore error if already deleted

	// Delete reference (backup, in case branch -D failed)
	refName := "refs/heads/" + branch.LocalName()
	r.DeleteRef(ctx, refName)

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

func (r *Repo) DetachHead(ctx context.Context) error {
	cmd := r.GitExec(ctx, "checkout --detach HEAD")
	return cmd.Run()
}

func (r *Repo) Head() (string, error) {
	ctx, cc := context.WithTimeout(context.Background(), time.Second*5)
	defer cc()
	cmd := r.GitExec(ctx, "rev-parse HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not get HEAD: %v", err)
	}
	return strings.TrimSpace(string(output)), nil
}
