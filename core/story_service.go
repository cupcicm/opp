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

func (s *StoryService) AddToRawTitle(commitMessages []string, rawTitle string) string {
	if s.storyInString(rawTitle) {
		return rawTitle
	}

	story, found := s.extractFromCommitMessages(commitMessages)
	if !found {
		return rawTitle
	}

	return strings.Join([]string{story, rawTitle}, " ")
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

func (s *StoryService) storyInString(str string) bool {
	return s.re.MatchString(str)
}
