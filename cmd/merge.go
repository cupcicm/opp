package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cupcicm/opp/core"
	"github.com/google/go-github/v56/github"
	"github.com/urfave/cli/v3"
)

var (
	ErrBeingEvaluated         = errors.New("still being checked by github")
	mergeabilityCheckInterval = time.Second * 2
	mergeabilityCheckTimeout  = time.Second * 30
)

type merger struct {
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

func MergeCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:    "merge",
		Aliases: []string{"m"},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			pr, mergingCurrentBranch, err := PrFromFirstArgument(repo, cmd)
			if err != nil {
				return err
			}
			merger := merger{Repo: repo, PullRequests: gh(ctx).PullRequests()}
			ancestors := pr.AllAncestors()
			if len(ancestors) >= 1 {
				fmt.Printf("%s is not mergeable because it has unmerged dependent PRs.\n", pr.Url())
				return fmt.Errorf("please merge %s first", ancestors[0].LocalBranch())
			}
			fmt.Print("Checking mergeability... ")

			isMergeable, err := merger.IsMergeable(ctx, pr)

			if errors.Is(err, ErrBeingEvaluated) {
				isMergeable, err = merger.WaitForMergeability(ctx, pr)
			}
			if !isMergeable {
				PrintFailure(nil)
				return cli.Exit(err, 1)
			}
			PrintSuccess()
			mergeContext, cancel := context.WithTimeoutCause(
				ctx, core.GetGithubTimeout(),
				fmt.Errorf("merging too slow, increase github.timeout"),
			)
			defer cancel()
			if err := merger.Merge(mergeContext, pr); err != nil {
				return cli.Exit("could not merge", 1)
			}
			if mergingCurrentBranch {
				repo.Checkout(repo.BaseBranch())
			}
			repo.CleanupAfterMerge(mergeContext, pr)
			return nil
		},
	}
	return cmd
}

// Is this PR, separately from its ancestor, mergeable in itself ?
func (m *merger) IsMergeable(ctx context.Context, pr *core.LocalPr) (bool, error) {
	mergeableContext, cancel := context.WithTimeoutCause(
		ctx, core.GetGithubTimeout(),
		fmt.Errorf("checking if mergeable is too slow, increase github.timeout"),
	)
	defer cancel()
	githubPr, _, err := m.PullRequests.Get(mergeableContext, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
	if err != nil {
		return false, err
	}
	if githubPr.Merged != nil && *githubPr.Merged {
		return false, errors.New("already merged")
	}
	if githubPr.Mergeable == nil {
		return false, ErrBeingEvaluated
	}
	switch *githubPr.MergeableState {
	case "dirty":
		return false, fmt.Errorf("cannot be merged cleanly into %s", m.Repo.BaseBranch().RemoteName())
	case "blocked":
		return false, errors.New("not authorized to merge")
	case "unstable":
		return false, errors.New("has some failing checks")
	case "draft":
		return false, errors.New("draft PR")
	case "clean":
		return true, nil
	default:
		return false, errors.New("not mergeable")
	}
}

func (m *merger) WaitForMergeability(ctx context.Context, pr *core.LocalPr) (bool, error) {
	t := time.NewTicker(mergeabilityCheckInterval)
	mergeabilityCheckCtx, cancel := context.WithTimeout(ctx, mergeabilityCheckTimeout)
	defer t.Stop()
	defer cancel()
	for i := 0; i < 5; i++ {
		select {
		case <-t.C:
			// Try again
		case <-ctx.Done():
			return false, ErrBeingEvaluated
		}
		mergeable, err := m.IsMergeable(mergeabilityCheckCtx, pr)
		if mergeable || err != ErrBeingEvaluated {
			return mergeable, err
		}
	}
	return false, ErrBeingEvaluated
}

func (m *merger) Merge(ctx context.Context, pr *core.LocalPr) error {
	tip, err := m.Repo.GetLocalTip(pr)
	if err != nil {
		return err
	}
	fmt.Printf("Merging %s... ", pr.LocalBranch())
	merge, r, err := m.PullRequests.Merge(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber, "",
		&github.PullRequestOptions{
			SHA:         tip,
			MergeMethod: core.GetGithubMergeMethod(),
		})
	if r != nil && r.StatusCode == http.StatusConflict {
		PrintFailure("wrong remote tip")
		return fmt.Errorf("did not merge %s", pr.LocalBranch())
	}
	if err != nil {
		PrintFailure(err)
		return err
	}
	PrintSuccess()
	pr.AddKnownTip(merge.GetSHA())
	return nil
}

func SetShortMergeabilityIntervalForTests() func() {
	initial := mergeabilityCheckInterval
	mergeabilityCheckInterval = time.Millisecond
	return func() {
		mergeabilityCheckInterval = initial
	}
}
