package editor

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type VSCodeSettingsStatus struct {
	Path          string `json:"path"`
	Exists        bool   `json:"exists"`
	CLIExecutable string `json:"cliExecutable,omitempty"`
	MatchesBinary bool   `json:"matchesBinary"`
}

type ManagedShimStatus struct {
	Entrypoint       string `json:"entrypoint"`
	Exists           bool   `json:"exists"`
	RealBinaryPath   string `json:"realBinaryPath,omitempty"`
	RealBinaryExists bool   `json:"realBinaryExists"`
	Installed        bool   `json:"installed"`
	MatchesBinary    bool   `json:"matchesBinary"`
}

func DetectVSCodeSettings(settingsPath, executable string) (VSCodeSettingsStatus, error) {
	status := VSCodeSettingsStatus{Path: settingsPath}
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return status, err
	}
	status.Exists = true

	settings, err := decodeVSCodeSettings(raw)
	if err != nil {
		return status, err
	}
	value, _ := settings["chatgpt.cliExecutable"].(string)
	status.CLIExecutable = strings.TrimSpace(value)
	status.MatchesBinary = sameCleanPath(status.CLIExecutable, executable)
	return status, nil
}

func DetectManagedShim(entrypointPath, wrapperBinary string) (ManagedShimStatus, error) {
	status := ManagedShimStatus{
		Entrypoint:     entrypointPath,
		RealBinaryPath: ManagedShimRealBinaryPath(entrypointPath),
	}
	if strings.TrimSpace(entrypointPath) == "" {
		return status, nil
	}

	if _, err := os.Stat(entrypointPath); err == nil {
		status.Exists = true
	} else if !os.IsNotExist(err) {
		return status, err
	}
	if _, err := os.Stat(status.RealBinaryPath); err == nil {
		status.RealBinaryExists = true
	} else if !os.IsNotExist(err) {
		return status, err
	}
	status.Installed = status.Exists && status.RealBinaryExists
	if status.Exists && strings.TrimSpace(wrapperBinary) != "" {
		matches, err := sameFileContents(entrypointPath, wrapperBinary)
		if err != nil {
			return status, err
		}
		status.MatchesBinary = matches
	}
	return status, nil
}

func sameCleanPath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func sameFileContents(leftPath, rightPath string) (bool, error) {
	left, err := fileDigest(leftPath)
	if err != nil {
		return false, err
	}
	right, err := fileDigest(rightPath)
	if err != nil {
		return false, err
	}
	return left == right, nil
}

func fileDigest(path string) ([32]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return [32]byte{}, err
	}
	var digest [32]byte
	copy(digest[:], hasher.Sum(nil))
	return digest, nil
}
