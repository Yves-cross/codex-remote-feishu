package relayruntime

import (
	"os"
	"os/exec"
	"path/filepath"
)

type HeadlessLaunchOptions struct {
	BinaryPath string
	ConfigPath string
	Env        []string
	Paths      Paths
	WorkDir    string
	InstanceID string
	Args       []string
}

func StartDetachedWrapper(opts HeadlessLaunchOptions) (int, error) {
	if err := os.MkdirAll(opts.Paths.LogsDir, 0o755); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(opts.Paths.StateDir, 0o755); err != nil {
		return 0, err
	}

	logPath := filepath.Join(opts.Paths.LogsDir, "codex-remote-headless-"+sanitizeFilename(opts.InstanceID)+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	binaryPath := filepath.Clean(opts.BinaryPath)
	if binaryPath == "" {
		return 0, os.ErrNotExist
	}
	args := append([]string{"app-server"}, opts.Args...)
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append([]string{}, opts.Env...)
	if opts.ConfigPath != "" {
		cmd.Env = append(cmd.Env, "CODEX_REMOTE_CONFIG="+opts.ConfigPath)
	}
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	prepareDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func sanitizeFilename(value string) string {
	if value == "" {
		return "unknown"
	}
	out := make([]rune, 0, len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r)
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '-', r == '_', r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
