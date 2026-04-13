//go:build !windows

package wrapper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

const hostWatcherPollInterval = 500 * time.Millisecond

func startHostLifetimeWatcher(ctx context.Context, cfg Config, onHostExit func()) error {
	if !strings.EqualFold(strings.TrimSpace(cfg.Lifetime), string(lifetimeHostBound)) {
		return nil
	}
	if cfg.ParentPID <= 0 {
		return nil
	}
	if cfg.ParentPID == os.Getpid() {
		return fmt.Errorf("host-bound lifetime requires parent pid different from current pid")
	}

	go watchHostProcessUntilExit(ctx, cfg.ParentPID, onHostExit)
	return nil
}

func watchHostProcessUntilExit(ctx context.Context, parentPID int, onHostExit func()) {
	if !hostProcessAlive(parentPID) {
		notifyHostExit(onHostExit)
		return
	}

	ticker := time.NewTicker(hostWatcherPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !hostProcessAlive(parentPID) {
				notifyHostExit(onHostExit)
				return
			}
		}
	}
}

func hostProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func notifyHostExit(onHostExit func()) {
	if onHostExit != nil {
		onHostExit()
	}
}
