package upgradeshim

import "testing"

func TestSidecarPath(t *testing.T) {
	cases := map[string]string{
		"/tmp/codex-remote-upgrade-shim":     "/tmp/codex-remote-upgrade-shim.remote.json",
		`C:\tmp\codex-remote-upgrade-shim`:   `C:\tmp\codex-remote-upgrade-shim.remote.json`,
		"/tmp/codex-remote-upgrade-shim.exe": "/tmp/codex-remote-upgrade-shim.remote.json",
	}
	for input, want := range cases {
		if got := SidecarPath(input); got != want {
			t.Fatalf("SidecarPath(%q) = %q, want %q", input, got, want)
		}
	}
}
