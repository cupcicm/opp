package core

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
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
	refName := fmt.Sprintf("refs/heads/%s", name)
	cmd := r.GitExec(context.Background(), "symbolic-ref %s 2>/dev/null", refName)
	if output, err := cmd.Output(); err == nil && strings.TrimSpace(string(output)) == "" {
		// Reference exists but is not a symbolic ref - it's a regular branch
		return fmt.Errorf("%w: a branch named '%s' already exists", ErrInvalidAliasName, name)
	}

	// Also check if it's a regular ref (not symbolic)
	cmd = r.GitExec(context.Background(), "show-ref --verify --quiet %s", refName)
	if cmd.Run() == nil {
		// Ref exists - check if it's symbolic
		cmd = r.GitExec(context.Background(), "symbolic-ref %s", refName)
		if _, err := cmd.Output(); err != nil {
			// It's a regular branch, not a symbolic ref
			return fmt.Errorf("%w: a branch named '%s' already exists", ErrInvalidAliasName, name)
		}
	}

	// Check that the target PR branch exists
	targetBranch := LocalBranchForPr(prNumber)
	targetRefName := fmt.Sprintf("refs/heads/%s", targetBranch)
	cmd = r.GitExec(context.Background(), "show-ref --verify --quiet %s", targetRefName)
	if cmd.Run() != nil {
		return fmt.Errorf("PR branch %s does not exist locally", targetBranch)
	}

	// Create symbolic reference
	cmd = r.GitExec(context.Background(), "symbolic-ref %s %s", refName, targetRefName)
	return cmd.Run()
}

// DeleteAlias removes a git symbolic-ref alias
func (r *Repo) DeleteAlias(name string) error {
	refName := fmt.Sprintf("refs/heads/%s", name)

	// Check if ref exists
	cmd := r.GitExec(context.Background(), "show-ref --verify --quiet %s", refName)
	if cmd.Run() != nil {
		return fmt.Errorf("%w: %s", ErrAliasNotFound, name)
	}

	// Check if it's a symbolic reference
	cmd = r.GitExec(context.Background(), "symbolic-ref %s", refName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s is not an alias (it's a regular branch)", name)
	}

	// Verify it points to a PR branch
	target := strings.TrimSpace(string(output))
	targetShort := strings.TrimPrefix(target, "refs/heads/")
	if _, err := ExtractPrNumber(targetShort); err != nil {
		return fmt.Errorf("%s is not an alias to a PR branch", name)
	}

	// Delete the symbolic reference
	cmd = r.GitExec(context.Background(), "update-ref -d %s", refName)
	return cmd.Run()
}

// ResolveAlias checks if a name is a symbolic-ref pointing to a PR branch
// Returns the PR number and true if it's a valid alias, 0 and false otherwise
// Returns false if the target branch no longer exists (orphaned alias)
func (r *Repo) ResolveAlias(name string) (int, bool) {
	refName := fmt.Sprintf("refs/heads/%s", name)

	// Check if it's a symbolic reference
	cmd := r.GitExec(context.Background(), "symbolic-ref %s", refName)
	output, err := cmd.Output()
	if err != nil {
		return 0, false
	}

	target := strings.TrimSpace(string(output))

	// Verify target branch still exists (handle orphaned refs)
	cmd = r.GitExec(context.Background(), "show-ref --verify --quiet %s", target)
	if cmd.Run() != nil {
		return 0, false // Target branch deleted - orphaned alias
	}

	targetShort := strings.TrimPrefix(target, "refs/heads/")
	prNumber, err := ExtractPrNumber(targetShort)
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
	// List all refs - use for-each-ref with proper format escaping
	cmd := r.GitExec(context.Background(), "for-each-ref '--format=%%(refname)' refs/heads/")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list references: %w", err)
	}

	var aliases []Alias
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}

		refName := line

		// Check if it's a symbolic ref
		cmd = r.GitExec(context.Background(), "symbolic-ref %s", refName)
		targetOutput, err := cmd.Output()
		if err != nil {
			continue // Not a symbolic ref
		}

		target := strings.TrimSpace(string(targetOutput))
		if target == "" {
			continue
		}

		// Verify target branch still exists (skip orphaned refs)
		cmd = r.GitExec(context.Background(), "show-ref --verify --quiet %s", target)
		if cmd.Run() != nil {
			continue // Target branch deleted - skip this orphaned alias
		}

		targetShort := strings.TrimPrefix(target, "refs/heads/")
		prNumber, err := ExtractPrNumber(targetShort)
		if err != nil {
			continue // Not pointing to a PR branch
		}

		aliasName := strings.TrimPrefix(refName, "refs/heads/")
		aliases = append(aliases, Alias{
			Name:     aliasName,
			PrNumber: prNumber,
		})
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
