package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestMenuActionKindRecognizesSteerAll(t *testing.T) {
	tests := []string{"steerall", "steer_all"}
	for _, key := range tests {
		got, ok := menuActionKind(key)
		if !ok || got != control.ActionSteerAll {
			t.Fatalf("event key %q => (%q, %v), want (%q, true)", key, got, ok, control.ActionSteerAll)
		}
	}
}

func TestMenuActionBuildsSteerAll(t *testing.T) {
	action, ok := menuAction("steer_all")
	if !ok {
		t.Fatal("expected steer_all menu action")
	}
	if action.Kind != control.ActionSteerAll {
		t.Fatalf("unexpected action: %#v", action)
	}
}
