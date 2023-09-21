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
					return cli.Exit("please run opp merge pr/XXX to merge a specific branch", 1)
				}
			} else {
				if cCtx.NArg() > 1 {
					return cli.Exit("too many arguments", 1)
				}
				prNumber, err := strconv.Atoi(cCtx.Args().First())
				if err == nil {
					pr = core.NewLocalPr(repo, prNumber)
				} else {
					prNumber, err := core.ExtractPrNumber(cCtx.Args().First())
					if err != nil {
						return cli.Exit(fmt.Errorf("%s is not a PR", cCtx.Args().First()), 1)
					}
					pr = core.NewLocalPr(repo, prNumber)
					headPr, headIsPr := repo.PrForHead()
					if headIsPr && headPr.PrNumber == pr.PrNumber {
						mergingCurrentBranch = true
					}
				}
			}
			merger := merger{Repo: repo, PullRequests: gh(cCtx.Context)}
			ancestors := pr.AllAncestors()
			mergeContext, cancel := context.WithTimeoutCause(
				cCtx.Context, core.GetGithubTimeout(),
				fmt.Errorf("merging too slow, increase github.timeout"),
			)
			defer cancel()
			if len(ancestors) >= 1 {
				fmt.Printf("%s is not mergeable because it has unmerged dependent PRs.\n", pr.Url())
				return fmt.Errorf("please merge %s first", ancestors[0].LocalBranch())
			}
			isMergeable, err := merger.IsMergeable(mergeContext, pr)
			if !isMergeable {
				return cli.Exit(err, 1)
			}
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
	githubPr, _, err := m.PullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
	if err != nil {
		return false, err
	}
	if githubPr.Merged != nil && *githubPr.Merged {
		return false, errors.New("already merged")
	}
	if githubPr.Mergeable == nil {
		return false, errors.New("still being checked by github")
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
		} else {
			fmt.Printf("❌ (%s)\n", err)
			return err
		}
	}
	return nil
}
