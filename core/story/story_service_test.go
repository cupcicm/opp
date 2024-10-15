package story

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStoryRegex(t *testing.T) {
	re, err := regexp.Compile(storyPattern)
	assert.Nil(t, err)

	testCases := []struct {
		name          string
		input         string
		expectedMatch bool
	}{
		{
			name:          "upper case",
			input:         "ABC-123",
			expectedMatch: true,
		},
		{
			name:          "lower case",
			input:         "abc-123",
			expectedMatch: true,
		},
		{
			name:          "mixed case",
			input:         "aBc-123",
			expectedMatch: true,
		},
		{
			name:          "single letter single digit",
			input:         "A-1",
			expectedMatch: true,
		},
		{
			name:          "high length",
			input:         "ABCDEFGHIJKLMOP-1234567890",
			expectedMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedMatch, re.MatchString(tc.input))
		})
	}
}

func TestStoryWihBracketRegex(t *testing.T) {
	re, err := regexp.Compile(storyPatternWithBrackets)
	assert.Nil(t, err)

	testCases := []struct {
		name          string
		input         string
		expectedMatch bool
	}{
		{
			name:          "upper case",
			input:         "[ABC-123]",
			expectedMatch: true,
		},
		{
			name:          "lower case",
			input:         "[abc-123]",
			expectedMatch: true,
		},
		{
			name:          "mixed case",
			input:         "[aBc-123]",
			expectedMatch: true,
		},
		{
			name:          "single letter single digit",
			input:         "[A-1]",
			expectedMatch: true,
		},
		{
			name:          "high length",
			input:         "[ABCDEFGHIJKLMOP-1234567890]",
			expectedMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedMatch, re.MatchString(tc.input))
		})
	}
}
