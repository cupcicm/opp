package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cupcicm/opp/core"
	"github.com/google/go-github/github"
	"github.com/urfave/cli/v2"
)

type merger struct {
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

func MergeCommand(repo *core.Repo, gh func(context.Context) core.GhPullRequest) *cli.Command {
	cmd := &cli.Command{
		Name:    "merge",
		Aliases: []string{"m"},
		Action: func(cCtx *cli.Context) error {
			var pr *core.LocalPr
			var mergingCurrentBranch bool
			if !cCtx.Args().Present() {
				// Merge the PR that is the current branch
				pr, mergingCurrentBranch = repo.PrForHead()
				if !mergingCurrentBranch {
					return errors.New("please run opp merge pr/XXX to merge a specific branch")
				}
			} else {
				if cCtx.NArg() > 1 {
					return errors.New("too many arguments")
				}
				prNumber, err := strconv.Atoi(cCtx.Args().First())
				if err == nil {
					pr = core.NewLocalPr(repo, prNumber)
				} else {
					prNumber, err := core.ExtractPrNumber(cCtx.Args().First())
					if err != nil {
						return fmt.Errorf("%s is not a PR", cCtx.Args().First())
					}
					pr = core.NewLocalPr(repo, prNumber)
				}
			}
			merger := merger{Repo: repo, PullRequests: gh(cCtx.Context)}
			ancestors := pr.AllAncestors()
			if len(ancestors) >= 1 {
				fmt.Printf("%s is not mergeable because it has unmerged dependent PRs.\n", pr.Url())
				return fmt.Errorf("please merge %s first", ancestors[0].LocalBranch())
			}
			isMergeable, err := merger.IsMergeable(cCtx.Context, pr)
			if !isMergeable {
				return err
			}
			merger.Merge(cCtx.Context, pr)
			if mergingCurrentBranch {
				repo.Checkout(repo.BaseBranch())
			}
			return nil
		},
	}
	return cmd
}

// Is this PR, separately from its ancestor, mergeable in itself ?
func (m *merger) IsMergeable(ctx context.Context, pr *core.LocalPr) (bool, error) {
	githubPr, _, err := m.PullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
	if err != nil {
		return false, err
	}
	if githubPr.Mergeable == nil {
		return false, fmt.Errorf("%s is still being checked by github. Retry soon", pr.Url())
	}
	if *githubPr.Mergeable {
		return true, nil
	}
	switch *githubPr.MergeableState {
	case "dirty":
		return false, fmt.Errorf("cannot be merged cleanly into %s", m.Repo.BaseBranch().RemoteName())
	case "blocked":
		return false, errors.New("has some failing checks")
	default:
		return false, errors.New("cannot be merged right now")
	}
}

func (m *merger) Merge(ctx context.Context, prs ...*core.LocalPr) error {
	for _, pr := range prs {
		tip, err := m.Repo.GetLocalTip(pr)
		if err != nil {
			return err
		}
		fmt.Printf("Merging %s... ", pr.LocalBranch())
		_, r, err := m.PullRequests.Merge(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber, "",
			&github.PullRequestOptions{
				SHA:         tip.Hash.String(),
				MergeMethod: core.GetGithubMergeMethod(),
			})
		if r != nil && r.StatusCode == http.StatusConflict {
			fmt.Println("(❌ - wrong remote tip)")
			return fmt.Errorf("did not merge %s", pr.LocalBranch())
		}
		if err == nil {
			fmt.Printf("✅\n")
			m.Repo.CleanupAfterMerge(ctx, pr)
		} else {
			fmt.Printf("❌ (%s)\n", err)
		}
	}
	return nil
}
