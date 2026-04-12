package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func pathPickerViewFromEvent(t *testing.T, event control.UIEvent) *control.FeishuPathPickerView {
	t.Helper()
	if event.Kind != control.UIEventFeishuPathPicker || event.FeishuPathPickerView == nil {
		t.Fatalf("expected path picker event, got %#v", event)
	}
	return event.FeishuPathPickerView
}

func singlePathPickerEvent(t *testing.T, events []control.UIEvent) *control.FeishuPathPickerView {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("expected one event, got %#v", events)
	}
	return pathPickerViewFromEvent(t, events[0])
}

func TestOpenPathPickerDirectoryModeNavigatesAndConfirmsCurrentDirectory(t *testing.T) {
	now := time.Date(2026, 4, 12, 20, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	events := svc.OpenPathPicker(control.Action{
		SurfaceSessionID: "surface-1",
		ActorUserID:      "user-1",
	}, control.PathPickerRequest{
		Mode:     control.PathPickerModeDirectory,
		RootPath: root,
	})
	view := singlePathPickerEvent(t, events)
	if !view.CanConfirm || view.CurrentPath != root {
		t.Fatalf("unexpected initial picker view: %#v", view)
	}

	enterEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerEnter,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
		PickerEntry:      "alpha",
	})
	entered := singlePathPickerEvent(t, enterEvents)
	if entered.CurrentPath != filepath.Join(root, "alpha") || !entered.CanGoUp {
		t.Fatalf("unexpected entered picker view: %#v", entered)
	}

	upEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerUp,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
	})
	back := singlePathPickerEvent(t, upEvents)
	if back.CurrentPath != root {
		t.Fatalf("expected to return to root, got %#v", back)
	}

	confirmEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerConfirm,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
	})
	if len(confirmEvents) != 1 || confirmEvents[0].Notice == nil || confirmEvents[0].Notice.Code != "path_picker_confirmed" {
		t.Fatalf("expected confirmed notice, got %#v", confirmEvents)
	}
	if svc.root.Surfaces["surface-1"].ActivePathPicker != nil {
		t.Fatalf("expected picker state to clear after confirm")
	}
}

func TestOpenPathPickerFileModeSelectsFileAndRejectsDirectorySelection(t *testing.T) {
	now := time.Date(2026, 4, 12, 20, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	events := svc.OpenPathPicker(control.Action{
		SurfaceSessionID: "surface-1",
		ActorUserID:      "user-1",
	}, control.PathPickerRequest{
		Mode:     control.PathPickerModeFile,
		RootPath: root,
	})
	view := singlePathPickerEvent(t, events)
	if view.CanConfirm {
		t.Fatalf("expected file picker to require an explicit file selection")
	}

	selectEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerSelect,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
		PickerEntry:      "file.txt",
	})
	selected := singlePathPickerEvent(t, selectEvents)
	if selected.SelectedPath != filepath.Join(root, "file.txt") || !selected.CanConfirm {
		t.Fatalf("unexpected selected picker view: %#v", selected)
	}

	rejectEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerSelect,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
		PickerEntry:      "subdir",
	})
	if len(rejectEvents) != 1 || rejectEvents[0].Notice == nil || rejectEvents[0].Notice.Code != "path_picker_not_file" {
		t.Fatalf("expected file-type rejection notice, got %#v", rejectEvents)
	}
}

func TestOpenPathPickerRejectsPathEscapesAndSymlinkEscapes(t *testing.T) {
	now := time.Date(2026, 4, 12, 20, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	root := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write inside: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "outside.txt"), []byte("no"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if err := os.Symlink(filepath.Join(outsideDir, "outside.txt"), filepath.Join(root, "escape.txt")); err != nil {
		t.Fatalf("symlink escape: %v", err)
	}
	events := svc.OpenPathPicker(control.Action{
		SurfaceSessionID: "surface-1",
		ActorUserID:      "user-1",
	}, control.PathPickerRequest{
		Mode:     control.PathPickerModeFile,
		RootPath: root,
	})
	view := singlePathPickerEvent(t, events)

	outsideEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerSelect,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
		PickerEntry:      "../outside.txt",
	})
	if len(outsideEvents) != 1 || outsideEvents[0].Notice == nil || outsideEvents[0].Notice.Code != "path_picker_invalid_entry" {
		t.Fatalf("expected out-of-root rejection notice, got %#v", outsideEvents)
	}

	escapeEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerSelect,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
		PickerEntry:      "escape.txt",
	})
	if len(escapeEvents) != 1 || escapeEvents[0].Notice == nil || escapeEvents[0].Notice.Code != "path_picker_invalid_entry" {
		t.Fatalf("expected symlink escape rejection notice, got %#v", escapeEvents)
	}
}

func TestOpenPathPickerDirectoryModeRejectsFileSelection(t *testing.T) {
	now := time.Date(2026, 4, 12, 20, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	events := svc.OpenPathPicker(control.Action{
		SurfaceSessionID: "surface-1",
		ActorUserID:      "user-1",
	}, control.PathPickerRequest{
		Mode:     control.PathPickerModeDirectory,
		RootPath: root,
	})
	view := singlePathPickerEvent(t, events)
	rejectEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerSelect,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
		PickerEntry:      "file.txt",
	})
	if len(rejectEvents) != 1 || rejectEvents[0].Notice == nil || rejectEvents[0].Notice.Code != "path_picker_not_directory" {
		t.Fatalf("expected directory-type rejection notice, got %#v", rejectEvents)
	}
}

func TestOpenPathPickerRejectsStalePickerID(t *testing.T) {
	now := time.Date(2026, 4, 12, 20, 0, 0, 0, time.UTC)
	svc := newServiceForTest(&now)
	root := t.TempDir()
	events := svc.OpenPathPicker(control.Action{
		SurfaceSessionID: "surface-1",
		ActorUserID:      "user-1",
	}, control.PathPickerRequest{
		Mode:     control.PathPickerModeDirectory,
		RootPath: root,
	})
	view := singlePathPickerEvent(t, events)
	events = svc.OpenPathPicker(control.Action{
		SurfaceSessionID: "surface-1",
		ActorUserID:      "user-1",
	}, control.PathPickerRequest{
		Mode:     control.PathPickerModeDirectory,
		RootPath: root,
	})
	latest := singlePathPickerEvent(t, events)
	rejectEvents := svc.ApplySurfaceAction(control.Action{
		Kind:             control.ActionPathPickerUp,
		SurfaceSessionID: "surface-1",
		PickerID:         view.PickerID,
	})
	if len(rejectEvents) != 1 || rejectEvents[0].Notice == nil || rejectEvents[0].Notice.Code != "path_picker_expired" {
		t.Fatalf("expected stale picker rejection, got %#v", rejectEvents)
	}
	if latest.PickerID == view.PickerID {
		t.Fatalf("expected new picker id")
	}
}
