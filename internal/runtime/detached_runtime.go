package relayruntime

import "os"

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

	return StartDetachedCommand(DetachedCommandOptions{
		BinaryPath: opts.BinaryPath,
		Args:       opts.Args,
		Env:        env,
		WorkDir:    opts.WorkDir,
		StdoutPath: opts.LogPath,
		StderrPath: opts.LogPath,
	})
}
