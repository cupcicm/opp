package core

import (
	"context"
	"os"
	"path"
	"strconv"

	"gopkg.in/yaml.v3"
)

type BranchState struct {
	Ancestor struct {
		Name      string
		KnownTips []string
	}
	KnownTips []string
	Tag       string
}

type StateStore struct {
	baseFolder string
}

func NewStateStore(r *Repo) *StateStore {
	return &StateStore{
		baseFolder: path.Join(r.DotOpDir(), "state"),
	}
}

func (s *StateStore) branchStateFile(b Branch) string {
	return path.Join(s.baseFolder, b.LocalName())
}

func (s *StateStore) StateBranchFile(b Branch) string {
	return s.branchStateFile(b)
}

func (s *StateStore) GetBranchState(b Branch) *BranchState {
	var exists = FileExists(s.branchStateFile(b))
	if exists {
		return Must(s.loadBranchState(s.branchStateFile(b)))
	}
	var newState = &BranchState{}
	err := s.SaveBranchState(b, newState)
	if err != nil {
		panic(err)
	}
	return newState
}

func (s *StateStore) DeleteBranchState(b Branch) {
	_ = os.Remove(s.branchStateFile(b))
}

func (s *StateStore) loadBranchState(file string) (*BranchState, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	state := BranchState{}
	err = yaml.Unmarshal(content, &state)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *StateStore) SaveBranchState(b Branch, state *BranchState) error {
	content, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(path.Dir(s.branchStateFile(b)), 0700)
	return os.WriteFile(s.branchStateFile(b), content, 0600)
}

func (s *StateStore) AllLocalPrNumbers(ctx context.Context) []int {
	if !FileExists(path.Join(s.baseFolder, "pr")) {
		return nil
	}
	files := Must(os.ReadDir(path.Join(s.baseFolder, "pr")))
	prs := make([]int, 0, len(files))
	for _, file := range files {
		num, err := strconv.Atoi(file.Name())
		if err != nil {
			continue
		}
		prs = append(prs, num)
	}
	return prs
}
