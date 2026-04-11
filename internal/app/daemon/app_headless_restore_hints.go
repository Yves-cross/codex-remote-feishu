package daemon

import (
	"log"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func (a *App) configureHeadlessRestoreHintsLocked(stateDir string) {
	path := headlessRestoreHintsStatePath(stateDir)
	store, err := loadHeadlessRestoreHintStore(path)
	if err != nil {
		log.Printf("load headless restore hints failed: path=%s err=%v", path, err)
		store = &headlessRestoreHintStore{
			path:    path,
			entries: map[string]HeadlessRestoreHint{},
		}
	}
	a.headlessRestoreHints = store
	a.refreshHeadlessRestoreHintsLocked()
	a.syncHeadlessRestoreStateLocked()
}

func (a *App) HeadlessRestoreHint(surfaceID string) *HeadlessRestoreHint {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.headlessRestoreHints == nil {
		return nil
	}
	hint, ok := a.headlessRestoreHints.Get(surfaceID)
	if !ok {
		return nil
	}
	copy := hint
	return &copy
}

func (a *App) refreshHeadlessRestoreHintsLocked() {
	if a.headlessRestoreHints == nil {
		return
	}
	for _, surface := range a.service.Surfaces() {
		if surface == nil {
			continue
		}
		hint, ok := a.currentHeadlessRestoreHintLocked(surface.SurfaceSessionID)
		if !ok {
			continue
		}
		a.upsertHeadlessRestoreHintLocked(hint)
	}
}

func (a *App) refreshHeadlessRestoreHintsForInstanceLocked(instanceID string) {
	if a.headlessRestoreHints == nil {
		return
	}
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return
	}
	for _, surface := range a.service.Surfaces() {
		if surface == nil || strings.TrimSpace(surface.AttachedInstanceID) != instanceID {
			continue
		}
		hint, ok := a.currentHeadlessRestoreHintFromLiveSurfaceLocked(surface.SurfaceSessionID)
		if !ok {
			continue
		}
		a.upsertHeadlessRestoreHintLocked(hint)
	}
}

func (a *App) syncHeadlessRestoreHintAfterActionLocked(action control.Action, before *control.Snapshot) {
	if a.headlessRestoreHints == nil {
		return
	}
	hint, ok := a.currentHeadlessRestoreHintFromLiveSurfaceLocked(action.SurfaceSessionID)
	if ok {
		a.upsertHeadlessRestoreHintLocked(hint)
		a.syncHeadlessRestoreStateLocked()
		return
	}
	if a.shouldClearHeadlessRestoreHintLocked(action, before) {
		a.clearHeadlessRestoreHintLocked(action.SurfaceSessionID)
	}
	a.syncHeadlessRestoreStateLocked()
}

func (a *App) shouldClearHeadlessRestoreHintLocked(action control.Action, before *control.Snapshot) bool {
	switch action.Kind {
	case control.ActionDetach:
		return true
	case control.ActionKillInstance:
		return snapshotCarriesHeadlessRestoreTarget(before)
	case control.ActionModeCommand:
		after := a.service.SurfaceSnapshot(action.SurfaceSessionID)
		if after == nil || before == nil {
			return false
		}
		return !strings.EqualFold(strings.TrimSpace(before.ProductMode), strings.TrimSpace(after.ProductMode))
	case control.ActionAttachInstance, control.ActionUseThread:
		after := a.service.SurfaceSnapshot(action.SurfaceSessionID)
		if after == nil {
			return false
		}
		if snapshotHasPendingHeadlessTarget(after) {
			return false
		}
		return strings.TrimSpace(after.Attachment.InstanceID) != "" &&
			(!after.Attachment.Managed || !strings.EqualFold(strings.TrimSpace(after.Attachment.Source), "headless"))
	default:
		return false
	}
}

func (a *App) currentHeadlessRestoreHintLocked(surfaceID string) (HeadlessRestoreHint, bool) {
	if hint, ok := a.currentHeadlessRestoreHintFromLiveSurfaceLocked(surfaceID); ok {
		return hint, true
	}
	if a.surfaceResumeState != nil {
		if entry, ok := a.surfaceResumeState.Get(surfaceID); ok {
			if hint, ok := headlessRestoreHintFromSurfaceResumeEntry(entry); ok {
				return hint, true
			}
		}
	}
	return HeadlessRestoreHint{}, false
}

func (a *App) currentHeadlessRestoreHintFromLiveSurfaceLocked(surfaceID string) (HeadlessRestoreHint, bool) {
	if surface := a.surfaceByIDLocked(surfaceID); surface != nil {
		target, ok := a.currentSurfaceResumeTargetLocked(surface)
		if !ok {
			return HeadlessRestoreHint{}, false
		}
		entry := SurfaceResumeEntry{
			SurfaceSessionID:  strings.TrimSpace(surface.SurfaceSessionID),
			GatewayID:         strings.TrimSpace(surface.GatewayID),
			ChatID:            strings.TrimSpace(surface.ChatID),
			ActorUserID:       strings.TrimSpace(surface.ActorUserID),
			ResumeThreadID:    target.ResumeThreadID,
			ResumeThreadTitle: target.ResumeThreadTitle,
			ResumeThreadCWD:   target.ResumeThreadCWD,
			ResumeHeadless:    target.ResumeHeadless,
		}
		return headlessRestoreHintFromSurfaceResumeEntry(entry)
	}
	return HeadlessRestoreHint{}, false
}

func (a *App) upsertHeadlessRestoreHintLocked(hint HeadlessRestoreHint) {
	if a.headlessRestoreHints == nil {
		return
	}
	if normalized, ok := normalizeHeadlessRestoreHint(hint); ok {
		if current, exists := a.headlessRestoreHints.Get(normalized.SurfaceSessionID); exists && sameHeadlessRestoreHintContent(current, normalized) {
			return
		}
		normalized.UpdatedAt = time.Now().UTC()
		if err := a.headlessRestoreHints.Put(normalized); err != nil {
			log.Printf("persist headless restore hint failed: surface=%s thread=%s err=%v", normalized.SurfaceSessionID, normalized.ThreadID, err)
		}
	}
}

func (a *App) clearHeadlessRestoreHintLocked(surfaceID string) {
	if a.headlessRestoreHints == nil {
		return
	}
	if _, ok := a.headlessRestoreHints.Get(surfaceID); !ok {
		return
	}
	if err := a.headlessRestoreHints.Delete(surfaceID); err != nil {
		log.Printf("clear headless restore hint failed: surface=%s err=%v", surfaceID, err)
	}
}

func sameHeadlessRestoreHintContent(left, right HeadlessRestoreHint) bool {
	return strings.TrimSpace(left.SurfaceSessionID) == strings.TrimSpace(right.SurfaceSessionID) &&
		strings.TrimSpace(left.GatewayID) == strings.TrimSpace(right.GatewayID) &&
		strings.TrimSpace(left.ChatID) == strings.TrimSpace(right.ChatID) &&
		strings.TrimSpace(left.ActorUserID) == strings.TrimSpace(right.ActorUserID) &&
		strings.TrimSpace(left.ThreadID) == strings.TrimSpace(right.ThreadID) &&
		strings.TrimSpace(left.ThreadTitle) == strings.TrimSpace(right.ThreadTitle) &&
		strings.TrimSpace(left.ThreadCWD) == strings.TrimSpace(right.ThreadCWD)
}

func headlessRestoreHintFromSurfaceResumeEntry(entry SurfaceResumeEntry) (HeadlessRestoreHint, bool) {
	if !entry.ResumeHeadless {
		return HeadlessRestoreHint{}, false
	}
	hint := HeadlessRestoreHint{
		SurfaceSessionID: strings.TrimSpace(entry.SurfaceSessionID),
		GatewayID:        strings.TrimSpace(entry.GatewayID),
		ChatID:           strings.TrimSpace(entry.ChatID),
		ActorUserID:      strings.TrimSpace(entry.ActorUserID),
		ThreadID:         strings.TrimSpace(entry.ResumeThreadID),
		ThreadTitle:      firstNonEmpty(strings.TrimSpace(entry.ResumeThreadTitle), strings.TrimSpace(entry.ResumeThreadID)),
		ThreadCWD:        state.NormalizeWorkspaceKey(entry.ResumeThreadCWD),
	}
	return normalizeHeadlessRestoreHint(hint)
}

func snapshotHasPendingHeadlessTarget(snapshot *control.Snapshot) bool {
	return snapshot != nil &&
		strings.TrimSpace(snapshot.PendingHeadless.InstanceID) != "" &&
		strings.TrimSpace(snapshot.PendingHeadless.ThreadID) != ""
}

func snapshotHasAttachedHeadlessTarget(snapshot *control.Snapshot) bool {
	return snapshot != nil &&
		strings.TrimSpace(snapshot.Attachment.InstanceID) != "" &&
		snapshot.Attachment.Managed &&
		strings.EqualFold(strings.TrimSpace(snapshot.Attachment.Source), "headless") &&
		strings.TrimSpace(snapshot.Attachment.SelectedThreadID) != ""
}

func snapshotCarriesHeadlessRestoreTarget(snapshot *control.Snapshot) bool {
	if snapshot == nil {
		return false
	}
	if snapshotHasPendingHeadlessTarget(snapshot) {
		return true
	}
	return strings.TrimSpace(snapshot.Attachment.InstanceID) != "" &&
		snapshot.Attachment.Managed &&
		strings.EqualFold(strings.TrimSpace(snapshot.Attachment.Source), "headless")
}
