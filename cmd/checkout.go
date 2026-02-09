package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v3"
)

func CheckoutCommand(repo *core.Repo) *cli.Command {
	cmd := &cli.Command{
		Name:    "checkout",
		Aliases: []string{"co"},
		Usage:   "Switch to a PR branch by number or alias",
		Description: `Switch to a PR branch using its number or an alias.

Examples:
  opp checkout            # Interactive selection with fzf
  opp checkout 123        # Checkout PR #123
  opp checkout pr/123     # Same as above
  opp checkout myfeature  # Checkout PR using alias

Note: You can also use native git commands with aliases:
  git checkout myfeature  # Works after creating alias with 'opp alias'`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() > 1 {
				return errors.New("too many arguments")
			}

			var prNumber int

			if !cmd.Args().Present() {
				// No argument provided - show interactive fzf selection
				selectedPr, err := selectPrWithFzf(ctx, repo)
				if err != nil {
					return err
				}
				prNumber = selectedPr
			} else {
				// Argument provided - resolve it
				arg := cmd.Args().First()
				var err error
				prNumber, err = ExtractPrNumberOrAlias(repo, arg)
				if err != nil {
					return fmt.Errorf("'%s' is not a valid PR number or alias", arg)
				}
			}

			pr := core.NewLocalPr(repo, prNumber)

			// Check that the branch exists locally
			if _, err := repo.GetLocalTip(pr); err != nil {
				return fmt.Errorf("PR branch %s does not exist locally", pr.LocalBranch())
			}

			// Checkout the branch
			if err := repo.Checkout(ctx, pr); err != nil {
				return fmt.Errorf("could not checkout %s: %w", pr.LocalBranch(), err)
			}

			// Show which branch we checked out, including any aliases
			aliases := repo.AliasesForPr(prNumber)
			if len(aliases) > 0 {
				fmt.Printf("Switched to %s (%s)\n", pr.LocalBranch(), aliases[0])
			} else {
				fmt.Printf("Switched to %s\n", pr.LocalBranch())
			}

			return nil
		},
	}
	return cmd
}

// selectPrWithFzf shows an interactive fzf selector for choosing a PR branch
func selectPrWithFzf(ctx context.Context, repo *core.Repo) (int, error) {
	// Get all PRs
	prs := repo.AllPrs(ctx)
	if len(prs) == 0 {
		return 0, fmt.Errorf("no PR branches available")
	}

	// Sort by modification time (most recent first)
	sort.Slice(prs, func(i, j int) bool {
		mtimeI, _ := repo.GetBranchMtime(&prs[i])
		mtimeJ, _ := repo.GetBranchMtime(&prs[j])
		return mtimeI.After(mtimeJ)
	})

	// Build options list with aliases
	options := make([]string, 0, len(prs))
	prMap := make(map[string]int) // Map option string to PR number

	for _, pr := range prs {
		aliases := repo.AliasesForPr(pr.PrNumber)
		var option string
		if len(aliases) > 0 {
			// Show alias first for easier searching
			option = fmt.Sprintf("%s -> pr/%d", aliases[0], pr.PrNumber)
		} else {
			option = fmt.Sprintf("pr/%d", pr.PrNumber)
		}
		options = append(options, option)
		prMap[option] = pr.PrNumber
	}

	// Show fzf selector
	selected, err := core.ChooseFromFzf(options, false, "Select PR branch")
	if err != nil {
		return 0, err
	}

	if len(selected) == 0 {
		return 0, fmt.Errorf("no selection made")
	}

	// Parse the selection to get PR number
	selection := selected[0]
	if prNumber, ok := prMap[selection]; ok {
		return prNumber, nil
	}

	// Fallback: try to extract PR number from selection string
	// Format is either "alias -> pr/123" or "pr/123"
	if strings.Contains(selection, "->") {
		parts := strings.Split(selection, "->")
		if len(parts) == 2 {
			selection = strings.TrimSpace(parts[1])
		}
	}
	return core.ExtractPrNumber(selection)
}
