package story

import (
	"fmt"

	"github.com/cupcicm/opp/core"
)

var urlPatterns = map[string]string{
	"jira":   "https://%s/browse/%s",
	"linear": "https://%s/issue/%s",
}

type storyTool struct {
	tool        string
	urlTemplate string
}

func getStoryTool() (*storyTool, error) {
	tool := core.GetStoryTool()
	urlTemplate, ok := urlPatterns[tool]
	if !ok {
		availableTools := []string{}
		for availableTool := range urlPatterns {
			availableTools = append(availableTools, availableTool)
		}
		return nil, fmt.Errorf("tool set in config (%s) doesn't match possible values (%s)", tool, availableTools)
	}

	return &storyTool{
		tool:        tool,
		urlTemplate: urlTemplate,
	}, nil
}
