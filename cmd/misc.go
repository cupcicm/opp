package cmd

import (
	"fmt"
	"strconv"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v3"
)

// PrFromFirstArgument returns the PR number supplied as a commandline argument, or if no argument is supplied,
// the PR for the current branch.
// The PR number can be supplied as a simple integer, or in the form `pr/$number`.
func PrFromFirstArgument(repo *core.Repo, cmd *cli.Command) (*core.LocalPr, bool, error) {
	var prParam string
	if cmd.Args().Present() {
		if cmd.NArg() > 1 {
			return nil, false, cli.Exit("too many arguments", 1)
		}
		prParam = cmd.Args().First()

	}
	return PrFromStringOrCurrentBranch(repo, prParam)
}

// PrFromStringOrCurrentBranch returns the PR based on the given string (if non-empty),
// or the current branch.
func PrFromStringOrCurrentBranch(repo *core.Repo, str string) (*core.LocalPr, bool, error) {
	var pr *core.LocalPr
	currentBranch := false
	if len(str) == 0 {
		// Use the PR that is the current branch
		currentBranch = true
		var found bool
		pr, found = repo.PrForHead()
		if !found {
			return nil, false, cli.Exit("please run opp with pr/XXX to specify a specific PR branch", 1)
		}
	} else {
		prNumber, err := strconv.Atoi(str)
		if err == nil {
			pr = core.NewLocalPr(repo, prNumber)
		} else {
			prNumber, err := core.ExtractPrNumber(str)
			if err != nil {
				return nil, false, cli.Exit(fmt.Errorf("%s is not a PR", str), 1)
			}
			pr = core.NewLocalPr(repo, prNumber)
		}
		headPr, headIsPr := repo.PrForHead()
		if headIsPr && headPr.PrNumber == pr.PrNumber {
			currentBranch = true
		}
	}
	return pr, currentBranch, nil
}

func PrintSuccess() {
	fmt.Println("✅")
}

func PrintFailure(err any) {
	if err == nil {
		fmt.Println("❌")
	} else {
		fmt.Printf("❌ (%s)\n", err)
	}
}
