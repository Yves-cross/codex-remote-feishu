package feishu

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (p *DriveMarkdownPreviewer) CleanupBefore(ctx context.Context, cutoff time.Time) (PreviewDriveCleanupResult, error) {
	if p == nil {
		return PreviewDriveCleanupResult{}, nil
	}
	if p.api == nil {
		return PreviewDriveCleanupResult{}, fmt.Errorf("preview drive api is not available")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.loadStateLocked()
	result, err := p.cleanupManagedPreviewFilesLocked(ctx, state, cutoff)
	if err != nil {
		return PreviewDriveCleanupResult{}, err
	}
	state.LastCleanupAt = p.nowUTC()
	result.Summary, err = p.summarizeManagedInventoryLocked(ctx, state)
	if err != nil {
		return PreviewDriveCleanupResult{}, err
	}
	if err := p.saveStateLocked(); err != nil {
		return PreviewDriveCleanupResult{}, err
	}
	return result, nil
}

func (p *DriveMarkdownPreviewer) Summary() (PreviewDriveSummary, error) {
	if p == nil {
		return PreviewDriveSummary{}, nil
	}
	if p.api == nil {
		return PreviewDriveSummary{}, fmt.Errorf("preview drive api is not available")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.loadStateLocked()
	beforeToken, beforeURL := previewRootSnapshot(state)
	summary, err := p.summarizeManagedInventoryLocked(context.Background(), state)
	if err != nil {
		return PreviewDriveSummary{}, err
	}
	afterToken, afterURL := previewRootSnapshot(state)
	if beforeToken != afterToken || beforeURL != afterURL {
		if err := p.saveStateLocked(); err != nil {
			return PreviewDriveSummary{}, err
		}
	}
	return summary, nil
}

func (p *DriveMarkdownPreviewer) summarizeManagedInventoryLocked(ctx context.Context, state *previewState) (PreviewDriveSummary, error) {
	state = normalizePreviewState(state)
	summary := PreviewDriveSummary{
		StatePath: strings.TrimSpace(p.config.StatePath),
	}

	snapshot, ok, err := p.loadManagedInventoryLocked(ctx, state)
	if err != nil {
		return PreviewDriveSummary{}, err
	}
	if !ok {
		return summary, nil
	}

	summary.RootToken = snapshot.root.Token
	summary.RootURL = snapshot.root.URL
	summary.FileCount = len(snapshot.files)
	summary.ScopeCount = len(snapshot.folders)

	recordsByToken := map[string]*previewFileRecord{}
	for _, record := range state.Files {
		if record == nil {
			continue
		}
		token := strings.TrimSpace(record.Token)
		if token == "" {
			continue
		}
		recordsByToken[token] = record
	}

	for _, node := range snapshot.files {
		record := recordsByToken[node.Token]
		if record != nil && record.SizeBytes > 0 {
			summary.EstimatedBytes += record.SizeBytes
		} else {
			summary.UnknownSizeFileCount++
		}
		if value, ok := previewInventorySummaryTime(record, node); ok {
			updatePreviewSummaryWindow(&summary, value)
		}
	}

	return summary, nil
}

func previewRootSnapshot(state *previewState) (string, string) {
	if state == nil || state.Root == nil {
		return "", ""
	}
	return strings.TrimSpace(state.Root.Token), strings.TrimSpace(state.Root.URL)
}

func previewInventorySummaryTime(record *previewFileRecord, node previewRemoteNode) (time.Time, bool) {
	if value, ok := previewRecordLastUsedAt(record); ok {
		return value, true
	}
	return previewRemoteCleanupTime(node)
}

func updatePreviewSummaryWindow(summary *PreviewDriveSummary, value time.Time) {
	if summary == nil || value.IsZero() {
		return
	}
	value = value.UTC()
	if summary.OldestLastUsedAt == nil || value.Before(*summary.OldestLastUsedAt) {
		copyValue := value
		summary.OldestLastUsedAt = &copyValue
	}
	if summary.NewestLastUsedAt == nil || value.After(*summary.NewestLastUsedAt) {
		copyValue := value
		summary.NewestLastUsedAt = &copyValue
	}
}
