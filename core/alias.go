package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

var (
	// ErrInvalidAliasName is returned when an alias name is not valid
	ErrInvalidAliasName = errors.New("invalid alias name")
	// ErrAliasNotFound is returned when an alias does not exist
	ErrAliasNotFound = errors.New("alias not found")
	// ErrAliasExists is returned when trying to create an alias that already exists
	ErrAliasExists = errors.New("alias already exists")
)

// validAliasPattern matches valid alias names (alphanumeric, hyphens, underscores)
var validAliasPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// gitReservedNames contains git internal reference names that cannot be used as aliases
var gitReservedNames = map[string]bool{
	"HEAD":             true,
	"FETCH_HEAD":       true,
	"ORIG_HEAD":        true,
	"MERGE_HEAD":       true,
	"CHERRY_PICK_HEAD": true,
	"REBASE_HEAD":      true,
	"BISECT_HEAD":      true,
}

// ValidateAliasName checks if an alias name is valid
func ValidateAliasName(name string) error {
	// Reject pure numbers (would conflict with PR numbers)
	if _, err := ExtractPrNumber(name); err == nil {
		return fmt.Errorf("%w: cannot use a number or pr/number format as alias", ErrInvalidAliasName)
	}

	// Reject names starting with pr/ (would conflict with PR branches)
	if strings.HasPrefix(name, "pr/") {
		return fmt.Errorf("%w: cannot start with 'pr/'", ErrInvalidAliasName)
	}

	// Reject git reserved names (case-insensitive)
	if gitReservedNames[strings.ToUpper(name)] {
		return fmt.Errorf("%w: '%s' is a git reserved name", ErrInvalidAliasName, name)
	}

	// Reject refs/ prefix (would conflict with git internals)
	if strings.HasPrefix(strings.ToLower(name), "refs/") {
		return fmt.Errorf("%w: cannot start with 'refs/'", ErrInvalidAliasName)
	}

	// Check valid pattern
	if !validAliasPattern.MatchString(name) {
		return fmt.Errorf("%w: must start with a letter and contain only alphanumeric, hyphens, or underscores", ErrInvalidAliasName)
	}

	return nil
}

// CreateAlias creates a git symbolic-ref that points to a PR branch
func (r *Repo) CreateAlias(name string, prNumber int) error {
	if err := ValidateAliasName(name); err != nil {
		return err
	}

	// Check if alias already exists
	if _, exists := r.ResolveAlias(name); exists {
		return fmt.Errorf("%w: %s", ErrAliasExists, name)
	}

	// Check if a regular branch with this name already exists
	refName := plumbing.NewBranchReferenceName(name)
	if existingRef, err := r.Reference(refName, false); err == nil {
		// Reference exists - check if it's a symbolic ref or a regular branch
		if existingRef.Type() != plumbing.SymbolicReference {
			return fmt.Errorf("%w: a branch named '%s' already exists", ErrInvalidAliasName, name)
		}
	}

	// Check that the target PR branch exists
	targetBranch := LocalBranchForPr(prNumber)
	targetRefName := plumbing.NewBranchReferenceName(targetBranch)
	if _, err := r.Reference(targetRefName, true); err != nil {
		return fmt.Errorf("PR branch %s does not exist locally", targetBranch)
	}

	// Create symbolic reference
	ref := plumbing.NewSymbolicReference(refName, targetRefName)
	return r.Storer.SetReference(ref)
}

// DeleteAlias removes a git symbolic-ref alias
func (r *Repo) DeleteAlias(name string) error {
	refName := plumbing.NewBranchReferenceName(name)
	ref, err := r.Reference(refName, false)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrAliasNotFound, name)
	}

	// Only delete if it's a symbolic reference pointing to a PR branch
	if ref.Type() != plumbing.SymbolicReference {
		return fmt.Errorf("%s is not an alias (it's a regular branch)", name)
	}

	target := ref.Target().Short()
	if _, err := ExtractPrNumber(target); err != nil {
		return fmt.Errorf("%s is not an alias to a PR branch", name)
	}

	return r.Storer.RemoveReference(refName)
}

// ResolveAlias checks if a name is a symbolic-ref pointing to a PR branch
// Returns the PR number and true if it's a valid alias, 0 and false otherwise
// Returns false if the target branch no longer exists (orphaned alias)
func (r *Repo) ResolveAlias(name string) (int, bool) {
	refName := plumbing.NewBranchReferenceName(name)
	ref, err := r.Reference(refName, false)
	if err != nil {
		return 0, false
	}

	if ref.Type() != plumbing.SymbolicReference {
		return 0, false
	}

	// Verify target branch still exists (handle orphaned refs)
	targetRefName := ref.Target()
	if _, err := r.Reference(targetRefName, true); err != nil {
		return 0, false // Target branch deleted - orphaned alias
	}

	target := ref.Target().Short()
	prNumber, err := ExtractPrNumber(target)
	if err != nil {
		return 0, false
	}

	return prNumber, true
}

// Alias represents a branch alias
type Alias struct {
	Name     string
	PrNumber int
}

// ListAliases returns all symbolic-refs that point to pr/* branches
// Skips orphaned aliases (where target branch no longer exists)
func (r *Repo) ListAliases() ([]Alias, error) {
	refs, err := r.References()
	if err != nil {
		return nil, fmt.Errorf("could not iterate references: %w", err)
	}

	var aliases []Alias
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.SymbolicReference {
			return nil
		}

		if !ref.Name().IsBranch() {
			return nil
		}

		// Verify target branch still exists (skip orphaned refs)
		targetRefName := ref.Target()
		if _, err := r.Reference(targetRefName, true); err != nil {
			return nil // Target branch deleted - skip this orphaned alias
		}

		target := ref.Target().Short()
		prNumber, err := ExtractPrNumber(target)
		if err != nil {
			return nil
		}

		aliases = append(aliases, Alias{
			Name:     ref.Name().Short(),
			PrNumber: prNumber,
		})
		return nil
	})

	if err != nil {
		return nil, err
	}

	return aliases, nil
}

// AliasesForPr returns all aliases pointing to a specific PR
func (r *Repo) AliasesForPr(prNumber int) []string {
	aliases, err := r.ListAliases()
	if err != nil {
		return nil
	}

	var result []string
	for _, alias := range aliases {
		if alias.PrNumber == prNumber {
			result = append(result, alias.Name)
		}
	}
	return result
}

// DeleteAliasesForPr removes all aliases pointing to a specific PR
func (r *Repo) DeleteAliasesForPr(prNumber int) error {
	aliases := r.AliasesForPr(prNumber)
	for _, alias := range aliases {
		if err := r.DeleteAlias(alias); err != nil {
			return err
		}
	}
	return nil
}

