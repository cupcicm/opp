package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/cupcicm/opp/cmd"
	"github.com/cupcicm/opp/core"
	"github.com/cupcicm/opp/core/story"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v56/github"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/urfave/cli/v3"
)

type Paths struct {
	Source      string
	Destination string
}

type TestRepo struct {
	*core.Repo
	Source           *git.Repository
	GithubRepo       *git.Repository
	Paths            Paths
	GithubMock       *GithubMock
	StoryFetcherMock *StoryFetcherMock
	App              *cli.Command
	Out              *strings.Builder
	In               *bytes.Buffer
}

func setConfig() {
	viper.Set("github.login", "cupcicm")
	viper.Set("repo.branch", "master")
	viper.Set("repo.github", "cupcicm/opp")
	viper.Set("repo.remote", "origin")
	viper.Set("story.tool", "linear")
	viper.Set("story.url", "https://my.base.url/browse")
	viper.Set("story.token", "my token")
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
	sourceConfig := core.Must(source.Config())
	sourceConfig.Raw.Section("core").SetOption("hooksPath", "")
	require.NoError(t, source.SetConfig(sourceConfig))
	githubConfig := core.Must(github.Config())
	githubConfig.Raw.Section("core").SetOption("hooksPath", "")
	require.NoError(t, github.SetConfig(githubConfig))

	repo := core.NewRepoFromGitRepo(source)
	mock := &GithubMock{
		PullRequestsMock: &PullRequestsMock{},
		IssuesMock:       &IssuesMock{},
	}
	storyFetcherMock := &StoryFetcherMock{}
	var out strings.Builder
	var in bytes.Buffer
	testRepo := TestRepo{
		Source:     source,
		GithubRepo: github,
		Repo:       repo,
		Paths: Paths{
			Source:      sourcePath,
			Destination: destPath,
		},
		GithubMock:       mock,
		StoryFetcherMock: storyFetcherMock,
		Out:              &out,
		In:               &in,
		App: cmd.MakeApp(&out, &in, repo, func(context.Context) core.Gh {
			return mock
		}, func(string, string) story.StoryFetcher {
			return storyFetcherMock
		}),
	}
	testRepo.PrepareSource()
	testRepo.AlwaysFailingEditor()
	return &testRepo
}

func (r *TestRepo) GetGithubMock(ctx context.Context) *GithubMock {
	return r.GithubMock
}

func (r *TestRepo) Run(command string, args ...string) error {
	return r.App.Run(context.Background(), append([]string{"opp", command}, args...))
}

func (r *TestRepo) Commit(msg string) plumbing.Hash {
	wt := core.Must(r.Source.Worktree())
	return core.Must(wt.Commit(msg, &git.CommitOptions{}))
}

func (r *TestRepo) RewriteLastCommit(msg string) {
	cmd := r.GitExec(context.Background(), "commit --amend -m \"%s\"", msg)
	cmd.Run()
}

func (r *TestRepo) AlwaysFailingEditor() {
	cmd := r.GitExec(context.Background(), "config core.editor true")
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func (r *TestRepo) PrepareSource() {
	r.GitExec(context.Background(), "config user.email test@robot.com").Run()
	r.GitExec(context.Background(), "config user.name Robot").Run()
	for i := 0; i < 10; i++ {
		os.WriteFile(path.Join(r.Path(), fmt.Sprint(i)), []byte(fmt.Sprint(i)), 0644)
	}
	wt := core.Must(r.Source.Worktree())
	wt.Add("1")
	hash := r.Commit("1")
	core.Must(r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.Paths.Destination},
	}))
	err := r.Push(context.Background(), hash.String(), "master")
	if err != nil {
		panic(err)
	}
	r.GitExec(context.Background(), "symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/master").Run()
	for i := 0; i < 5; i++ {
		wt.Add(strconv.Itoa(i))
		r.Commit(strconv.Itoa(i))
	}
}

func (r *TestRepo) AssertHasPr(t *testing.T, n int) *core.LocalPr {
	_, err := r.Source.Branch(fmt.Sprintf("pr/%d", n))
	assert.Nil(t, err)
	_, err = r.GithubRepo.Reference(plumbing.NewBranchReferenceName(fmt.Sprintf("cupcicm/pr/%d", n)), true)
	assert.Nil(t, err)

	return core.NewLocalPr(r.Repo, n)
}

func (r *TestRepo) CreatePr(t *testing.T, ref string, prNumber int, args ...string) *core.LocalPr {
	return r.CreatePrWithStories(t, ref, prNumber, []story.Story{}, false, "", args...)
}

func (r *TestRepo) CreatePrWithStories(t *testing.T, ref string, prNumber int, stories []story.Story, errStories bool, selectedStory string, args ...string) *core.LocalPr {
	r.GithubMock.IssuesMock.CallListAndReturnPr(prNumber - 1)
	r.GithubMock.PullRequestsMock.CallCreate(prNumber)
	r.StoryFetcherMock.CallFetchInProgressStories(stories, errStories)
	if !errStories && len(stories) > 0 {
		r.In.Write([]byte(fmt.Sprintf("%s\n", selectedStory)))
	}
	r.Run("pr", append(args, ref)...)
	return r.AssertHasPr(t, prNumber)
}

func (r *TestRepo) CreatePrAssertPrDetails(t *testing.T, ref string, prNumber int, prDetails github.NewPullRequest, args ...string) *core.LocalPr {
	pr := r.CreatePr(t, ref, prNumber, args...)
	r.GithubMock.PullRequestsMock.AssertCalled(t, "Create", mock.Anything, "cupcicm", "opp", &prDetails)
	return pr
}

func (r *TestRepo) CreatePrAssertPrDetailsWithStories(t *testing.T, ref string, prNumber int, stories []story.Story, errStories bool, selectedStory string, prDetails github.NewPullRequest, args ...string) *core.LocalPr {
	pr := r.CreatePrWithStories(t, ref, prNumber, stories, errStories, selectedStory, args...)
	r.GithubMock.PullRequestsMock.AssertCalled(t, "Create", mock.Anything, "cupcicm", "opp", &prDetails)
	return pr
}

func (r *TestRepo) MergePr(t *testing.T, pr *core.LocalPr) error {
	tip := core.Must(r.GetLocalTip(pr))
	r.GithubMock.PullRequestsMock.CallGetAndReturnMergeable(pr.PrNumber, true)
	r.GithubMock.PullRequestsMock.CallMerge(pr.PrNumber, tip)
	err := r.Run("merge", fmt.Sprintf("pr/%d", pr.PrNumber))
	if err != nil {
		return err
	}
	return r.Push(context.Background(), tip, r.BaseBranch().RemoteName())
}

type GithubMock struct {
	*PullRequestsMock
	*IssuesMock
}

func (g GithubMock) PullRequests() core.GhPullRequest {
	return g.PullRequestsMock
}
func (g GithubMock) Issues() core.GhIssues {
	return g.IssuesMock
}

type PullRequestsMock struct {
	mock.Mock
}
type IssuesMock struct {
	mock.Mock
}

func (m *PullRequestsMock) List(ctx context.Context, owner string, repo string, opt *github.PullRequestListOptions) ([]*github.PullRequest, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, opt)
	return args.Get(0).([]*github.PullRequest), nil, args.Error(2)
}

func (m *PullRequestsMock) Create(ctx context.Context, owner string, repo string, pull *github.NewPullRequest) (*github.PullRequest, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, pull)
	return args.Get(0).(*github.PullRequest), nil, args.Error(2)
}

func (m *PullRequestsMock) Get(ctx context.Context, owner string, repo string, number int) (*github.PullRequest, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, number)
	return args.Get(0).(*github.PullRequest), nil, args.Error(2)
}

func (m *PullRequestsMock) Merge(ctx context.Context, owner string, repo string, number int, commitMessage string, options *github.PullRequestOptions) (*github.PullRequestMergeResult, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, number, commitMessage, options)
	return args.Get(0).(*github.PullRequestMergeResult), nil, args.Error(2)
}
func (m *IssuesMock) ListByRepo(ctx context.Context, owner string, repo string, opts *github.IssueListByRepoOptions) ([]*github.Issue, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, opts)
	return args.Get(0).([]*github.Issue), nil, args.Error(2)
}

func (m *IssuesMock) CreateComment(ctx context.Context, owner string, repo string, number int, comment *github.IssueComment) (*github.IssueComment, *github.Response, error) {
	args := m.Mock.Called(ctx, owner, repo, comment)
	return args.Get(0).(*github.IssueComment), nil, args.Error(2)
}

func (m *IssuesMock) CallListAndReturnPr(prNumber int) {
	pr := github.Issue{
		Number: &prNumber,
	}
	m.On("ListByRepo", mock.Anything, "cupcicm", "opp", mock.Anything).Return(
		[]*github.Issue{&pr}, nil, nil,
	).Once()
}

func (m *PullRequestsMock) CallCreate(prNumber int) {
	pr := github.PullRequest{
		Number: &prNumber,
	}
	m.On("Create", mock.Anything, "cupcicm", "opp", mock.Anything).Return(
		&pr, nil, nil,
	).Once()
}

func (m *PullRequestsMock) CallGetAndReturnMergeable(prNumber int, mergeable bool) {
	reason := "dirty"
	if mergeable {
		reason = "clean"
	}
	state := "open"
	pr := github.PullRequest{
		Number:         &prNumber,
		Mergeable:      &mergeable,
		MergeableState: &reason,
		State:          &state,
	}
	m.On("Get", mock.Anything, "cupcicm", "opp", prNumber).Return(
		&pr, nil, nil,
	).Once()
}
func (m *PullRequestsMock) CallGetAndReturnMergeabilityBeingEvaluated(prNumber int) {
	reason := "clean"
	state := "open"
	pr := github.PullRequest{
		Number:         &prNumber,
		Mergeable:      nil,
		MergeableState: &reason,
		State:          &state,
	}
	m.On("Get", mock.Anything, "cupcicm", "opp", prNumber).Return(
		&pr, nil, nil,
	).Once()
}

func (m *PullRequestsMock) CallMerge(prNumber int, tip string) {
	response := github.PullRequestMergeResult{
		SHA: &tip,
	}
	m.On("Merge", mock.Anything, "cupcicm", "opp", prNumber, "", mock.Anything).Return(
		&response, nil, nil,
	).Once()
}

type StoryFetcherMock struct {
	mock.Mock
}

func (s *StoryFetcherMock) FetchInProgressStories(ctx context.Context) ([]story.Story, error) {
	args := s.Mock.Called(ctx)
	return args.Get(0).([]story.Story), args.Error(1)
}

func (s *StoryFetcherMock) CallFetchInProgressStories(stories []story.Story, fetchError bool) {
	var err error
	if fetchError {
		err = errors.New("Story fetch error")
	}

	s.On("FetchInProgressStories", mock.Anything).Return(stories, err).Once()
}
