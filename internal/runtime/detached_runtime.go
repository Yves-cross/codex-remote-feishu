package relayruntime

import (
	"os"
	"path/filepath"
)

type runtimeDetachedLaunchOptions struct {
	BinaryPath string
	Args       []string
	Env        []string
	ConfigPath string
	WorkDir    string
	LogPath    string
	Paths      Paths
}

func startRuntimeDetachedProcess(opts runtimeDetachedLaunchOptions) (int, error) {
	if err := os.MkdirAll(opts.Paths.StateDir, 0o755); err != nil {
		return 0, err
	}

	env := append([]string{}, opts.Env...)
	if opts.ConfigPath != "" {
		env = append(env, "CODEX_REMOTE_CONFIG="+opts.ConfigPath)
	}
	if value := xdgEnvForPath(opts.Paths.ConfigDir); value != "" {
		env = append(env, "XDG_CONFIG_HOME="+value)
	}
	if value := xdgEnvForPath(opts.Paths.DataDir); value != "" {
		env = append(env, "XDG_DATA_HOME="+value)
	}
	if value := xdgEnvForPath(opts.Paths.StateDir); value != "" {
		env = append(env, "XDG_STATE_HOME="+value)
	}

	return StartDetachedCommand(DetachedCommandOptions{
		BinaryPath: opts.BinaryPath,
		Args:       opts.Args,
		Env:        env,
		WorkDir:    opts.WorkDir,
		StdoutPath: opts.LogPath,
		StderrPath: opts.LogPath,
	})
}

func xdgEnvForPath(path string) string {
	if path == "" {
		return ""
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == string(filepath.Separator) {
		return ""
	}
	return dir
}
