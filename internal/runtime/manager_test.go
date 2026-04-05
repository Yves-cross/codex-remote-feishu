package relayruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func TestAcquireLockClearsStaleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.lock")
	if err := os.WriteFile(path, []byte(`{"pid":999999,"token":"stale","createdAt":"2026-04-05T00:00:00Z"}`), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	lock, err := AcquireLock(context.Background(), path, false)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock.Release()
}

func TestAcquireLockFailsWhileLiveOwnerHoldsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.lock")
	first, err := AcquireLock(context.Background(), path, false)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer first.Release()

	second, err := AcquireLock(context.Background(), path, false)
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("expected ErrLockHeld, got lock=%v err=%v", second, err)
	}
}

func TestManagerEnsureReadySkipsStartWhenRelayIsCompatible(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Identity: agentproto.BinaryIdentity{
			Product:          ProductName,
			Version:          "1.0.0",
			BuildFingerprint: "fp-1",
		},
		Paths: testPaths(t),
	})
	started := false
	stopped := false
	manager.probeFunc = func(context.Context) ProbeResult {
		return ProbeResult{
			Status: ProbeCompatible,
			Welcome: agentproto.Welcome{
				Protocol: agentproto.WireProtocol,
				Server: &agentproto.ServerIdentity{
					BinaryIdentity: agentproto.BinaryIdentity{
						Product:          ProductName,
						Version:          "1.0.0",
						BuildFingerprint: "fp-1",
					},
					PID: 111,
				},
			},
		}
	}
	manager.startFunc = func(context.Context) (int, error) {
		started = true
		return 0, nil
	}
	manager.stopFunc = func(context.Context, int) error {
		stopped = true
		return nil
	}

	if err := manager.EnsureReady(context.Background()); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if started || stopped {
		t.Fatalf("expected no start/stop, got started=%v stopped=%v", started, stopped)
	}
}

func TestManagerEnsureReadyStartsDaemonWhenRelayIsUnavailable(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Identity: agentproto.BinaryIdentity{
			Product:          ProductName,
			Version:          "1.0.0",
			BuildFingerprint: "fp-1",
		},
		Paths: testPaths(t),
	})
	sequence := []ProbeResult{
		{Status: ProbeUnreachable, Err: errors.New("dial tcp 127.0.0.1:9500: connection refused")},
		{Status: ProbeUnreachable, Err: errors.New("dial tcp 127.0.0.1:9500: connection refused")},
		{
			Status: ProbeCompatible,
			Welcome: agentproto.Welcome{
				Protocol: agentproto.WireProtocol,
				Server: &agentproto.ServerIdentity{
					BinaryIdentity: agentproto.BinaryIdentity{
						Product:          ProductName,
						Version:          "1.0.0",
						BuildFingerprint: "fp-1",
					},
					PID: 222,
				},
			},
		},
	}
	index := 0
	manager.probeFunc = func(context.Context) ProbeResult {
		result := sequence[index]
		if index < len(sequence)-1 {
			index++
		}
		return result
	}
	starts := 0
	manager.startFunc = func(context.Context) (int, error) {
		starts++
		return 222, nil
	}
	manager.stopFunc = func(context.Context, int) error {
		t.Fatal("unexpected stop")
		return nil
	}

	if err := manager.EnsureReady(context.Background()); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if starts != 1 {
		t.Fatalf("expected one daemon start, got %d", starts)
	}
}

func TestManagerEnsureReadyReplacesIncompatibleDaemonDuringBootstrap(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Identity: agentproto.BinaryIdentity{
			Product:          ProductName,
			Version:          "2.0.0",
			BuildFingerprint: "fp-new",
		},
		Paths: testPaths(t),
	})
	sequence := []ProbeResult{
		{
			Status: ProbeIncompatible,
			Welcome: agentproto.Welcome{
				Protocol: agentproto.WireProtocol,
				Server: &agentproto.ServerIdentity{
					BinaryIdentity: agentproto.BinaryIdentity{
						Product:          ProductName,
						Version:          "1.0.0",
						BuildFingerprint: "fp-old",
					},
					PID: 333,
				},
			},
		},
		{
			Status: ProbeIncompatible,
			Welcome: agentproto.Welcome{
				Protocol: agentproto.WireProtocol,
				Server: &agentproto.ServerIdentity{
					BinaryIdentity: agentproto.BinaryIdentity{
						Product:          ProductName,
						Version:          "1.0.0",
						BuildFingerprint: "fp-old",
					},
					PID: 333,
				},
			},
		},
		{
			Status: ProbeCompatible,
			Welcome: agentproto.Welcome{
				Protocol: agentproto.WireProtocol,
				Server: &agentproto.ServerIdentity{
					BinaryIdentity: agentproto.BinaryIdentity{
						Product:          ProductName,
						Version:          "2.0.0",
						BuildFingerprint: "fp-new",
					},
					PID: 444,
				},
			},
		},
	}
	index := 0
	manager.probeFunc = func(context.Context) ProbeResult {
		result := sequence[index]
		if index < len(sequence)-1 {
			index++
		}
		return result
	}
	var stoppedPID int
	var starts int
	manager.stopFunc = func(_ context.Context, pid int) error {
		stoppedPID = pid
		return nil
	}
	manager.startFunc = func(context.Context) (int, error) {
		starts++
		return 444, nil
	}

	if err := manager.EnsureReady(context.Background()); err != nil {
		t.Fatalf("EnsureReady: %v", err)
	}
	if stoppedPID != 333 {
		t.Fatalf("expected incompatible daemon pid 333 to be stopped, got %d", stoppedPID)
	}
	if starts != 1 {
		t.Fatalf("expected one replacement start, got %d", starts)
	}
}

func testPaths(t *testing.T) Paths {
	t.Helper()
	base := t.TempDir()
	return Paths{
		StateDir:        base,
		LogsDir:         filepath.Join(base, "logs"),
		DaemonLogFile:   filepath.Join(base, "logs", "relayd.log"),
		ManagerLockFile: filepath.Join(base, "relay-manager.lock"),
		DaemonLockFile:  filepath.Join(base, "relayd.lock"),
		PIDFile:         filepath.Join(base, "codex-remote-relayd.pid"),
		IdentityFile:    filepath.Join(base, "codex-remote-relayd.identity.json"),
	}
}

func TestManagerWelcomeCompatible(t *testing.T) {
	manager := NewManager(ManagerConfig{
		Identity: agentproto.BinaryIdentity{
			Product:          ProductName,
			Version:          "1.0.0",
			BuildFingerprint: "fp-1",
		},
		Paths: testPaths(t),
	})
	if !manager.WelcomeCompatible(agentproto.Welcome{
		Protocol: agentproto.WireProtocol,
		Server: &agentproto.ServerIdentity{
			BinaryIdentity: agentproto.BinaryIdentity{
				Product:          ProductName,
				Version:          "1.0.0",
				BuildFingerprint: "fp-1",
			},
		},
	}) {
		t.Fatal("expected welcome to be compatible")
	}
	if manager.WelcomeCompatible(agentproto.Welcome{
		Protocol: agentproto.WireProtocol,
		Server: &agentproto.ServerIdentity{
			BinaryIdentity: agentproto.BinaryIdentity{
				Product:          ProductName,
				Version:          "1.0.1",
				BuildFingerprint: "fp-2",
			},
			StartedAt: time.Now(),
		},
	}) {
		t.Fatal("expected welcome to be incompatible")
	}
}
