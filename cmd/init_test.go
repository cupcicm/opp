package cmd

import (
	"bufio"
	"strings"
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func cleanupInitViper(t *testing.T) {
	t.Cleanup(func() {
		viper.Set("github.token", "")
		viper.Set("github.token-cmd", "")
	})
}

func TestAskGithubToken_TokenInput(t *testing.T) {
	cleanupInitViper(t)
	i := initializer{}
	reader := bufio.NewReader(strings.NewReader("ghp_mytoken123\n"))
	i.askGithubToken(reader)

	assert.Equal(t, "ghp_mytoken123", viper.GetString("github.token"))
	assert.Empty(t, core.GetGithubTokenCmd())
}

func TestAskGithubToken_CommandInputWithExclamation(t *testing.T) {
	cleanupInitViper(t)
	i := initializer{}
	reader := bufio.NewReader(strings.NewReader("!gh auth token\n"))
	i.askGithubToken(reader)

	assert.Equal(t, "gh auth token", viper.GetString("github.token-cmd"))
	assert.Empty(t, viper.GetString("github.token"))
}

func TestAskGithubToken_AlreadyConfiguredSkips(t *testing.T) {
	cleanupInitViper(t)
	viper.Set("github.token-cmd", "echo already-set")
	i := initializer{}
	// Empty reader; if it attempted to read, it would panic or error on core.Must
	reader := bufio.NewReader(strings.NewReader(""))
	i.askGithubToken(reader)

	assert.Equal(t, "echo already-set", core.GetGithubTokenCmd())
}
