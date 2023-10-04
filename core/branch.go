package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

type LocalPr struct {
	PrNumber int
	HasState bool
	state    *state
	Repo     *Repo
}

type branch struct {
	Repo  *Repo
	Name  string
	Local bool
}

type Branch interface {
	IsPr() bool
	LocalName() string
	RemoteName() string
}

type state struct {
	Ancestor struct {
		Name string
	}
}

func (b *LocalPr) StateFile() string {
	return path.Join(b.Repo.DotOpDir(), "state", b.LocalBranch())
}

func (b *LocalPr) saveState() error {
	content, err := yaml.Marshal(b.state)
	if err != nil {
		return err
	}
	os.MkdirAll(path.Dir(b.StateFile()), 0755)
	return os.WriteFile(b.StateFile(), content, 0644)
}

func (b *LocalPr) loadState() {
	hasState := FileExists(b.StateFile())
	if hasState {
		b.state = Must(loadState(b.StateFile()))
	} else {
		b.state = &state{}
	}
	b.HasState = true
}

func loadState(file string) (*state, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	state := state{}
	err = yaml.Unmarshal(content, &state)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (b *LocalPr) DeleteState() {
	os.Remove(b.StateFile())
}

func NewLocalPr(repo *Repo, prNumber int) *LocalPr {
	pr := LocalPr{
		Repo:     repo,
		PrNumber: prNumber,
	}
	pr.loadState()
	return &pr
}

func NewBranch(repo *Repo, name string) Branch {
	branch := branch{
		Repo: repo,
		Name: name,
	}
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

func (b *LocalPr) Url() string {
	return fmt.Sprintf("https://github.com/%s/pull/%d", GetGithubRepo(), b.PrNumber)
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

func (b *LocalPr) SetAncestor(branch Branch) {
	b.state.Ancestor.Name = branch.LocalName()
	b.saveState()
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

type PrStates struct {
	Repo *Repo
}

func (s PrStates) StatesPath() string {
	return path.Join(s.Repo.DotOpDir(), "state", "pr")
}

func (s PrStates) AllPrs(ctx context.Context) []LocalPr {
	files := Must(os.ReadDir(s.StatesPath()))
	prs := make([]LocalPr, 0, len(files))
	toclean := make([]*LocalPr, 0)
	for _, file := range files {
		num, err := strconv.Atoi(file.Name())
		if err != nil {
			continue
		}
		pr := NewLocalPr(s.Repo, num)
		// Check that the branch exists.
		_, err = s.Repo.GetLocalTip(pr)
		if err != nil {
			// This PR does not exist locally: clean it.
			toclean = append(toclean, pr)
		} else {
			prs = append(prs, *pr)
		}
	}
	s.Repo.CleanupMultiple(ctx, toclean, prs)
	return prs
}
