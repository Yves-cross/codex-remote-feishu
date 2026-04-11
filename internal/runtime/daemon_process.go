package relayruntime

type LaunchOptions struct {
	BinaryPath string
	ConfigPath string
	Env        []string
	Paths      Paths
}

func StartDetachedDaemon(opts LaunchOptions) (int, error) {
	return startRuntimeDetachedProcess(runtimeDetachedLaunchOptions{
		BinaryPath: opts.BinaryPath,
		Args:       []string{"daemon"},
		Env:        opts.Env,
		ConfigPath: opts.ConfigPath,
		LogPath:    opts.Paths.DaemonLogFile,
		Paths:      opts.Paths,
	})
}
