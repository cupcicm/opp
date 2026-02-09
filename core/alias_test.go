package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAliasName(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{"valid simple", "myfeature", false},
		{"valid with hyphen", "my-feature", false},
		{"valid with underscore", "my_feature", false},
		{"valid with numbers", "feature123", false},
		{"valid mixed", "my-feature_123", false},

		{"invalid pure number", "123", true},
		{"invalid pr format", "pr/123", true},
		{"invalid starts with number", "123feature", true},
		{"invalid starts with hyphen", "-feature", true},
		{"invalid contains space", "my feature", true},
		{"invalid empty", "", true},
		{"invalid special chars", "my@feature", true},

		// Git reserved names
		{"invalid HEAD", "HEAD", true},
		{"invalid head lowercase", "head", true},
		{"invalid FETCH_HEAD", "FETCH_HEAD", true},
		{"invalid ORIG_HEAD", "ORIG_HEAD", true},
		{"invalid MERGE_HEAD", "MERGE_HEAD", true},
		{"invalid CHERRY_PICK_HEAD", "CHERRY_PICK_HEAD", true},
		{"invalid REBASE_HEAD", "REBASE_HEAD", true},

		// refs/ prefix
		{"invalid refs prefix", "refs/heads/foo", true},
		{"invalid REFS prefix", "REFS/heads/foo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAliasName(tt.alias)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractPrNumberWithAlias(t *testing.T) {
	// Test that ExtractPrNumber works as expected for PR numbers
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"plain number", "123", 123, false},
		{"pr format", "pr/456", 456, false},
		{"invalid format", "feature", 0, true},
		{"invalid pr format", "pr/abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractPrNumber(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
