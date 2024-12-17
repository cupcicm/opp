package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v3"
)

func CleanCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "clean",
		Aliases:     []string{"gc"},
		Description: "Deletes all local PRs that have been closed on github",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			repo.Fetch(ctx)
			localPrs := repo.AllPrs(ctx)
			pullRequests := gh(ctx).PullRequests()

			return cleaner{repo, localPrs, pullRequests}.Clean(ctx)
		},
	}
	return cmd
}

type cleanResult struct {
	err error
	pr  core.LocalPr
}

type cleaner struct {
	repo         *core.Repo
	localPrs     []core.LocalPr
	pullRequests core.GhPullRequest
}

func (c cleaner) Clean(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// results channel will receive the results of each pr cleaning operation
	results := c.cleaningPipeline(ctx)

	for result := range results {
		if result.err != nil {
			fmt.Printf("Issue when cleaning %d: %s", result.pr.PrNumber, result.err)
		}
		// TODO: also Print here the output in case of success (instead of directly in CleanupAfterMerge)
	}

	return nil
}

func pushLocalPrsToChannel(ctx context.Context, prs ...core.LocalPr) <-chan core.LocalPr {
	out := make(chan core.LocalPr)

	go func() {
		defer close(out)
		for _, pr := range prs {
			select {
			case out <- pr:
			case <-ctx.Done():
				return
			}
		}

	}()
	return out
}

func (c cleaner) cleanPRFromChannel(ctx context.Context, in <-chan core.LocalPr) <-chan cleanResult {
	out := make(chan cleanResult)
	go func() {
		defer close(out)
		for pr := range in {
			select {
			case out <- cleanResult{c.cleanPR(ctx, pr), pr}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (c cleaner) cleanPR(ctx context.Context, pr core.LocalPr) error {
	_, err := c.repo.GetRemoteTip(&pr)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		// The remote tip does not exist anymore : it has been deleted on the github repo.
		// Probably because the PR is either abandonned or merged.
		c.repo.CleanupAfterMerge(ctx, &pr)
	} else {
		githubPr, _, err := c.pullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
		if err != nil {
			return err
		}
		if *githubPr.State == "closed" {
			c.repo.CleanupAfterMerge(ctx, &pr)
		}
	}
	return nil
}

func mergeResultChannels(ctx context.Context, cs ...<-chan cleanResult) <-chan cleanResult {
	out := make(chan cleanResult)
	var wg sync.WaitGroup

	output := func(c <-chan cleanResult) {
		defer wg.Done()
		for n := range c {
			select {
			case out <- n:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func (c cleaner) cleaningPipeline(ctx context.Context) <-chan cleanResult {
	in := pushLocalPrsToChannel(ctx, c.localPrs...)

	resultChannelNb := int(math.Min(float64(runtime.GOMAXPROCS(0)), float64(len(c.localPrs))))

	resultChannels := make([]<-chan cleanResult, 0)
	for i := 1; i <= resultChannelNb; i++ {
		resultChannels = append(resultChannels, c.cleanPRFromChannel(ctx, in))
	}

	return mergeResultChannels(ctx, resultChannels...)
}
