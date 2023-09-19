package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/slices"
)

type status struct {
	Out          io.Writer
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

func StatusCommand(out io.Writer, repo *core.Repo, gh func(context.Context) core.GhPullRequest) *cli.Command {
	cmd := &cli.Command{
		Name:    "status",
		Aliases: []string{"s"},
		Action: func(cCtx *cli.Context) error {
			if cCtx.NArg() > 0 {
				return cli.Exit("too many arguments", 1)
			}
			status := status{Out: out, Repo: repo, PullRequests: gh(cCtx.Context)}
			repo.Fetch(cCtx.Context)
			localPrs := repo.AllPrs(cCtx.Context)
			alreadyMentioned := make(map[int]bool)
			slices.SortFunc(localPrs, func(pr1 core.LocalPr, pr2 core.LocalPr) int {
				if len(pr1.AllAncestors()) > len(pr2.AllAncestors()) {
					return -1
				}
				return 1
			})
			for _, pr := range localPrs {
				if _, ok := alreadyMentioned[pr.PrNumber]; ok {
					continue
				}
				alreadyMentioned[pr.PrNumber] = true
				ancestors := pr.AllAncestors()
				if len(ancestors) >= 1 {
					fmt.Fprintf(out, "PR chain #%d\n", ancestors[0].PrNumber)
					for i, ancestor := range append(ancestors, &pr) {
						alreadyMentioned[ancestor.PrNumber] = true
						fmt.Fprintf(out, "  %d. ", i+1)
						status.PrintStatus(cCtx.Context, ancestor, 3)
					}

				} else {
					fmt.Fprintf(out, "PR #%d. ", pr.PrNumber)
					status.PrintStatus(cCtx.Context, &pr, 0)
				}
			}
			return nil
		},
	}
	return cmd
}

func (m *status) PrintStatus(ctx context.Context, pr *core.LocalPr, indent int) {
	mergeable, err := m.isMergeable(ctx, pr)
	isUpToDate := core.Must(m.isUpToDate(ctx, pr))
	var mergeableString string
	var isUpToDateString string
	fmt.Fprintln(m.Out, pr.Url())
	if mergeable {
		mergeableString = "✅"
	} else {
		mergeableString = fmt.Sprintf("❌ - %s", err)
	}
	if isUpToDate {
		isUpToDateString = "✅"
	} else {
		isUpToDateString = "❌"
	}
	fmt.Fprintf(m.Out, "%smergeable  %s\n", strings.Repeat(" ", indent+2), mergeableString)
	fmt.Fprintf(m.Out, "%sup-to-date %s\n", strings.Repeat(" ", indent+2), isUpToDateString)
}

// Is this PR, separately from its ancestor, mergeable in itself ?
func (s *status) isMergeable(ctx context.Context, pr *core.LocalPr) (bool, error) {
	githubPr, _, err := s.PullRequests.Get(ctx, core.GetGithubOwner(), core.GetGithubRepoName(), pr.PrNumber)
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
		return false, fmt.Errorf("cannot be merged cleanly into %s", s.Repo.BaseBranch().RemoteName())
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

func (s *status) isUpToDate(ctx context.Context, pr *core.LocalPr) (bool, error) {
	remote, err := s.Repo.Reference(plumbing.NewRemoteReferenceName(core.GetRemoteName(), pr.RemoteBranch()), true)
	if err != nil {
		return false, err
	}
	local, err := s.Repo.Reference(plumbing.NewBranchReferenceName(pr.LocalBranch()), true)
	if err != nil {
		return false, err
	}
	return local.Hash() == remote.Hash(), nil
}
