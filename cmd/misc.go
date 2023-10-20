package cmd

import (
	"fmt"
	"strconv"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v2"
)

func PrFromFirstArgument(repo *core.Repo, cCtx *cli.Context) (*core.LocalPr, bool, error) {
	var pr *core.LocalPr
	currentBranch := false
	if !cCtx.Args().Present() {
		// Merge the PR that is the current branch
		currentBranch = true
		var found bool
		pr, found = repo.PrForHead()
		if !found {
			return nil, false, cli.Exit("please run opp merge pr/XXX to merge a specific branch", 1)
		}
	} else {
		if cCtx.NArg() > 1 {
			return nil, false, cli.Exit("too many arguments", 1)
		}
		prNumber, err := strconv.Atoi(cCtx.Args().First())
		if err == nil {
			pr = core.NewLocalPr(repo, prNumber)
		} else {
			prNumber, err := core.ExtractPrNumber(cCtx.Args().First())
			if err != nil {
				return nil, false, cli.Exit(fmt.Errorf("%s is not a PR", cCtx.Args().First()), 1)
			}
			pr = core.NewLocalPr(repo, prNumber)
			headPr, headIsPr := repo.PrForHead()
			if headIsPr && headPr.PrNumber == pr.PrNumber {
				currentBranch = true
			}
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
