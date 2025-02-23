package story

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/cupcicm/opp/core"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const storyPattern = `\w+[-_]\d+`

var storyPatternWithBrackets = fmt.Sprintf(`\[%s\]`, storyPattern)

type StoryService interface {
	EnrichBodyAndTitle(ctx context.Context, commitMessages []string, rawTitle, rawBody string) (title, body string, err error)
}

func NewStoryService(storyFetcher func(string, string) StoryFetcher, in io.Reader) StoryService {
	tool := core.GetStoryTool()
	url := core.GetStoryToolUrl()
	token := core.GetStoryToolToken()

	if tool == "" && url == "" && token == "" {
		return &StoryServiceNoop{}
	}

	if tool == "" || url == "" || token == "" {
		panic("please fill in all Story fields in the config (story.tool, story.url and story.token)")
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
		storyFetcher:   storyFetcher(tool, token),
		in:             in,
	}
}

type StoryServiceNoop struct{}

func (s *StoryServiceNoop) EnrichBodyAndTitle(_ context.Context, _ []string, rawTitle, rawBody string) (title, body string, err error) {
	return rawTitle, rawBody, nil
}

type StoryServiceEnabled struct {
	re             *regexp.Regexp
	reWithBrackets *regexp.Regexp
	tool           string
	url            string
	storyFetcher   StoryFetcher
	in             io.Reader
}

func (s *StoryServiceEnabled) EnrichBodyAndTitle(ctx context.Context, commitMessages []string, rawTitle, rawBody string) (title, body string, err error) {
	story, title := s.getStoryAndEnrichTitle(ctx, s.in, commitMessages, rawTitle)
	body, err = s.enrichBody(rawBody, story)
	if err != nil {
		return "", "", err
	}
	return title, body, nil
}

func (s *StoryServiceEnabled) getStoryAndEnrichTitle(ctx context.Context, in io.Reader, commitMessages []string, rawTitle string) (story, title string) {
	story, found := s.storyFromMessageOrTitle(rawTitle)

	if found {
		return story, rawTitle
	}

	story, found = s.findStory(ctx, in, commitMessages)
	if found {
		return story, strings.Join([]string{s.formatStoryInPRTitle(story), rawTitle}, " ")
	}

	return "", rawTitle
}

func (s *StoryServiceEnabled) findStory(ctx context.Context, in io.Reader, commitMessages []string) (story string, found bool) {
	story, found = s.extractFromCommitMessages(commitMessages)
	if found {
		return story, true
	}

	story, found = s.fetchStory(ctx, in)
	if found {
		return story, true
	}

	return "", false
}

func (s *StoryServiceEnabled) fetchStory(ctx context.Context, in io.Reader) (story string, found bool) {
	stories, err := s.storyFetcher.FetchInProgressStories(ctx)
	if err != nil {
		fmt.Printf("could not fetch In Progress Stories: %s\n", err.Error())
		return "", false
	}

	if len(stories) == 0 {
		return "", false
	}

	story, err = s.selectStory(in, stories)
	if err != nil {
		fmt.Printf("could not select the Story: %s\n", err.Error())
		return "", false
	}

	return story, true
}

func (s *StoryServiceEnabled) selectStory(in io.Reader, stories []Story) (selectedStory string, err error) {
	fmt.Println("In Progress stories assigned to me:")

	for idx, story := range stories {
		fmt.Printf("%d - [%s] %s\n", idx+1, story.Identifier, story.Title)
	}

	fmt.Println("")

	fmt.Println("Choose index: ")

	// Taking input from user
	reader := bufio.NewReader(in)
	chosenIndex, err := reader.ReadString('\n')
	if err != nil {
		return "", errors.New("the input could not be read")
	}

	chosenIndex = strings.TrimSuffix(chosenIndex, "\n")

	index, err := strconv.Atoi(chosenIndex)
	if err != nil {
		return "", errors.New("the input could not be converted to integer")
	}
	index -= 1

	if index < 0 || index > len(stories)-1 {
		return "", errors.New("the input is out from the story range")
	}

	return stories[index].Identifier, nil
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
