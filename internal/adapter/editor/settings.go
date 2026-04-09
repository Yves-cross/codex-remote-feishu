package editor

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func PatchVSCodeSettings(settingsPath string, executable string) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}

	settings := map[string]any{}
	if raw, err := os.ReadFile(settingsPath); err == nil && len(raw) > 0 {
		settings, err = decodeVSCodeSettings(raw)
		if err != nil {
			return err
		}
	}
	settings["chatgpt.cliExecutable"] = executable

	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(settingsPath, encoded, 0o644)
}

func ClearVSCodeSettingsExecutable(settingsPath string) error {
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	settings, err := decodeVSCodeSettings(raw)
	if err != nil {
		return err
	}
	delete(settings, "chatgpt.cliExecutable")

	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(settingsPath, encoded, 0o644)
}
