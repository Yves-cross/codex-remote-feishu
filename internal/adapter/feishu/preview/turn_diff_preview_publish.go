package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func joinPreviewErrors(errs ...error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		text := strings.TrimSpace(err.Error())
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}

func (p *DriveMarkdownPreviewer) maybePublishTurnDiffPreview(ctx context.Context, req FinalBlockPreviewRequest) (*control.TurnDiffPreview, error) {
	if p == nil || p.webPublisher == nil || strings.TrimSpace(p.config.CacheDir) == "" {
		return nil, nil
	}
	if req.TurnDiffSnapshot == nil || strings.TrimSpace(req.TurnDiffSnapshot.Diff) == "" {
		return nil, nil
	}

	scopeKey := previewScopeKey(req.GatewayID, req.SurfaceSessionID, req.ChatID, req.ActorUserID)
	if strings.TrimSpace(scopeKey) == "" {
		return nil, nil
	}

	artifact, err := p.buildTurnDiffPreviewArtifact(req)
	if err != nil {
		return nil, err
	}
	if artifact == nil || len(artifact.Files) == 0 {
		return nil, nil
	}
	content, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("marshal turn diff preview artifact: %w", err)
	}
	sum := sha256.Sum256(content)
	contentHash := hex.EncodeToString(sum[:])

	result, err := p.publishWebPreviewArtifact(ctx, PreviewPublishRequest{
		Request:  req,
		ScopeKey: scopeKey,
		Plan: PreviewPlan{
			Artifact: PreparedPreviewArtifact{
				SourcePath:   turnDiffPreviewSourcePath(req.TurnDiffSnapshot),
				DisplayName:  "变更查看",
				ContentHash:  contentHash,
				ArtifactKind: turnDiffPreviewArtifactKind,
				MIMEType:     "application/json",
				RendererKind: turnDiffPreviewRendererKind,
				Text:         string(content),
				Bytes:        content,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(result.URL) == "" {
		return nil, nil
	}
	return &control.TurnDiffPreview{URL: result.URL}, nil
}

func (p *DriveMarkdownPreviewer) buildTurnDiffPreviewArtifact(req FinalBlockPreviewRequest) (*turnDiffPreviewArtifact, error) {
	if req.TurnDiffSnapshot == nil || strings.TrimSpace(req.TurnDiffSnapshot.Diff) == "" {
		return nil, nil
	}
	parsedFiles := parseTurnDiffUnifiedDiff(req.TurnDiffSnapshot.Diff)
	if len(parsedFiles) == 0 {
		parsedFiles = []turnDiffParsedFile{newTurnDiffFallbackFile(req.TurnDiffSnapshot.Diff)}
	}
	files := make([]turnDiffPreviewFile, 0, len(parsedFiles))
	for _, parsed := range parsedFiles {
		files = append(files, p.buildTurnDiffPreviewFile(parsed, req))
	}
	return &turnDiffPreviewArtifact{
		SchemaVersion:  turnDiffPreviewSchemaV1,
		ThreadID:       strings.TrimSpace(req.TurnDiffSnapshot.ThreadID),
		TurnID:         strings.TrimSpace(req.TurnDiffSnapshot.TurnID),
		GeneratedAt:    p.nowUTC(),
		RawUnifiedDiff: req.TurnDiffSnapshot.Diff,
		Files:          files,
	}, nil
}

func (p *DriveMarkdownPreviewer) buildTurnDiffPreviewFile(file turnDiffParsedFile, req FinalBlockPreviewRequest) turnDiffPreviewFile {
	stats := turnDiffFileStats(file)
	artifact := turnDiffPreviewFile{
		Key:         turnDiffPreviewFileKey(file),
		Name:        turnDiffDisplayName(file),
		OldPath:     strings.TrimSpace(file.OldPath),
		NewPath:     strings.TrimSpace(file.NewPath),
		ChangeKind:  strings.TrimSpace(file.ChangeKind),
		Binary:      file.Binary,
		ParseStatus: "raw_patch",
		RawPatch:    strings.Join(file.RawLines, "\n"),
		Stats:       stats,
	}

	anchorText, anchorIsAfter, ok := p.loadTurnDiffAnchorText(file, req)
	if ok && len(file.Hunks) > 0 {
		lines, hunks, err := buildTurnDiffMergedView(file, anchorText, anchorIsAfter)
		if err == nil && len(lines) != 0 && len(hunks) != 0 {
			artifact.ParseStatus = "ok"
			artifact.Lines = lines
			artifact.Hunks = hunks
			if anchorIsAfter {
				artifact.AfterText = anchorText
				artifact.BeforeText = joinTurnDiffLinesText(lines, "context", "remove")
			} else {
				artifact.BeforeText = anchorText
				artifact.AfterText = joinTurnDiffLinesText(lines, "context", "add")
			}
			return artifact
		}
		if anchorIsAfter {
			artifact.AfterText = anchorText
		} else {
			artifact.BeforeText = anchorText
		}
	}

	lines, hunks := buildTurnDiffPatchOnlyView(file)
	artifact.Lines = lines
	artifact.Hunks = hunks
	switch {
	case file.Binary:
		artifact.ParseStatus = "binary"
	case len(file.Hunks) > 0:
		artifact.ParseStatus = "patch_only"
	default:
		artifact.ParseStatus = "raw_patch"
	}
	return artifact
}

func turnDiffPreviewFileKey(file turnDiffParsedFile) string {
	name := strings.TrimSpace(file.NewPath)
	if name == "" {
		name = strings.TrimSpace(file.OldPath)
	}
	if name == "" {
		return fmt.Sprintf("file-%d", file.Index)
	}
	return shortStablePreviewID(fmt.Sprintf("turn-diff-file|%d|%s", file.Index, name))
}

func turnDiffPreviewSourcePath(snapshot *control.TurnDiffSnapshot) string {
	if snapshot == nil {
		return filepath.Join("turn-diff", "snapshot.json")
	}
	threadID := shortStablePreviewID(strings.TrimSpace(snapshot.ThreadID))
	turnID := shortStablePreviewID(strings.TrimSpace(snapshot.TurnID))
	return filepath.Join("turn-diff", threadID, turnID+".json")
}
