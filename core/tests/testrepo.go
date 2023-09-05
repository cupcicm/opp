package tests

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/github"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type Paths struct {
	Source      string
	Destination string
}

type TestRepo struct {
	*core.Repo
	Source     *git.Repository
	GithubRepo *git.Repository
	Paths      Paths
	GithubMock *GithubMock
}

func setConfig() {
	viper.Set("github.login", "cupcicm")
	viper.Set("repo.branch", "master")
	viper.Set("repo.github", "cupcicm/opp")
	viper.Set("repo.remote", "origin")
}

func NewTestRepo(t *testing.T) *TestRepo {
	setConfig()
	dir := t.TempDir()
	sourcePath := path.Join(dir, "s")
	destPath := path.Join(dir, "g")
	os.Mkdir(sourcePath, 0755)
	os.Mkdir(destPath, 0755)
	source := core.Must(git.PlainInit(sourcePath, false))
	github := core.Must(git.PlainInit(destPath, true))

	repo := core.Repo{Repository: source}
	testRepo := TestRepo{
		Source:     source,
		GithubRepo: github,
		Repo:       &repo,
		Paths: Paths{
			Source:      sourcePath,
			Destination: destPath,
		},
		GithubMock: &GithubMock{},
	}
	testRepo.PrepareSource()
	return &testRepo
}

func (r *TestRepo) PrepareSource() {
	for i := 0; i < 10; i++ {
		os.WriteFile(path.Join(r.Path(), fmt.Sprint(i)), []byte(fmt.Sprint(i)), 0644)
	}
	wt := core.Must(r.Source.Worktree())
	wt.Add("1")
	hash := core.Must(wt.Commit("1", &git.CommitOptions{}))
	core.Must(r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.Paths.Destination},
	}))
	err := r.Push(context.Background(), hash, "master")
	if err != nil {
		panic(err)
	}
	for i := 0; i < 5; i++ {
		wt.Add(strconv.Itoa(i))
		wt.Commit(strconv.Itoa(i), &git.CommitOptions{})
	}
}

func (r *TestRepo) AssertHasPr(t *testing.T, n int) *core.LocalPr {
	_, err := r.Source.Branch(fmt.Sprintf("pr/%d", n))
	assert.Nil(t, err)
	_, err = r.GithubRepo.Reference(plumbing.NewBranchReferenceName(fmt.Sprintf("cupcicm/pr/%d", n)), true)
	assert.Nil(t, err)

	return core.NewLocalPr(r.Repo, n)
}

func (r *TestRepo) CreatePr(t *testing.T, ref string, prNumber int) *core.LocalPr {
	cmd := cmd.PrCommand(r.Repo, r.GithubMock)

	r.GithubMock.CallListAndReturnPr(prNumber - 1)
	r.GithubMock.CallCreate(prNumber)

	cmd.SetArgs([]string{ref})
	cmd.Execute()
	return r.AssertHasPr(t, prNumber)
}

type GithubMock struct {
	mock.Mock
}

func (m *GithubMock) List(ctx context.Context, owner string, repo string, opt *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, opt)
	return args.Get(0).([]*github.PullRequest), nil, args.Error(2)
}

func (m *GithubMock) Create(ctx context.Context, owner string, repo string, pull *github.NewPullRequest) (*github.PullRequest, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, pull)
	return args.Get(0).(*github.PullRequest), nil, args.Error(2)
}

func (m *GithubMock) Get(ctx context.Context, owner string, repo string, number int) (*github.PullRequest, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, number)
	return args.Get(0).(*github.PullRequest), nil, args.Error(2)
}

func (m *GithubMock) Merge(ctx context.Context, owner string, repo string, number int, commitMessage string, options *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, number, commitMessage, options)
	return nil, nil, args.Error(2)
}

func (m *GithubMock) CallListAndReturnPr(prNumber int) {
	pr := github.PullRequest{
		Number: &prNumber,
	}
	m.On("List", mock.Anything, "cupcicm", "opp", mock.Anything).Return(
		[]*github.PullRequest{&pr}, nil, nil,
	).Once()
}

func (m *GithubMock) CallCreate(prNumber int) {
	pr := github.PullRequest{
		Number: &prNumber,
	}
	m.On("Create", mock.Anything, "cupcicm", "opp", mock.Anything).Return(
		&pr, nil, nil,
	).Once()
}

func (m *GithubMock) CallGetAndReturnMergeable(prNumber int, mergeable bool) {
	reason := "dirty"
	pr := github.PullRequest{
		Number:         &prNumber,
		Mergeable:      &mergeable,
		MergeableState: &reason,
	}
	m.On("Get", mock.Anything, "cupcicm", "opp", prNumber).Return(
		&pr, nil, nil,
	).Once()
}

func (m *GithubMock) CallMerge(prNumber int) {
	m.On("Merge", mock.Anything, "cupcicm", "opp", prNumber, "", mock.Anything).Return(
		nil, nil, nil,
	).Once()
}
