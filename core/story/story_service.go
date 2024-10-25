package story

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cupcicm/opp/core"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const storyPattern = `\w+[-_]\d+`

var storyPatternWithBrackets = fmt.Sprintf(`\[%s\]`, storyPattern)

type StoryService interface {
	EnrichBodyAndTitle(commitMessages []string, rawTitle, rawBody string) (title, body string, err error)
}

func NewStoryService() (StoryService, error) {
	tool := core.GetStoryTool()
	url := core.GetStoryToolUrl()

	if tool == "" && url == "" {
		return &StoryServiceNoop{}, nil
	}

	if tool == "" || url == "" {
		panic("please fill in all fields for the Story in the config (story.tool and story.url)")
	}

	re, err := regexp.Compile(storyPattern)
	if err != nil {
		panic("storyPattern regexp doesn't compile")
	}

	reWithBrackets, err := regexp.Compile(storyPatternWithBrackets)
	if err != nil {
		panic("storyPatternWithBrackets regexp doesn't compile")
	}

	return &StoryServiceEnabled{
		re:             re,
		reWithBrackets: reWithBrackets,
		tool:           tool,
		url:            url,
	}, nil
}

type StoryServiceNoop struct{}

func (s *StoryServiceNoop) EnrichBodyAndTitle(commitMessages []string, rawTitle, rawBody string) (title, body string, err error) {
	return rawTitle, rawBody, nil
}

type StoryServiceEnabled struct {
	re             *regexp.Regexp
	reWithBrackets *regexp.Regexp
	tool           string
	url            string
}

func (s *StoryServiceEnabled) EnrichBodyAndTitle(commitMessages []string, rawTitle, rawBody string) (title, body string, err error) {
	story, title := s.getStoryAndEnrichTitle(commitMessages, rawTitle)
	body, err = s.enrichBody(rawBody, story)
	if err != nil {
		return "", "", err
	}
	return title, body, nil
}

func (s *StoryServiceEnabled) getStoryAndEnrichTitle(commitMessages []string, rawTitle string) (story, title string) {
	story, found := s.storyFromMessageOrTitle(rawTitle)

	if found {
		return story, rawTitle
	}

	story, found = s.findStory(commitMessages)
	if found {
		return story, strings.Join([]string{s.formatStoryInPRTitle(story), rawTitle}, " ")
	}

	return "", rawTitle
}

func (s *StoryServiceEnabled) findStory(commitMessages []string) (story string, found bool) {
	story, found = s.extractFromCommitMessages(commitMessages)
	if found {
		return story, true
	}

	story, found = s.fetchStory()
	if found {
		return story, true
	}

	return "", false
}

func (s *StoryServiceEnabled) fetchStory() (story string, found bool) {
	// TODO(ClairePhi): implement
	return "", false
}

func (s *StoryServiceEnabled) extractFromCommitMessages(messages []string) (story string, found bool) {
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

func (s *StoryServiceEnabled) storyFromMessageOrTitle(str string) (string, bool) {
	result := s.reWithBrackets.FindString(str)
	return s.sanitizeStory(result), result != ""
}

func (s *StoryServiceEnabled) sanitizeStory(storyBracket string) string {
	return s.re.FindString(storyBracket)
}

func (s *StoryServiceEnabled) formatStoryInPRTitle(story string) string {
	return fmt.Sprintf("[%s]", story)
}

func (s *StoryServiceEnabled) enrichBody(rawBody, story string) (string, error) {
	if story == "" || !core.EnrichPrBodyWithStoryEnabled() {
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

func (s *StoryServiceEnabled) formatBodyInPRTitle(story string) (string, error) {
	url := fmt.Sprintf("%s/%s", s.url, story)

	return fmt.Sprintf("%s [%s](%s)", cases.Title(language.Und).String(s.tool), story, url), nil
}
