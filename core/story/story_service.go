package story

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cupcicm/opp/core"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const storyPattern = `\w+[-_]\d+`

var (
	storyPatternWithBrackets = fmt.Sprintf(`\[%s\]`, storyPattern)
)

type StoryService struct {
	re             *regexp.Regexp
	reWithBrackets *regexp.Regexp
	storyFetcher   StoryFetcher
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
		storyFetcher:   getStoryFetcher(),
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

	story, found = s.findStory(ctx, commitMessages)
	if found {
		return story, strings.Join([]string{s.formatStoryInPRTitle(story), rawTitle}, " ")
	}

	return "", rawTitle
}

func (s *StoryService) findStory(ctx context.Context, commitMessages []string) (story string, found bool) {
	story, found = s.extractFromCommitMessages(commitMessages)
	if found {
		return story, true
	}

	story, found = s.fetchStory(ctx)
	if found {
		return story, true
	}

	return "", false
}

func (s *StoryService) fetchStory(ctx context.Context) (story string, found bool) {
	stories, err := s.storyFetcher.FetchInProgressStories(ctx)
	if err != nil {
		fmt.Printf("could not fetch In Progress Stories: %s\n", err.Error())
		return "", false
	}

	if len(stories) == 0 {
		fmt.Println("there is no In Progress Story")
		return "", false
	}

	story, err = s.selectStory(stories)
	if err != nil {
		fmt.Printf("could not select the story: %s\n", err.Error())
		return "", false
	}

	return story, true
}

func (s *StoryService) selectStory(stories []Story) (selectedStory string, err error) {
	fmt.Println("In Progress stories assigned to Me:")

	for idx, story := range stories {
		fmt.Printf("%d - [%s] %s\n", idx, story.identifier, story.title)
	}

	fmt.Println("")

	fmt.Println("Choose index: ")

	var choosenIndex string
	// Taking input from user
	fmt.Scanln(&choosenIndex)

	index, err := strconv.Atoi(choosenIndex)
	if err != nil {
		fmt.Println("Your input could not be converted to an integer. Aborting")
		return "", fmt.Errorf("could not select Story: the input could not be converted to integer: %w", err)
	}

	if index < 0 || index > len(stories)-1 {
		return "", fmt.Errorf("could not select Story: the input is out from the story range: %w", err)
	}

	return stories[index].identifier, nil
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

func (s *StoryService) enrichBody(rawBody, story string) (string, error) {
	if story == "" || !core.BodyEnrichmentEnabled() {
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
	storyTool, err := getStoryTool()
	if err != nil {
		return "", err
	}

	baseUrl := core.GetStoryToolBaseUrl()
	url := fmt.Sprintf(storyTool.urlTemplate, baseUrl, story)

	return fmt.Sprintf("%s [%s](%s)", cases.Title(language.Und).String(storyTool.tool), story, url), nil
}
