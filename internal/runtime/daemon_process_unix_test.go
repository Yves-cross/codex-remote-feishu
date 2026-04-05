//go:build !windows

package relayruntime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartDetachedDaemonSurvivesParentExit(t *testing.T) {
	if os.Getenv("GO_WANT_DETACHED_HELPER") == "1" {
		helperRunDetached(t)
		return
	}

	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "child.pid")
	logFile := filepath.Join(tempDir, "relayd.log")
	script := filepath.Join(tempDir, "child.sh")
	scriptBody := "#!/usr/bin/env bash\nset -euo pipefail\necho $$ > \"$PID_FILE\"\ntrap 'exit 0' TERM INT\nwhile true; do sleep 1; done\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0o755); err != nil {
		t.Fatalf("write helper script: %v", err)
	}

	helper := exec.Command(os.Args[0], "-test.run=^TestStartDetachedDaemonSurvivesParentExit$")
	helper.Env = append(os.Environ(),
		"GO_WANT_DETACHED_HELPER=1",
		"DETACHED_HELPER_SCRIPT="+script,
		"DETACHED_HELPER_PID_FILE="+pidFile,
		"DETACHED_HELPER_LOG_FILE="+logFile,
	)
	output, err := helper.CombinedOutput()
	if err != nil {
		t.Fatalf("helper failed: %v\n%s", err, string(output))
	}

	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(pidFile)
		if err == nil {
			rawText := strings.TrimSpace(string(raw))
			if _, scanErr := fmt.Sscanf(rawText, "%d", &pid); scanErr == nil && pid > 0 {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pid <= 0 {
		t.Fatal("expected detached child pid to be recorded")
	}
	if !processAlive(pid) {
		t.Fatalf("expected detached child pid %d to survive parent exit", pid)
	}
	if err := terminateProcess(pid, time.Second); err != nil {
		t.Fatalf("terminate detached child %d: %v", pid, err)
	}
}

func helperRunDetached(t *testing.T) {
	t.Helper()
	script := os.Getenv("DETACHED_HELPER_SCRIPT")
	pidFile := os.Getenv("DETACHED_HELPER_PID_FILE")
	logFile := os.Getenv("DETACHED_HELPER_LOG_FILE")
	if script == "" || pidFile == "" || logFile == "" {
		t.Fatal("missing detached helper env")
	}
	paths := Paths{
		StateDir:      filepath.Dir(logFile),
		LogsDir:       filepath.Dir(logFile),
		DaemonLogFile: logFile,
	}
	_, err := StartDetachedDaemon(LaunchOptions{
		BinaryPath: script,
		Env:        append(os.Environ(), "PID_FILE="+pidFile),
		Paths:      paths,
	})
	if err != nil {
		t.Fatalf("StartDetachedDaemon: %v", err)
	}
}
