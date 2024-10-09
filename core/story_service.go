package core

import (
	"fmt"
	"regexp"
	"strings"
)

const storyPattern = `\w+[-_]\d+`

var storyPatternWithBrackets = fmt.Sprintf(`\[%s\]`, storyPattern)

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

func (s *StoryService) EnrichBodyAndTitle(commitMessages []string, rawTitle, rawBody string) (title, body string) {
	story, title := s.getStoryAndEnrichTitle(commitMessages, rawTitle)
	return title, s.enrichBody(rawBody, story)
}

func (s *StoryService) getStoryAndEnrichTitle(commitMessages []string, rawTitle string) (story, title string) {
	story, found := s.storyFromMessageOrTitle(rawTitle)

	if found {
		return story, rawTitle
	}

	story, found = s.extractFromCommitMessages(commitMessages)
	if found {
		return story, strings.Join([]string{s.formatStoryInPRTitle(story), rawTitle}, " ")
	}

	return "", rawTitle
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

func (s *StoryService) formatStoryInPRTitle(story string) string {
	return fmt.Sprintf("[%s]", story)
}

func (s *StoryService) enrichBody(rawBody, _ string) string {
	// TODO(claire.philippe): to implement
	return rawBody
}
