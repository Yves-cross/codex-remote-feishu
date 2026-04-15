package daemon

import (
	"sort"
	"strings"
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/orchestrator"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

type adminExecCommandProgressEntryView struct {
	ItemID  string `json:"itemId,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Label   string `json:"label,omitempty"`
	Summary string `json:"summary,omitempty"`
	Status  string `json:"status,omitempty"`
}

type adminExecCommandProgressBlockRowView struct {
	RowID     string   `json:"rowId,omitempty"`
	Kind      string   `json:"kind,omitempty"`
	Items     []string `json:"items,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Secondary string   `json:"secondary,omitempty"`
}

type adminExecCommandProgressBlockView struct {
	BlockID string                                 `json:"blockId,omitempty"`
	Kind    string                                 `json:"kind,omitempty"`
	Status  string                                 `json:"status,omitempty"`
	Rows    []adminExecCommandProgressBlockRowView `json:"rows,omitempty"`
}

type adminExecCommandProgressView struct {
	ThreadID string                              `json:"threadId,omitempty"`
	TurnID   string                              `json:"turnId,omitempty"`
	ItemID   string                              `json:"itemId,omitempty"`
	Status   string                              `json:"status,omitempty"`
	Command  string                              `json:"command,omitempty"`
	Commands []string                            `json:"commands,omitempty"`
	CWD      string                              `json:"cwd,omitempty"`
	Blocks   []adminExecCommandProgressBlockView `json:"blocks,omitempty"`
	Entries  []adminExecCommandProgressEntryView `json:"entries,omitempty"`
}

type adminSurfaceStatusSummary struct {
	SurfaceSessionID     string                        `json:"surfaceSessionId"`
	Platform             string                        `json:"platform,omitempty"`
	ProductMode          string                        `json:"productMode,omitempty"`
	DisplayTitle         string                        `json:"displayTitle"`
	ThreadTitle          string                        `json:"threadTitle,omitempty"`
	FirstUserMessage     string                        `json:"firstUserMessage,omitempty"`
	LastUserMessage      string                        `json:"lastUserMessage,omitempty"`
	LastAssistantMessage string                        `json:"lastAssistantMessage,omitempty"`
	WorkspacePath        string                        `json:"workspacePath,omitempty"`
	InstanceDisplayName  string                        `json:"instanceDisplayName,omitempty"`
	LastActiveAt         *time.Time                    `json:"lastActiveAt,omitempty"`
	Progress             *adminExecCommandProgressView `json:"progress,omitempty"`
}

func (a *App) runtimeSurfaceStatusesLocked(surfaces []*state.SurfaceConsoleRecord) []adminSurfaceStatusSummary {
	summaries := make([]adminSurfaceStatusSummary, 0, len(surfaces))
	for _, surface := range surfaces {
		if surface == nil {
			continue
		}
		snapshot := a.service.SurfaceSnapshot(surface.SurfaceSessionID)
		summary := adminSurfaceStatusSummary{
			SurfaceSessionID: surface.SurfaceSessionID,
			Platform:         strings.TrimSpace(surface.Platform),
			ProductMode:      string(state.NormalizeProductMode(surface.ProductMode)),
			Progress:         adminExecCommandProgressViewFromControl(orchestrator.ExecCommandProgressSnapshot(surface.ActiveExecProgress)),
		}
		if snapshot != nil {
			summary.ThreadTitle = normalizeAdminSurfaceText(snapshot.Attachment.SelectedThreadTitle)
			summary.FirstUserMessage = normalizeAdminSurfaceText(snapshot.Attachment.SelectedThreadFirstUserMessage)
			summary.LastUserMessage = normalizeAdminSurfaceText(snapshot.Attachment.SelectedThreadLastUserMessage)
			summary.LastAssistantMessage = normalizeAdminSurfaceText(snapshot.Attachment.SelectedThreadLastAssistantMessage)
			summary.WorkspacePath = normalizeAdminSurfaceText(snapshot.WorkspaceKey)
			summary.InstanceDisplayName = normalizeAdminSurfaceText(snapshot.Attachment.DisplayName)
		}
		if summary.WorkspacePath == "" {
			summary.WorkspacePath = normalizeAdminSurfaceText(surface.ClaimedWorkspaceKey)
		}
		if !surface.LastInboundAt.IsZero() {
			lastActive := surface.LastInboundAt
			summary.LastActiveAt = &lastActive
		}
		summary.DisplayTitle = adminSurfaceDisplayTitle(summary)
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		leftHasProgress := adminSurfaceHasVisibleProgress(summaries[i])
		rightHasProgress := adminSurfaceHasVisibleProgress(summaries[j])
		if leftHasProgress != rightHasProgress {
			return leftHasProgress
		}
		leftAt := adminSurfaceLastActiveUnix(summaries[i])
		rightAt := adminSurfaceLastActiveUnix(summaries[j])
		if leftAt != rightAt {
			return leftAt > rightAt
		}
		if summaries[i].DisplayTitle != summaries[j].DisplayTitle {
			return summaries[i].DisplayTitle < summaries[j].DisplayTitle
		}
		return summaries[i].SurfaceSessionID < summaries[j].SurfaceSessionID
	})
	return summaries
}

func adminExecCommandProgressViewFromControl(progress *control.ExecCommandProgress) *adminExecCommandProgressView {
	if progress == nil {
		return nil
	}
	view := &adminExecCommandProgressView{
		ThreadID: normalizeAdminSurfaceText(progress.ThreadID),
		TurnID:   normalizeAdminSurfaceText(progress.TurnID),
		ItemID:   normalizeAdminSurfaceText(progress.ItemID),
		Status:   normalizeAdminSurfaceText(progress.Status),
		Command:  normalizeAdminSurfaceText(progress.Command),
		CWD:      normalizeAdminSurfaceText(progress.CWD),
	}
	for _, command := range progress.Commands {
		if normalized := normalizeAdminSurfaceText(command); normalized != "" {
			view.Commands = append(view.Commands, normalized)
		}
	}
	for _, block := range progress.Blocks {
		blockView := adminExecCommandProgressBlockView{
			BlockID: normalizeAdminSurfaceText(block.BlockID),
			Kind:    normalizeAdminSurfaceText(block.Kind),
			Status:  normalizeAdminSurfaceText(block.Status),
		}
		for _, row := range block.Rows {
			rowView := adminExecCommandProgressBlockRowView{
				RowID:     normalizeAdminSurfaceText(row.RowID),
				Kind:      normalizeAdminSurfaceText(row.Kind),
				Summary:   normalizeAdminSurfaceText(row.Summary),
				Secondary: normalizeAdminSurfaceText(row.Secondary),
			}
			for _, item := range row.Items {
				if normalized := normalizeAdminSurfaceText(item); normalized != "" {
					rowView.Items = append(rowView.Items, normalized)
				}
			}
			if rowView.Kind == "" {
				continue
			}
			if len(rowView.Items) == 0 && rowView.Summary == "" {
				continue
			}
			blockView.Rows = append(blockView.Rows, rowView)
		}
		if blockView.Kind == "" || len(blockView.Rows) == 0 {
			continue
		}
		view.Blocks = append(view.Blocks, blockView)
	}
	for _, entry := range progress.Entries {
		entryView := adminExecCommandProgressEntryView{
			ItemID:  normalizeAdminSurfaceText(entry.ItemID),
			Kind:    normalizeAdminSurfaceText(entry.Kind),
			Label:   normalizeAdminSurfaceText(entry.Label),
			Summary: normalizeAdminSurfaceText(entry.Summary),
			Status:  normalizeAdminSurfaceText(entry.Status),
		}
		if entryView.Summary == "" {
			continue
		}
		view.Entries = append(view.Entries, entryView)
	}
	if len(view.Blocks) == 0 && len(view.Entries) == 0 && len(view.Commands) == 0 && view.Command == "" && view.Status == "" {
		return nil
	}
	return view
}

func normalizeAdminSurfaceText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func adminSurfaceDisplayTitle(summary adminSurfaceStatusSummary) string {
	for _, candidate := range []string{
		summary.ThreadTitle,
		summary.LastUserMessage,
		summary.FirstUserMessage,
		summary.LastAssistantMessage,
		summary.InstanceDisplayName,
		summary.WorkspacePath,
	} {
		if normalized := normalizeAdminSurfaceText(candidate); normalized != "" {
			return normalized
		}
	}
	return "未命名会话"
}

func adminSurfaceHasVisibleProgress(summary adminSurfaceStatusSummary) bool {
	progress := summary.Progress
	if progress == nil {
		return false
	}
	return len(progress.Blocks) != 0 || len(progress.Entries) != 0 || len(progress.Commands) != 0 || progress.Command != ""
}

func adminSurfaceLastActiveUnix(summary adminSurfaceStatusSummary) int64 {
	if summary.LastActiveAt == nil {
		return 0
	}
	return summary.LastActiveAt.Unix()
}
