package core

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoreCreatesFolder(t *testing.T) {

	dir, err := os.MkdirTemp(os.TempDir(), "state_store")
	require.NoError(t, err)
	var s = StateStore{
		baseFolder: dir,
	}
	var pr = &LocalPr{PrNumber: 1234}
	s.GetBranchState(pr)
	require.FileExists(t, path.Join(dir, "pr", "1234"))

	s.DeleteBranchState(pr)
	require.NoFileExists(t, path.Join(dir, "pr", "1234"))
}

func TestStoresCorrectly(t *testing.T) {

	dir, err := os.MkdirTemp(os.TempDir(), "state_store")
	require.NoError(t, err)
	var s = StateStore{
		baseFolder: dir,
	}
	var pr = &LocalPr{PrNumber: 1234}

	var state = &BranchState{}
	state.Ancestor.Name = "main"
	s.SaveBranchState(pr, state)
	require.FileExists(t, path.Join(dir, "pr", "1234"))

	content, err := os.ReadFile(path.Join(dir, "pr", "1234"))
	require.NoError(t, err)
	require.Contains(t, string(content), "name: main")
}
