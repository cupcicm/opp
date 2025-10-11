package core

import (
	"testing"
)

func TestClipboardWorks(t *testing.T) {
	CopyRichText("hello", "world", "https://wikipedia.org")
	// After this you can paste what's in your buffer and it should work
}
