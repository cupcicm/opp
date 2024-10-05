package core

import (
	"regexp"
)

const storyPattern = `\[\w+[-_]\d+\]`

type StoryService interface {
    ExtractFromCommitMessages(messages []string) (string, bool)
    StoryInString(str string) bool
}

func NewStoryService() (StoryService, error) {
    re, err := regexp.Compile(storyPattern)
    if err != nil {
        return nil, err
    }

    return &storyService{
        re: re,
    }, nil
}

type storyService struct {
    re *regexp.Regexp 
}

func (s *storyService) ExtractFromCommitMessages(messages []string) (string, bool) {
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

func (s *storyService) StoryInString(str string) bool {
    return s.re.MatchString(str)
}