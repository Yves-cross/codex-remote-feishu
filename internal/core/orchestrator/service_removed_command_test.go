package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestRemovedNewInstanceCommandShowsMigrationNotice(t *testing.T) {
	now := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionRemovedCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/newinstance",
	})

	if len(events) != 1 || events[0].Notice == nil || events[0].Notice.Code != "command_removed_newinstance" {
		t.Fatalf("expected removed command notice, got %#v", events)
	}
	if !strings.Contains(events[0].Notice.Text, "/use") || !strings.Contains(events[0].Notice.Text, "/useall") {
		t.Fatalf("expected migration guidance in removed command notice, got %#v", events[0].Notice)
	}
}

func TestRemovedResumeHeadlessCardShowsMigrationNotice(t *testing.T) {
	now := time.Date(2026, 4, 8, 10, 5, 0, 0, time.UTC)
	svc := newServiceForTest(&now)

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionRemovedCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "resume_headless_thread",
	})
	if len(events) != 1 || events[0].Notice == nil || events[0].Notice.Code != "selection_expired" {
		t.Fatalf("expected stale headless card notice, got %#v", events)
	}
	if !strings.Contains(events[0].Notice.Text, "/newinstance") {
		t.Fatalf("expected stale headless card notice to mention removed command, got %#v", events[0].Notice)
	}
	if !strings.Contains(events[0].Notice.Text, "/use") || !strings.Contains(events[0].Notice.Text, "/useall") {
		t.Fatalf("expected migration guidance for stale headless card, got %#v", events[0].Notice)
	}
}

func TestRemovedKillInstanceCommandShowsDetachMigrationNotice(t *testing.T) {
	now := time.Date(2026, 4, 8, 10, 5, 30, 0, time.UTC)
	svc := newServiceForTest(&now)

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionRemovedCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/killinstance",
	})
	if len(events) != 1 || events[0].Notice == nil || events[0].Notice.Code != "command_removed_killinstance" {
		t.Fatalf("expected killinstance migration notice, got %#v", events)
	}
	if !strings.Contains(events[0].Notice.Text, "/detach") {
		t.Fatalf("expected killinstance migration to mention /detach, got %#v", events[0].Notice)
	}
}

func TestRemovedUnknownCommandShowsConcreteCommand(t *testing.T) {
	now := time.Date(2026, 4, 8, 10, 6, 0, 0, time.UTC)
	svc := newServiceForTest(&now)

	events := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionRemovedCommand,
		SurfaceSessionID: "surface-1",
		ChatID:           "chat-1",
		ActorUserID:      "user-1",
		Text:             "/legacy-command",
	})

	if len(events) != 1 || events[0].Notice == nil || events[0].Notice.Code != "command_removed" {
		t.Fatalf("expected generic removed command notice, got %#v", events)
	}
	if !strings.Contains(events[0].Notice.Text, "/legacy-command") {
		t.Fatalf("expected generic removed notice to mention concrete command, got %#v", events[0].Notice)
	}
}
