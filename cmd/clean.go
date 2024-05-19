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

			type cleanResult struct {
				err error
				pr  core.LocalPr
			}

			cleaningPipeline := func(ctx context.Context) chan cleanResult {
				results := make(chan cleanResult)
				maxNumberOfGoroutines := int64(runtime.GOMAXPROCS(0))
				sem := semaphore.NewWeighted(maxNumberOfGoroutines)

				cleanPr := func(pr core.LocalPr) {
					defer sem.Release(1)
					_, err := repo.GetRemoteTip(&pr)
					if errors.Is(err, plumbing.ErrReferenceNotFound) {
						// The remote tip does not exist anymore : it has been deleted on the github repo.
						// Probably because the PR is either abandonned or merged.
						repo.CleanupAfterMerge(cCtx.Context, &pr)
					} else {
						githubPr, _, err := pullRequests.Get(cCtx.Context, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
						if err != nil {
							results <- cleanResult{err, pr}
						}
						if *githubPr.State == "closed" {
							repo.CleanupAfterMerge(cCtx.Context, &pr)
						}
					}
					results <- cleanResult{nil, pr}
				}

				for _, pr := range localPrs {
					if err := sem.Acquire(ctx, 1); err != nil {
						results <- cleanResult{err, pr}
						break
					}
					go cleanPr(pr)
				}

				go func() {
					if err := sem.Acquire(ctx, maxNumberOfGoroutines); err != nil {
						log.Panicf("Failed to acquire semaphore: %v", err)
					}
					close(results)
				}()

				return results
			}

			for result := range cleaningPipeline(cCtx.Context) {
				if result.err != nil {
					fmt.Printf("Issue when cleaning %d: %s", result.pr.PrNumber, result.err)
				}
			}

			return nil
		},
	}
	return cmd
}
