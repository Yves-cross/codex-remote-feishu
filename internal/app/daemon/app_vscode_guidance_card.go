package daemon

import (
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const (
	vscodeGuidanceTrackingPrefix = "vscode-guidance:"
	vscodeGuidanceCardTTL        = 30 * time.Minute
)

func vscodeGuidanceTrackingKey(surfaceID string) string {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return ""
	}
	return vscodeGuidanceTrackingPrefix + surfaceID
}

func (a *App) syncVSCodeGuidanceCardsLocked() {
	if a.surfaceResumeRuntime.vscodeGuidanceCards == nil {
		a.surfaceResumeRuntime.vscodeGuidanceCards = map[string]*vscodeGuidanceCardState{}
	}
	now := time.Now().UTC()
	for surfaceID, card := range a.surfaceResumeRuntime.vscodeGuidanceCards {
		if strings.TrimSpace(surfaceID) == "" || card == nil {
			delete(a.surfaceResumeRuntime.vscodeGuidanceCards, surfaceID)
			continue
		}
		if !card.ExpiresAt.IsZero() && !card.ExpiresAt.After(now) {
			delete(a.surfaceResumeRuntime.vscodeGuidanceCards, surfaceID)
			continue
		}
		snapshot := a.service.SurfaceSnapshot(surfaceID)
		if snapshot == nil || state.NormalizeProductMode(state.ProductMode(snapshot.ProductMode)) != state.ProductModeVSCode {
			delete(a.surfaceResumeRuntime.vscodeGuidanceCards, surfaceID)
		}
	}
}

func (a *App) activeVSCodeGuidanceCardLocked(surfaceID string) *vscodeGuidanceCardState {
	a.syncVSCodeGuidanceCardsLocked()
	return a.surfaceResumeRuntime.vscodeGuidanceCards[strings.TrimSpace(surfaceID)]
}

func (a *App) ensureVSCodeGuidanceCardLocked(surfaceID, messageID string) *vscodeGuidanceCardState {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return nil
	}
	a.syncVSCodeGuidanceCardsLocked()
	card := a.surfaceResumeRuntime.vscodeGuidanceCards[surfaceID]
	if card == nil {
		card = &vscodeGuidanceCardState{SurfaceSessionID: surfaceID}
		a.surfaceResumeRuntime.vscodeGuidanceCards[surfaceID] = card
	}
	card.ExpiresAt = time.Now().UTC().Add(vscodeGuidanceCardTTL)
	if trimmed := strings.TrimSpace(messageID); trimmed != "" {
		card.MessageID = trimmed
	}
	return card
}

func (a *App) clearVSCodeGuidanceCardLocked(surfaceID string) {
	delete(a.surfaceResumeRuntime.vscodeGuidanceCards, strings.TrimSpace(surfaceID))
}

func (a *App) recordVSCodeGuidanceCardMessageLocked(trackingKey, messageID string) {
	trackingKey = strings.TrimSpace(trackingKey)
	if !strings.HasPrefix(trackingKey, vscodeGuidanceTrackingPrefix) {
		return
	}
	surfaceID := strings.TrimSpace(strings.TrimPrefix(trackingKey, vscodeGuidanceTrackingPrefix))
	if surfaceID == "" {
		return
	}
	a.ensureVSCodeGuidanceCardLocked(surfaceID, messageID)
}

func isVSCodeCompatibilityCatalog(catalog *control.FeishuDirectCommandCatalog) bool {
	if catalog == nil {
		return false
	}
	if strings.HasPrefix(strings.TrimSpace(catalog.Title), "VS Code 接入需要") {
		return true
	}
	for _, button := range catalog.RelatedButtons {
		if strings.TrimSpace(button.CommandText) == vscodeMigrateCommandText {
			return true
		}
	}
	for _, section := range catalog.Sections {
		for _, entry := range section.Entries {
			for _, button := range entry.Buttons {
				if strings.TrimSpace(button.CommandText) == vscodeMigrateCommandText {
					return true
				}
			}
		}
	}
	return false
}

func isVSCodeGuidanceNotice(surfaceMode state.ProductMode, notice *control.Notice) bool {
	if notice == nil {
		return false
	}
	code := strings.TrimSpace(notice.Code)
	switch code {
	case "surface_mode_switched",
		"no_online_instances",
		"not_attached_vscode",
		"surface_resume_instance_attached",
		"surface_resume_instance_busy",
		"surface_resume_instance_not_found",
		"surface_resume_open_vscode",
		"vscode_open_required",
		"vscode_migration_check_failed",
		"vscode_migration_not_needed",
		"vscode_migration_failed",
		"vscode_migration_applied_detect_failed",
		"vscode_migration_incomplete",
		"vscode_migration_applied":
		return true
	}
	if surfaceMode == state.ProductModeVSCode {
		text := strings.ToLower(strings.TrimSpace(notice.Title + " " + notice.Text))
		return strings.Contains(text, "vscode") || strings.Contains(text, "vs code")
	}
	return false
}

func vscodeGuidanceThemeForNotice(notice *control.Notice) string {
	if notice == nil {
		return "info"
	}
	if theme := strings.TrimSpace(notice.ThemeKey); theme != "" {
		return theme
	}
	code := strings.TrimSpace(notice.Code)
	switch code {
	case "surface_resume_instance_attached", "vscode_migration_not_needed", "vscode_migration_applied":
		return "success"
	case "vscode_migration_check_failed", "vscode_migration_failed", "vscode_migration_applied_detect_failed", "vscode_migration_incomplete", "surface_resume_instance_busy", "surface_resume_instance_not_found":
		return "error"
	default:
		return "info"
	}
}

func vscodeGuidanceButtonsForNotice(notice *control.Notice) []control.CommandCatalogButton {
	if notice == nil {
		return nil
	}
	switch strings.TrimSpace(notice.Code) {
	case "not_attached_vscode", "surface_resume_instance_busy", "surface_resume_instance_not_found":
		return []control.CommandCatalogButton{
			runCommandButton("选择实例", "/list", "primary", false),
		}
	case "surface_resume_instance_attached":
		return []control.CommandCatalogButton{
			runCommandButton("选择会话", "/use", "primary", false),
		}
	default:
		return nil
	}
}

func vscodeGuidanceCatalogFromNotice(notice *control.Notice) *control.FeishuDirectCommandCatalog {
	if notice == nil {
		return nil
	}
	title := strings.TrimSpace(notice.Title)
	if title == "" {
		title = "VS Code"
	}
	sections := append([]control.FeishuCardTextSection(nil), notice.Sections...)
	if len(sections) == 0 {
		sections = commandCatalogSummarySections(notice.Text)
	}
	buttons := vscodeGuidanceButtonsForNotice(notice)
	return &control.FeishuDirectCommandCatalog{
		Title:           title,
		SummarySections: sections,
		ThemeKey:        vscodeGuidanceThemeForNotice(notice),
		Patchable:       true,
		Interactive:     len(buttons) > 0,
		RelatedButtons:  buttons,
	}
}

func (a *App) rewriteVSCodeGuidanceEventLocked(event control.UIEvent, sourceMessageID string, replacement bool) control.UIEvent {
	surfaceID := strings.TrimSpace(event.SurfaceSessionID)
	if surfaceID == "" {
		return event
	}
	surfaceMode := state.ProductModeNormal
	if snapshot := a.service.SurfaceSnapshot(surfaceID); snapshot != nil {
		surfaceMode = state.NormalizeProductMode(state.ProductMode(snapshot.ProductMode))
	}

	var catalog *control.FeishuDirectCommandCatalog
	switch {
	case isVSCodeCompatibilityCatalog(event.FeishuDirectCommandCatalog):
		cloned := *event.FeishuDirectCommandCatalog
		cloned.Patchable = true
		catalog = &cloned
	case isVSCodeGuidanceNotice(surfaceMode, event.Notice):
		catalog = vscodeGuidanceCatalogFromNotice(event.Notice)
	default:
		return event
	}
	if catalog == nil {
		return event
	}

	card := a.ensureVSCodeGuidanceCardLocked(surfaceID, sourceMessageID)
	if card == nil {
		return event
	}
	if replacement {
		catalog.MessageID = ""
		catalog.TrackingKey = ""
	} else if strings.TrimSpace(card.MessageID) != "" {
		catalog.MessageID = strings.TrimSpace(card.MessageID)
		catalog.TrackingKey = ""
	} else {
		catalog.MessageID = ""
		catalog.TrackingKey = vscodeGuidanceTrackingKey(surfaceID)
	}

	event.Kind = control.UIEventFeishuDirectCommandCatalog
	event.FeishuDirectCommandCatalog = catalog
	event.Notice = nil
	return event
}
