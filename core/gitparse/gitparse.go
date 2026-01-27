package gitparse

import (
	"fmt"
	"strings"
)

// Commit represents a parsed git commit
type Commit struct {
	Hash      string
	Message   string
	ParentIDs []string
}

// ParseCommit parses output from "git log --format='%H%n%P%n%s' -1 <hash>"
// Expected format:
// <full-hash>
// <parent-hashes-space-separated>
// <subject>
func ParseCommit(output string) (*Commit, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("invalid commit format: expected at least 3 lines, got %d", len(lines))
	}

	hash := strings.TrimSpace(lines[0])
	if hash == "" {
		return nil, fmt.Errorf("empty commit hash")
	}

	parents := strings.Fields(lines[1]) // Space-separated parent hashes

	// Subject is on line 3, but there might be a blank line for commits with no parents
	subject := ""
	if len(lines) >= 3 {
		subject = strings.TrimSpace(lines[2])
	}

	return &Commit{
		Hash:      hash,
		Message:   subject,
		ParentIDs: parents,
	}, nil
}

// ParseCommits parses multiple commits from git log output
func ParseCommits(output string) ([]*Commit, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return []*Commit{}, nil
	}

	// Split by the separator we'll use
	commitBlocks := strings.Split(output, "\n--COMMIT--\n")
	commits := make([]*Commit, 0, len(commitBlocks))

	for _, block := range commitBlocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		commit, err := ParseCommit(block)
		if err != nil {
			return nil, fmt.Errorf("failed to parse commit block: %w", err)
		}
		commits = append(commits, commit)
	}

	return commits, nil
}
