package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStoryServiceExtractFromCommitMessages(t *testing.T) {
    s, err := NewStoryService()
    assert.NoError(t, err)

    testCases := []struct{
        name string
        commitMessages []string
        expectedFound bool
        expectedStory string
    }{
        {
            name: "single commit with story",
            commitMessages: []string{"[ABC-345] fix that"},
            expectedFound: true,
            expectedStory: "[ABC-345]",
        },
        {
            name: "single commit no story",
            commitMessages: []string{"fix that"},
            expectedFound: false,
            expectedStory: "",
        },
        {
            name: "several commits with one story",
            commitMessages: []string{"[ABC-345] fix that", "do that"},
            expectedFound: true,
            expectedStory: "[ABC-345]",
        },
        {
            name: "several commits with no story",
            commitMessages: []string{"fix that", "do that"},
            expectedFound: false,
            expectedStory: "",
        },
        {
            name: "several commits with several stories",
            commitMessages: []string{"[ABC-345] fix that", "[DEF-678] do that"},
            expectedFound: true,
            expectedStory: "[ABC-345]",
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T){
            value, found := s.ExtractFromCommitMessages(tc.commitMessages)
            
            if tc.expectedFound {
                assert.True(t, found)
            } else {
                assert.False(t, found)
            }
            assert.Equal(t, tc.expectedStory, value)
        })
    }
}

func TestStoryServiceStoryInString(t *testing.T) {
    s, err := NewStoryService()
    assert.NoError(t, err)

    testCases := []struct{
        name string
        message string
        expectedFound bool
    }{
        {
            name: "at the beginning of the commit message",
            message: "[ABC-345] fix that",
            expectedFound: true,
        },
        {
            name: "in the middle of the commit message",
            message: "fix [ABC-345] that",
            expectedFound: true,
        },
        {
            name: "at the end of the commit message",
            message: "fix that [ABC-345]",
            expectedFound: true,
        },
        {
            name: "not in the commit message",
            message: "fix that",
            expectedFound: false,
        },
        {
            name: "twice in the commit message",
            message: "[ABC-345] fix that [DEF-678]",
            expectedFound: true,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T){
            found := s.StoryInString(tc.message)
            
            if tc.expectedFound {
                assert.True(t, found)
            } else {
                assert.False(t, found)
            }
        })
    }
}