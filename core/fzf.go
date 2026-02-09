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

	height := calculateFzfHeight(len(options))
	prompt = strings.TrimSpace(prompt) + " "

	cmd := exec.Command("fzf",
		"--prompt="+prompt,
		"--no-info",
		fmt.Sprintf("--height=%d", height),
	)

	if multi {
		cmd.Args = append(cmd.Args, "--multi")
	}

	if len(defaultQuery) > 0 && defaultQuery[0] != "" {
		cmd.Args = append(cmd.Args, "--query="+defaultQuery[0])
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		defer stdin.Close()
		for _, opt := range options {
			fmt.Fprintln(stdin, opt)
		}
	}()

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return nil, errors.New("fzf not found - install with `brew install fzf`")
		}
		// User cancelled (Ctrl+C or Esc)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return nil, errors.New("selection cancelled")
		}
		return nil, err
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
