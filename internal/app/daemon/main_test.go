package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	tempRoot, err := os.MkdirTemp("", "codex-remote-daemon-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon TestMain temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempRoot)

	homeDir := filepath.Join(tempRoot, "home")
	configHome := filepath.Join(tempRoot, "xdg-config")
	dataHome := filepath.Join(tempRoot, "xdg-data")
	stateHome := filepath.Join(tempRoot, "xdg-state")
	repoRoot := filepath.Join(tempRoot, "repo")
	for _, dir := range []string{homeDir, configHome, dataHome, stateHome, repoRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "daemon TestMain mkdir %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	setenvOrExit := func(key, value string) {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "daemon TestMain setenv %s: %v\n", key, err)
			os.Exit(1)
		}
	}
	setenvOrExit("HOME", homeDir)
	setenvOrExit("XDG_CONFIG_HOME", configHome)
	setenvOrExit("XDG_DATA_HOME", dataHome)
	setenvOrExit("XDG_STATE_HOME", stateHome)
	setenvOrExit("CODEX_REMOTE_REPO_ROOT", repoRoot)

	os.Exit(m.Run())
}
