package issuedocsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func LoadState(path string, repo string) (StateFile, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewState(repo), nil
		}
		return StateFile{}, fmt.Errorf("read state file: %w", err)
	}

	var state StateFile
	if err := json.Unmarshal(payload, &state); err != nil {
		return StateFile{}, fmt.Errorf("decode state file: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Repo == "" {
		state.Repo = repo
	}
	if state.Issues == nil {
		state.Issues = map[string]IssueRecord{}
	}
	return state, nil
}

func SaveState(path string, state StateFile) error {
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Issues == nil {
		state.Issues = map[string]IssueRecord{}
	}

	sorted := sortedIssues(state.Issues)
	payload, err := json.MarshalIndent(struct {
		Version int            `json:"version"`
		Repo    string         `json:"repo"`
		Issues  []sortedRecord `json:"issues"`
	}{Version: state.Version, Repo: state.Repo, Issues: sorted}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state file: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}

func NewState(repo string) StateFile {
	return StateFile{
		Version: 1,
		Repo:    repo,
		Issues:  map[string]IssueRecord{},
	}
}

type sortedRecord struct {
	Key string `json:"key"`
	IssueRecord
}

func sortedIssues(input map[string]IssueRecord) []sortedRecord {
	keys := make([]int, 0, len(input))
	for key := range input {
		number, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		keys = append(keys, number)
	}
	sort.Ints(keys)
	out := make([]sortedRecord, 0, len(keys))
	for _, key := range keys {
		record := input[strconv.Itoa(key)]
		out = append(out, sortedRecord{
			Key:         strconv.Itoa(key),
			IssueRecord: record,
		})
	}
	return out
}
