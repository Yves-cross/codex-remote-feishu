package relayruntime

import (
	"os"
	"os/exec"
	"path/filepath"
)

type LaunchOptions struct {
	BinaryPath string
	ConfigPath string
	Env        []string
	Paths      Paths
}

func StartDetachedDaemon(opts LaunchOptions) (int, error) {
	if err := os.MkdirAll(opts.Paths.LogsDir, 0o755); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(opts.Paths.StateDir, 0o755); err != nil {
		return 0, err
	}

	logFile, err := os.OpenFile(opts.Paths.DaemonLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	binaryPath := opts.BinaryPath
	if binaryPath == "" {
		return 0, os.ErrNotExist
	}
	binaryPath = filepath.Clean(binaryPath)
	cmd := exec.Command(binaryPath, "daemon")
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append([]string{}, opts.Env...)
	if opts.ConfigPath != "" {
		cmd.Env = append(cmd.Env, "CODEX_REMOTE_CONFIG="+opts.ConfigPath)
	}
	prepareDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}
