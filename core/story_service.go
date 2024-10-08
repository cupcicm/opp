package core

import (
	"regexp"
	"strings"
)

const storyPattern = `\[\w+[-_]\d+\]`

func NewStoryService() (*StoryService, error) {
	re, err := regexp.Compile(storyPattern)
	if err != nil {
		return nil, err
	}

	return &StoryService{
		re: re,
	}, nil
}

type StoryService struct {
	re *regexp.Regexp
}

func (s *StoryService) EnrichBodyAndTitle(commitMessages []string, rawTitle, rawBody string) (title, body string) {
	story, title := s.getStoryAndEnrichTitle(commitMessages, rawTitle)
	return title, s.enrichBody(rawBody, story)
}

func (s *StoryService) getStoryAndEnrichTitle(commitMessages []string, rawTitle string) (story, title string) {
	story, found := s.storyFromString(rawTitle)

	if found {
		return story, rawTitle
	}

	story, found = s.extractFromCommitMessages(commitMessages)
	if found {
		return story, strings.Join([]string{story, rawTitle}, " ")
	}

	return "", rawTitle
}

func (s *StoryService) extractFromCommitMessages(messages []string) (string, bool) {
	var found string
	for _, m := range messages {
		found = s.re.FindString(m)
		if found == "" {
			continue
		} else {
			return found, true
		}
	}

	// pattern not found
	return "", false
}

func (s *StoryService) storyFromString(str string) (string, bool) {
	result := s.re.FindString(str)
	return result, result != ""
}

func (s *StoryService) enrichBody(rawBody, _ string) string {
	// TODO(claire.philippe): to implement
	return rawBody
}
