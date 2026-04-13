package wrapper

import "testing"

func TestResolveInstanceLifetimeDefaultsHeadlessManagedToDaemonOwned(t *testing.T) {
	t.Parallel()

	lifetime, parentPID, err := resolveInstanceLifetime("headless", true, "", "", 123)
	if err != nil {
		t.Fatalf("resolveInstanceLifetime() error = %v", err)
	}
	if lifetime != lifetimeDaemonOwned {
		t.Fatalf("lifetime = %q, want %q", lifetime, lifetimeDaemonOwned)
	}
	if parentPID != 0 {
		t.Fatalf("parentPID = %d, want 0", parentPID)
	}
}

func TestResolveInstanceLifetimeDefaultsVSCodeToHostBound(t *testing.T) {
	t.Parallel()

	lifetime, parentPID, err := resolveInstanceLifetime("vscode", false, "", "", 456)
	if err != nil {
		t.Fatalf("resolveInstanceLifetime() error = %v", err)
	}
	if lifetime != lifetimeHostBound {
		t.Fatalf("lifetime = %q, want %q", lifetime, lifetimeHostBound)
	}
	if parentPID != 456 {
		t.Fatalf("parentPID = %d, want 456", parentPID)
	}
}

func TestResolveInstanceLifetimeHonorsExplicitStandalone(t *testing.T) {
	t.Parallel()

	lifetime, parentPID, err := resolveInstanceLifetime("vscode", false, "standalone", "999", 456)
	if err != nil {
		t.Fatalf("resolveInstanceLifetime() error = %v", err)
	}
	if lifetime != lifetimeStandalone {
		t.Fatalf("lifetime = %q, want %q", lifetime, lifetimeStandalone)
	}
	if parentPID != 0 {
		t.Fatalf("parentPID = %d, want 0", parentPID)
	}
}

func TestResolveInstanceLifetimeHonorsExplicitHostBoundParentPID(t *testing.T) {
	t.Parallel()

	lifetime, parentPID, err := resolveInstanceLifetime("vscode", false, "host-bound", "789", 456)
	if err != nil {
		t.Fatalf("resolveInstanceLifetime() error = %v", err)
	}
	if lifetime != lifetimeHostBound {
		t.Fatalf("lifetime = %q, want %q", lifetime, lifetimeHostBound)
	}
	if parentPID != 789 {
		t.Fatalf("parentPID = %d, want 789", parentPID)
	}
}

func TestResolveInstanceLifetimeFallsBackToStandaloneWhenNoParentPID(t *testing.T) {
	t.Parallel()

	lifetime, parentPID, err := resolveInstanceLifetime("vscode", false, "host-bound", "", 0)
	if err != nil {
		t.Fatalf("resolveInstanceLifetime() error = %v", err)
	}
	if lifetime != lifetimeStandalone {
		t.Fatalf("lifetime = %q, want %q", lifetime, lifetimeStandalone)
	}
	if parentPID != 0 {
		t.Fatalf("parentPID = %d, want 0", parentPID)
	}
}

func TestResolveInstanceLifetimeRejectsInvalidLifetime(t *testing.T) {
	t.Parallel()

	if _, _, err := resolveInstanceLifetime("vscode", false, "bad-lifetime", "", 1); err == nil {
		t.Fatal("expected invalid lifetime error")
	}
}

func TestResolveInstanceLifetimeRejectsInvalidParentPID(t *testing.T) {
	t.Parallel()

	if _, _, err := resolveInstanceLifetime("vscode", false, "host-bound", "bad-pid", 1); err == nil {
		t.Fatal("expected invalid parent pid error")
	}
}
