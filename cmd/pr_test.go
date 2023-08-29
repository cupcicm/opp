package cmd_test

import (
	"testing"

	"github.com/cupcicm/opp/core/tests"
	"github.com/stretchr/testify/assert"
)

func TestCanCreatePR(t *testing.T) {
	r := tests.NewTestRepo(t)

	localPr := r.CreatePr(t, "HEAD^", 2)

	assert.True(t, localPr.HasState)
	ancestor, err := localPr.GetAncestor()
	if assert.Nil(t, err) {
		assert.Equal(t, "master", ancestor.LocalName())
	}
}
