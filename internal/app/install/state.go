package install

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func LoadState(path string) (InstallState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return InstallState{}, err
	}
	var state InstallState
	if err := json.Unmarshal(raw, &state); err != nil {
		return InstallState{}, err
	}
	return state, nil
}

func WriteState(path string, state InstallState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
