package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
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

func (b *LocalPr) GetAncestor() (Branch, error) {
	if b.state.Ancestor.Name == "" {
		return nil, errors.New("no ancestors have been set")
	}
	number, errNotAPr := ExtractPrNumber(b.state.Ancestor.Name)
	if errNotAPr != nil {
		return NewBranch(b.Repo, b.state.Ancestor.Name), nil
	}
	pr := NewLocalPr(b.Repo, number)
	_, err := b.Repo.GetLocalTip(pr)
	if err == plumbing.ErrReferenceNotFound {
		// This PR has been merged / deleted locally.
		ancestor, err := pr.GetAncestor()
		if err != nil {
			return nil, err
		}
		b.SetAncestor(ancestor)
		b.Repo.CleanupAfterMerge(pr)
		return ancestor, nil
	}
	return pr, nil
}

func (b *LocalPr) SetAncestor(branch Branch) {
	b.state.Ancestor.Name = branch.LocalName()
	b.saveState()
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
	parts := strings.Split(branchname, "/")
	if len(parts) != 2 {
		return 0, ErrNotAPrBranch
	}
	if parts[0] != "pr" {
		return 0, ErrNotAPrBranch
	}
	number, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, ErrNotAPrBranch
	}
	return number, nil
}
