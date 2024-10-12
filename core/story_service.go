package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const storyPattern = `\w+[-_]\d+`

var (
	urlPatterns = map[string]string{
		"jira":   "https://%s/browse/%s",
		"linear": "https://%s/issue/%s",
	}
	storyPatternWithBrackets = fmt.Sprintf(`\[%s\]`, storyPattern)
)

type StoryService struct {
	re             *regexp.Regexp
	reWithBrackets *regexp.Regexp
}

func NewStoryService() (*StoryService, error) {
	re, err := regexp.Compile(storyPattern)
	if err != nil {
		return nil, err
	}

	reWithBrackets, err := regexp.Compile(storyPatternWithBrackets)
	if err != nil {
		return nil, err
	}

	return &StoryService{
		re:             re,
		reWithBrackets: reWithBrackets,
	}, nil
}

func (s *StoryService) EnrichBodyAndTitle(ctx context.Context, commitMessages []string, rawTitle, rawBody string) (title, body string, err error) {
	story, title := s.getStoryAndEnrichTitle(ctx, commitMessages, rawTitle)
	body, err = s.enrichBody(rawBody, story)
	if err != nil {
		return "", "", err
	}
	return title, body, nil
}

func (s *StoryService) getStoryAndEnrichTitle(ctx context.Context, commitMessages []string, rawTitle string) (story, title string) {
	story, found := s.storyFromMessageOrTitle(rawTitle)

	if found {
		return story, rawTitle
	}

	story, found = s.getStory(ctx, commitMessages)
	if found {
		return story, strings.Join([]string{s.formatStoryInPRTitle(story), rawTitle}, " ")
	}

	return "", rawTitle
}

func (s *StoryService) getStory(ctx context.Context, messages []string) (story string, found bool) {
	story, found = s.extractFromCommitMessages(messages)
	if found {
		return story, found
	}

	if FetchStoriesEnabled() {
		return s.fetchStory(ctx)
	}

	return "", false
}

func (s *StoryService) extractFromCommitMessages(messages []string) (story string, found bool) {
	for _, m := range messages {
		story, found = s.storyFromMessageOrTitle(m)
		if !found {
			continue
		} else {
			return story, true
		}
	}

	// pattern not found
	return "", false
}

func (s *StoryService) storyFromMessageOrTitle(str string) (string, bool) {
	result := s.reWithBrackets.FindString(str)
	return s.sanitizeStory(result), result != ""
}

func (s *StoryService) sanitizeStory(storyBracket string) string {
	return s.re.FindString(storyBracket)
}

func (s *StoryService) fetchStory(ctx context.Context) (story string, found bool) {
	stories, err := NewLinearStoryFetcher().FetchInProgressStories(ctx)
	if err != nil {
		return "", found
	}
	fmt.Println(stories)
	return "", false
	// if len(stories) == 0 {
	// 	return "", false
	// }
	// story, selected = StorySelector{
	// 	stories: stories,
	// }.Run()
	// return story, selected
}

func (s *StoryService) formatStoryInPRTitle(story string) string {
	return fmt.Sprintf("[%s]", story)
}

func (s *StoryService) enrichBody(rawBody, story string) (string, error) {
	if story == "" || !BodyEnrichmentEnabled() {
		return rawBody, nil
	}

	link, err := s.formatBodyInPRTitle(story)
	if err != nil {
		return "", fmt.Errorf("could not enrich the body with the Story: %w", err)
	}

	if rawBody == "" {
		return fmt.Sprintf("- %s", link), nil
	}

	return strings.Join([]string{fmt.Sprintf("- %s", link), rawBody}, "\n\n"), nil
}

func (s *StoryService) formatBodyInPRTitle(story string) (string, error) {
	tool := GetStoryTool()
	urlTemplate, ok := urlPatterns[tool]
	if !ok {
		availableTools := []string{}
		for availableTool := range urlPatterns {
			availableTools = append(availableTools, availableTool)
		}
		return "", fmt.Errorf("tool set in config (%s) doesn't match possible values (%s)", tool, availableTools)
	}

	baseUrl := GetStoryToolBaseUrl()
	url := fmt.Sprintf(urlTemplate, baseUrl, story)

	return fmt.Sprintf("%s [%s](%s)", cases.Title(language.Und).String(tool), story, url), nil
}
