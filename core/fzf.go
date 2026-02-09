package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/buger/goterm"
)

// ChooseFromFzf displays an interactive fuzzy finder for selecting from options
// Returns the selected option(s) or an error if selection fails
func ChooseFromFzf(options []string, multi bool, prompt string, defaultQuery ...string) ([]string, error) {
	if len(options) == 0 {
		return nil, errors.New("no options to choose from")
	}

	// Check if fzf is available
	if _, err := exec.LookPath("fzf"); err != nil {
		return nil, errors.New("fzf not found - install with `brew install fzf`")
	}

	// Write options to a temporary file
	tmpFile, err := os.CreateTemp("", "opp-fzf-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	for _, opt := range options {
		fmt.Fprintln(tmpFile, opt)
	}
	tmpFile.Close()

	height := calculateFzfHeight(len(options))
	prompt = strings.TrimSpace(prompt) + " "

	// Build fzf arguments - read from the temp file
	// Quote arguments that may contain spaces
	args := []string{
		fmt.Sprintf("--prompt='%s'", prompt),
		"--no-info",
		fmt.Sprintf("--height=%d", height),
	}

	if multi {
		args = append(args, "--multi")
	}

	if len(defaultQuery) > 0 && defaultQuery[0] != "" {
		args = append(args, fmt.Sprintf("--query='%s'", defaultQuery[0]))
	}

	// Use shell to cat the file and pipe to fzf
	// This keeps stdin available for fzf's interactive input
	shellCmd := fmt.Sprintf("cat %s | fzf %s", tmpFile.Name(), strings.Join(args, " "))

	cmd := exec.Command("bash", "-c", shellCmd)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin // Keep stdin for interactive input

	var out bytes.Buffer
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		// User cancelled (Ctrl+C or Esc) - exit code 130 or 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 130 || exitErr.ExitCode() == 1 {
				return nil, errors.New("selection cancelled")
			}
		}
		return nil, fmt.Errorf("fzf error: %w", err)
	}

	selected := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(selected) == 0 || (len(selected) == 1 && selected[0] == "") {
		return nil, errors.New("no selection made")
	}

	// Pretty print the selection
	fmt.Printf("%s\n", goterm.Color(goterm.Bold(prompt)+":", goterm.CYAN))
	fmt.Printf(" ➤ %v\n", goterm.Color(goterm.Bold(strings.Join(selected, ", ")), goterm.CYAN))

	return selected, nil
}

// calculateFzfHeight determines the appropriate height for the fzf window
func calculateFzfHeight(numItems int) int {
	const additionalLines = 2
	const maxHeight = 10

	totalLines := numItems + additionalLines
	if totalLines > maxHeight {
		return maxHeight
	}
	return totalLines
}
