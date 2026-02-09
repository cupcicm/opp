package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/cupcicm/opp/core"
	"github.com/urfave/cli/v3"
)

func AliasCommand(repo *core.Repo) *cli.Command {
	var deleteFlag bool
	var renameFlag bool

	cmd := &cli.Command{
		Name:        "alias",
		Aliases:     []string{"a"},
		Usage:       "Manage branch aliases for PRs",
		Description: "Create, delete, or rename aliases. Run without args to list. Works with git commands too.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "delete",
				Aliases:     []string{"d"},
				Usage:       "Delete the specified alias",
				Destination: &deleteFlag,
			},
			&cli.BoolFlag{
				Name:        "rename",
				Aliases:     []string{"r"},
				Usage:       "Rename an alias: opp alias -r oldname newname",
				Destination: &renameFlag,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			args := cmd.Args()

			// No arguments: list aliases
			if !args.Present() {
				return listAliases(repo)
			}

			aliasName := args.First()

			// Delete alias
			if deleteFlag {
				if err := repo.DeleteAlias(aliasName); err != nil {
					return err
				}
				fmt.Printf("Deleted alias '%s'\n", aliasName)
				return nil
			}

			// Rename alias: opp alias -r oldname newname
			if renameFlag {
				if args.Len() < 2 {
					return errors.New("rename requires two arguments: opp alias -r oldname newname")
				}
				oldName := args.First()
				newName := args.Get(1)

				// Get the PR number from the old alias
				prNumber, exists := repo.ResolveAlias(oldName)
				if !exists {
					return fmt.Errorf("alias '%s' not found", oldName)
				}

				// Delete old alias
				if err := repo.DeleteAlias(oldName); err != nil {
					return fmt.Errorf("could not delete old alias: %w", err)
				}

				// Create new alias
				if err := repo.CreateAlias(newName, prNumber); err != nil {
					// Try to restore the old alias if new one fails
					repo.CreateAlias(oldName, prNumber)
					return fmt.Errorf("could not create new alias: %w", err)
				}

				fmt.Printf("Renamed alias '%s' -> '%s' (pr/%d)\n", oldName, newName, prNumber)
				return nil
			}

			// Create alias
			var prNumber int
			if args.Len() >= 2 {
				// Alias for specific PR: opp alias myfeature 123
				var err error
				prNumber, err = ExtractPrNumberOrAlias(repo, args.Get(1))
				if err != nil {
					return fmt.Errorf("invalid PR: %s", args.Get(1))
				}
			} else {
				// Alias for current branch: opp alias myfeature
				pr, found := repo.PrForHead()
				if !found {
					return errors.New("not on a PR branch; specify a PR number: opp alias <name> <pr>")
				}
				prNumber = pr.PrNumber
			}

			if err := repo.CreateAlias(aliasName, prNumber); err != nil {
				return err
			}

			fmt.Printf("Created alias '%s' -> pr/%d\n", aliasName, prNumber)
			return nil
		},
	}
	return cmd
}

func listAliases(repo *core.Repo) error {
	aliases, err := repo.ListAliases()
	if err != nil {
		return fmt.Errorf("could not list aliases: %w", err)
	}

	if len(aliases) == 0 {
		fmt.Println("No aliases defined.")
		fmt.Println("Create one with: opp alias <name> [pr]")
		return nil
	}

	// Sort aliases by name
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i].Name < aliases[j].Name
	})

	fmt.Println("Aliases:")
	for _, alias := range aliases {
		fmt.Printf("  %s -> pr/%d\n", alias.Name, alias.PrNumber)
	}
	return nil
}
