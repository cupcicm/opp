package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v2"
	"golang.org/x/sync/semaphore"
)

func CleanCommand(repo *core.Repo, gh func(context.Context) core.Gh) *cli.Command {
	cmd := &cli.Command{
		Name:        "clean",
		Aliases:     []string{"gc"},
		Description: "Deletes all local PRs that have been closed on github",
		Action: func(cCtx *cli.Context) error {
			repo.Fetch(cCtx.Context)
			localPrs := repo.AllPrs(cCtx.Context)
			pullRequests := gh(cCtx.Context).PullRequests()

			return cleaner{repo, localPrs, pullRequests}.Clean(cCtx.Context)
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
	results, err := c.cleaningPipeline(ctx)
	if err != nil {
		return err
	}

	for result := range results {
		if result.err != nil {
			fmt.Printf("Issue when cleaning %d: %s", result.pr.PrNumber, result.err)
		}
		// TODO: also Print here the output in case of success (instead of directly in CleanupAfterMerge)
	}

	return nil
}

func (c cleaner) cleaningPipeline(ctx context.Context) (chan cleanResult, error) {
	results := make(chan cleanResult)

	// The semaphore will be used to limit the number of goroutines that can be launched in parallel.
	maxNumberOfGoroutines := int64(runtime.GOMAXPROCS(0))
	sem := semaphore.NewWeighted(maxNumberOfGoroutines)

	// Wait for the semaphore to be fully released before closing the results channel.
	defer func() {
		go func() {
			err := sem.Acquire(ctx, maxNumberOfGoroutines)
			if err != nil && ctx.Err() == nil {
				log.Panicf("What is the error if not the context error? Error: %s.", err)
			}
			close(results)
		}()
	}()

	cleanPr := func(pr core.LocalPr) {
		defer sem.Release(1)
		_, err := c.repo.GetRemoteTip(&pr)
		if err != nil {
			fmt.Printf("error Get RemoteTip: %s\n", err.Error())
		} else {
			fmt.Printf("no error GetRemoteTip\n")
		}

		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			fmt.Println("reference not found. Cleanup and merge")
			// The remote tip does not exist anymore : it has been deleted on the github repo.
			// Probably because the PR is either abandonned or merged.
			c.repo.CleanupAfterMerge(ctx, &pr)
		} else {
			githubPr, _, err := c.pullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
			if err != nil {
				fmt.Printf("error Get PullRequests: %s", err.Error())
				select {
				case results <- cleanResult{err, pr}:
				case <-ctx.Done():
				}
				return
			}
			if *githubPr.State == "closed" {
				fmt.Println("PR closed. Cleanup and merge")
				c.repo.CleanupAfterMerge(ctx, &pr)
			}
		}
		select {
		case results <- cleanResult{nil, pr}:
		case <-ctx.Done():
		}
	}

	for _, pr := range c.localPrs {
		if err := sem.Acquire(ctx, 1); err != nil {
			return nil, err
		}
		go cleanPr(pr)
	}

	return results, nil
}
