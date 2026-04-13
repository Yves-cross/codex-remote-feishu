//go:build windows

package wrapper

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/windows"
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
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(parentPID))
	if err != nil {
		notifyHostExit(onHostExit)
		return
	}
	defer windows.CloseHandle(handle)

	poll := uint32(hostWatcherPollInterval.Milliseconds())
	if poll == 0 {
		poll = 1
	}
	for {
		waitResult, waitErr := windows.WaitForSingleObject(handle, poll)
		if waitErr != nil {
			return
		}
		switch waitResult {
		case uint32(windows.WAIT_OBJECT_0):
			notifyHostExit(onHostExit)
			return
		case uint32(windows.WAIT_TIMEOUT):
			select {
			case <-ctx.Done():
				return
			default:
			}
		default:
			return
		}
	}
}

func notifyHostExit(onHostExit func()) {
	if onHostExit != nil {
		onHostExit()
	}
}
