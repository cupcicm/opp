package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/exp/slices"
)

type LocalPr struct {
	PrNumber int
	state    *BranchState
	Repo     *Repo
}

type branch struct {
	Repo  *Repo
	Name  string
	Local bool

	state *BranchState
}

type Branch interface {
	IsPr() bool
	LocalName() string
	RemoteName() string
	Tag() string
}

func (b *LocalPr) ReloadState() {
	b.state = b.Repo.StateStore().GetBranchState(b)
}

func (b *LocalPr) DeleteState() {
	b.Repo.StateStore().DeleteBranchState(b)
}

func NewLocalPr(repo *Repo, prNumber int) *LocalPr {
	pr := LocalPr{
		Repo:     repo,
		PrNumber: prNumber,
	}
	pr.state = repo.StateStore().GetBranchState(&pr)
	return &pr
}

func (pr *LocalPr) StateIsLoaded() bool {
	return pr.state != nil
}

func NewBranch(repo *Repo, name string) Branch {
	branch := branch{
		Repo: repo,
		Name: name,
	}
	branch.state = repo.StateStore().GetBranchState(&branch)
	return &branch
}

func (b *LocalPr) IsPr() bool {
	return true
}

func (b *LocalPr) LocalName() string {
	return b.LocalBranch()
}

func (b *LocalPr) RemoteName() string {
	return b.RemoteBranch()
}

func (b *branch) IsPr() bool {
	return false
}

func (b *branch) LocalName() string {
	return b.Name
}

func (b *branch) RemoteName() string {
	return b.Name
}

func (b *branch) Tag() string {
	return b.state.Tag
}

func (b *LocalPr) Url() string {
	return fmt.Sprintf("https://github.com/%s/pull/%d", GetGithubRepo(), b.PrNumber)
}

func (b *LocalPr) Tag() string {
	return b.state.Tag
}

func (b *LocalPr) GetAncestor() (Branch, error) {
	if b.state.Ancestor.Name == "" {
		return b.Repo.BaseBranch(), errors.New("no ancestors have been set")
	}
	number, errNotAPr := ExtractPrNumber(b.state.Ancestor.Name)
	if errNotAPr != nil {
		return NewBranch(b.Repo, b.state.Ancestor.Name), nil
	}
	return NewLocalPr(b.Repo, number), nil
}

func (b *LocalPr) AncestorTips() []string {
	ancestor, err := b.GetAncestor()
	if err != nil {
		ancestor = b.Repo.BaseBranch()
	}
	var tips []string
	if ancestor.IsPr() {
		tips = ancestor.(*LocalPr).state.KnownTips
	}
	return append(tips, b.state.Ancestor.KnownTips...)
}

func (b *LocalPr) RememberCurrentTip() {
	tip := Must(b.Repo.GetLocalTip(b))
	b.AddKnownTip(tip.Hash)
}

func (b *LocalPr) AddKnownTip(tip plumbing.Hash) {
	b.state.KnownTips = append(b.state.KnownTips, tip.String())
	b.Repo.StateStore().SaveBranchState(b, b.state)
}

func (b *LocalPr) SetKnownTipsFromAncestor(ancestor *LocalPr) {
	b.state.Ancestor.KnownTips = ancestor.state.KnownTips
	b.Repo.StateStore().SaveBranchState(b, b.state)
}

func (b *LocalPr) SetAncestor(branch Branch) {
	b.state.Ancestor.Name = branch.LocalName()
	b.Repo.StateStore().SaveBranchState(b, b.state)
}

// Returns all ancestor but not itself.
func (b *LocalPr) AllAncestors() []*LocalPr {
	all := b.allAncestors(make([]*LocalPr, 0))[1:]
	slices.Reverse(all)
	return all
}

func (b *LocalPr) allAncestors(descendents []*LocalPr) []*LocalPr {
	descendents = append(descendents, b)
	ancestor, err := b.GetAncestor()
	if err != nil {
		return descendents
	}
	ancestorPr, ok := ancestor.(*LocalPr)
	if !ok {
		return descendents
	}
	return ancestorPr.allAncestors(descendents)
}

func (b *LocalPr) LocalBranch() string {
	return LocalBranchForPr(b.PrNumber)
}

func (b *LocalPr) RemoteBranch() string {
	return RemoteBranchForPr(b.PrNumber)
}

func RemoteBranchForPr(number int) string {
	return fmt.Sprintf("%s/pr/%d", GetGithubUsername(), number)
}

func LocalBranchForPr(number int) string {
	return fmt.Sprintf("pr/%d", number)
}

func (b *LocalPr) Push(ctx context.Context) error {
	tip, err := b.Repo.GetLocalTip(b)
	if err != nil {
		return fmt.Errorf("PR %s has no local branch", b.LocalBranch())
	}
	return b.Repo.Push(ctx, tip.Hash, b.RemoteBranch())
}

var ErrNotAPrBranch = errors.New("not a pr branch")

func ExtractPrNumber(branchname string) (int, error) {
	number, err := strconv.Atoi(branchname)
	if err == nil {
		return number, nil
	}
	parts := strings.Split(branchname, "/")
	if len(parts) != 2 {
		return 0, ErrNotAPrBranch
	}
	if parts[0] != "pr" {
		return 0, ErrNotAPrBranch
	}
	number, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, ErrNotAPrBranch
	}
	return number, nil
}
