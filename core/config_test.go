package core_test

import (
	"testing"

	"github.com/cupcicm/opp/core"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func cleanupViper(t *testing.T) {
	t.Cleanup(func() {
		viper.Set("github.token", "")
		viper.Set("github.token-cmd", "")
	})
}

func TestGetGithubToken_StaticToken(t *testing.T) {
	cleanupViper(t)
	viper.Set("github.token", "ghp_static123")

	assert.Equal(t, "ghp_static123", core.GetGithubToken())
	assert.True(t, core.HasGithubTokenConfigured())
}

func TestGetGithubToken_TokenCmd(t *testing.T) {
	cleanupViper(t)
	viper.Set("github.token-cmd", "echo ghp_from_cmd")

	assert.Equal(t, "ghp_from_cmd", core.GetGithubToken())
	assert.True(t, core.HasGithubTokenConfigured())
}

func TestGetGithubToken_TokenCmd_TrimWhitespace(t *testing.T) {
	cleanupViper(t)
	viper.Set("github.token-cmd", "printf '  ghp_trimmed_token  \\n\\n'")

	assert.Equal(t, "ghp_trimmed_token", core.GetGithubToken())
}

func TestGetGithubToken_PrecedenceOverStaticToken(t *testing.T) {
	cleanupViper(t)
	viper.Set("github.token", "ghp_static")
	viper.Set("github.token-cmd", "echo ghp_cmd_wins")

	assert.Equal(t, "ghp_cmd_wins", core.GetGithubToken())
}

func TestGetGithubToken_PipelineAndEnv(t *testing.T) {
	cleanupViper(t)
	viper.Set("github.token-cmd", "MY_VAR=secret123 && echo $MY_VAR | tr '[:lower:]' '[:upper:]'")

	assert.Equal(t, "SECRET123", core.GetGithubToken())
}

func TestGetGithubToken_CommandFailurePanics(t *testing.T) {
	cleanupViper(t)
	viper.Set("github.token-cmd", "echo 'something went wrong' >&2; exit 1")

	assert.Panics(t, func() {
		core.GetGithubToken()
	})
}

func TestHasGithubTokenConfigured_Empty(t *testing.T) {
	cleanupViper(t)
	assert.False(t, core.HasGithubTokenConfigured())
}
