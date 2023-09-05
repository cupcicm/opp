package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

type status struct {
	Out          io.Writer
	Repo         *core.Repo
	PullRequests core.GhPullRequest
}

func StatusCommand(out io.Writer, repo *core.Repo, gh core.GhPullRequest) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"s"},
		Args:    cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			status := status{Out: out, Repo: repo, PullRequests: gh}
			repo.Fetch(cmd.Context())
			localPrs := repo.AllPrs(cmd.Context())
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
						status.PrintStatus(cmd.Context(), ancestor, 3)
					}

				} else {
					fmt.Fprintf(out, "PR #%d. ", pr.PrNumber)
					status.PrintStatus(cmd.Context(), &pr, 0)
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
	if *githubPr.Mergeable {
		return true, nil
	}
	switch *githubPr.MergeableState {
	case "dirty":
		return false, fmt.Errorf("cannot be merged cleanly into %s", s.Repo.BaseBranch().RemoteName())
	case "blocked":
		return false, errors.New("has some failing checks")
	default:
		return false, errors.New("cannot be merged right now")
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
