package feishu

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"

	"github.com/kxn/codex-remote-feishu/internal/core/render"
)

const (
	defaultPreviewRootFolderName = "Codex Remote Previews"
	defaultPreviewMaxFileBytes   = 20 * 1024 * 1024
	previewFileType              = "file"
	previewFolderType            = "folder"
	previewPermissionView        = "view"
)

var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
var markdownLineSuffixPattern = regexp.MustCompile(`^(.*\.md)(:\d+(?::\d+)?)$`)

type MarkdownPreviewService interface {
	RewriteFinalBlock(context.Context, MarkdownPreviewRequest) (render.Block, error)
}

type PreviewDriveAdminService interface {
	Summary() (PreviewDriveSummary, error)
	CleanupBefore(context.Context, time.Time) (PreviewDriveCleanupResult, error)
	Reconcile(context.Context) (PreviewDriveReconcileResult, error)
}

type MarkdownPreviewRequest struct {
	GatewayID        string
	SurfaceSessionID string
	ChatID           string
	ActorUserID      string
	WorkspaceRoot    string
	ThreadCWD        string
	Block            render.Block
}

type MarkdownPreviewConfig struct {
	StatePath      string
	RootFolderName string
	ProcessCWD     string
	MaxFileBytes   int64
}

type DriveMarkdownPreviewer struct {
	api    previewDriveAPI
	config MarkdownPreviewConfig

	mu     sync.Mutex
	loaded bool
	state  *previewState
}

type previewDriveAPI interface {
	CreateFolder(context.Context, string, string) (previewRemoteNode, error)
	UploadFile(context.Context, string, string, []byte) (string, error)
	QueryMetaURL(context.Context, string, string) (string, error)
	GrantPermission(context.Context, string, string, previewPrincipal) error
	DeleteFile(context.Context, string, string) error
	ListFiles(context.Context, string) ([]previewRemoteNode, error)
	ListPermissionMembers(context.Context, string, string) (map[string]bool, error)
}

type previewRemoteNode struct {
	Token string
	URL   string
	Type  string
}

type previewPrincipal struct {
	Key        string
	MemberType string
	MemberID   string
	Type       string
}

type previewState struct {
	Root   *previewFolderRecord           `json:"root,omitempty"`
	Scopes map[string]*previewScopeRecord `json:"scopes,omitempty"`
	Files  map[string]*previewFileRecord  `json:"files,omitempty"`
}

type previewScopeRecord struct {
	Folder     *previewFolderRecord `json:"folder,omitempty"`
	LastUsedAt time.Time            `json:"lastUsedAt,omitempty"`
}

type previewFolderRecord struct {
	Token            string          `json:"token,omitempty"`
	URL              string          `json:"url,omitempty"`
	Shared           map[string]bool `json:"shared,omitempty"`
	LastReconciledAt time.Time       `json:"lastReconciledAt,omitempty"`
}

type previewFileRecord struct {
	Path       string          `json:"path,omitempty"`
	SHA256     string          `json:"sha256,omitempty"`
	Token      string          `json:"token,omitempty"`
	URL        string          `json:"url,omitempty"`
	Shared     map[string]bool `json:"shared,omitempty"`
	ScopeKey   string          `json:"scopeKey,omitempty"`
	SizeBytes  int64           `json:"sizeBytes,omitempty"`
	CreatedAt  time.Time       `json:"createdAt,omitempty"`
	LastUsedAt time.Time       `json:"lastUsedAt,omitempty"`
}

type PreviewDriveSummary struct {
	StatePath            string     `json:"statePath,omitempty"`
	RootToken            string     `json:"rootToken,omitempty"`
	RootURL              string     `json:"rootURL,omitempty"`
	FileCount            int        `json:"fileCount"`
	ScopeCount           int        `json:"scopeCount"`
	EstimatedBytes       int64      `json:"estimatedBytes"`
	UnknownSizeFileCount int        `json:"unknownSizeFileCount"`
	OldestLastUsedAt     *time.Time `json:"oldestLastUsedAt,omitempty"`
	NewestLastUsedAt     *time.Time `json:"newestLastUsedAt,omitempty"`
}

type PreviewDriveCleanupResult struct {
	DeletedFileCount            int                 `json:"deletedFileCount"`
	DeletedEstimatedBytes       int64               `json:"deletedEstimatedBytes"`
	SkippedUnknownLastUsedCount int                 `json:"skippedUnknownLastUsedCount"`
	Summary                     PreviewDriveSummary `json:"summary"`
}

type PreviewDriveReconcileResult struct {
	Summary                 PreviewDriveSummary `json:"summary"`
	RootMissing             bool                `json:"rootMissing"`
	RemoteMissingScopeCount int                 `json:"remoteMissingScopeCount"`
	RemoteMissingFileCount  int                 `json:"remoteMissingFileCount"`
	LocalOnlyScopeCount     int                 `json:"localOnlyScopeCount"`
	LocalOnlyFileCount      int                 `json:"localOnlyFileCount"`
	PermissionDriftCount    int                 `json:"permissionDriftCount"`
}

type driveAPIError struct {
	Code int
	Msg  string
}

func (e *driveAPIError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Msg) == "" {
		return fmt.Sprintf("feishu drive api error %d", e.Code)
	}
	return fmt.Sprintf("feishu drive api error %d: %s", e.Code, strings.TrimSpace(e.Msg))
}

func NewDriveMarkdownPreviewer(api previewDriveAPI, cfg MarkdownPreviewConfig) *DriveMarkdownPreviewer {
	if cfg.RootFolderName == "" {
		cfg.RootFolderName = defaultPreviewRootFolderName
	}
	if cfg.MaxFileBytes <= 0 {
		cfg.MaxFileBytes = defaultPreviewMaxFileBytes
	}
	if cfg.ProcessCWD == "" {
		if cwd, err := os.Getwd(); err == nil {
			cfg.ProcessCWD = cwd
		}
	}
	return &DriveMarkdownPreviewer{
		api:    api,
		config: cfg,
	}
}

func (p *DriveMarkdownPreviewer) RewriteFinalBlock(ctx context.Context, req MarkdownPreviewRequest) (render.Block, error) {
	block := req.Block
	if p == nil || p.api == nil {
		return block, nil
	}
	if !block.Final || block.Kind != render.BlockAssistantMarkdown || strings.TrimSpace(block.Text) == "" {
		return block, nil
	}

	principals := previewPrincipals(req.SurfaceSessionID, req.ChatID, req.ActorUserID)
	if len(principals) == 0 {
		return block, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.loadStateLocked()
	rewritten, changed, rewriteErr := p.rewriteMarkdownLinksLocked(ctx, state, req, principals)
	if changed {
		block.Text = rewritten
		if err := p.saveStateLocked(); err != nil && rewriteErr == nil {
			rewriteErr = err
		}
	}
	return block, rewriteErr
}

func (p *DriveMarkdownPreviewer) rewriteMarkdownLinksLocked(ctx context.Context, state *previewState, req MarkdownPreviewRequest, principals []previewPrincipal) (string, bool, error) {
	text := req.Block.Text
	matches := markdownLinkPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, false, nil
	}

	scopeKey := previewScopeKey(req.GatewayID, req.SurfaceSessionID, req.ChatID, req.ActorUserID)
	rewrittenTargets := map[string]string{}
	var errs []string

	var builder strings.Builder
	last := 0
	changed := false
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		targetStart := match[2]
		targetEnd := match[3]
		rawTarget := text[targetStart:targetEnd]

		builder.WriteString(text[last:targetStart])
		replacement := rawTarget
		if cached, ok := rewrittenTargets[rawTarget]; ok {
			replacement = cached
			if replacement != rawTarget {
				changed = true
			}
		} else {
			url, ok, err := p.materializeMarkdownTargetLocked(ctx, state, rawTarget, req, scopeKey, principals)
			switch {
			case err != nil:
				errs = append(errs, err.Error())
			case ok && url != "":
				replacement = url
				rewrittenTargets[rawTarget] = url
				changed = true
			default:
				rewrittenTargets[rawTarget] = rawTarget
			}
		}
		builder.WriteString(replacement)
		last = targetEnd
	}
	builder.WriteString(text[last:])

	var rewriteErr error
	if len(errs) > 0 {
		rewriteErr = errors.New(strings.Join(errs, "; "))
	}
	return builder.String(), changed, rewriteErr
}

func (p *DriveMarkdownPreviewer) materializeMarkdownTargetLocked(ctx context.Context, state *previewState, rawTarget string, req MarkdownPreviewRequest, scopeKey string, principals []previewPrincipal) (string, bool, error) {
	resolvedPath, ok, err := p.resolveMarkdownPath(rawTarget, req)
	if err != nil || !ok {
		return "", ok, err
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", true, fmt.Errorf("read markdown preview source %s: %w", resolvedPath, err)
	}
	if len(content) == 0 {
		return "", true, fmt.Errorf("skip empty markdown preview source %s", resolvedPath)
	}
	if int64(len(content)) > p.config.MaxFileBytes {
		return "", true, fmt.Errorf("markdown preview source exceeds %d bytes: %s", p.config.MaxFileBytes, resolvedPath)
	}

	sum := sha256.Sum256(content)
	contentSHA := hex.EncodeToString(sum[:])
	fileKey := previewFileKey(scopeKey, resolvedPath, contentSHA)

	scopeFolder, err := p.ensureScopeFolderLocked(ctx, state, scopeKey, principals)
	if err != nil {
		return "", true, err
	}

	record := state.Files[fileKey]
	if record == nil {
		record = &previewFileRecord{
			Path:      resolvedPath,
			SHA256:    contentSHA,
			ScopeKey:  scopeKey,
			SizeBytes: int64(len(content)),
		}
		state.Files[fileKey] = record
	}
	if record.ScopeKey == "" {
		record.ScopeKey = scopeKey
	}
	if record.SizeBytes <= 0 {
		record.SizeBytes = int64(len(content))
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.LastUsedAt = now
	scope := state.Scopes[scopeKey]
	if scope != nil {
		scope.LastUsedAt = now
	}

	if record.Token == "" {
		if err := p.uploadPreviewFileLocked(ctx, record, scopeFolder.Token, resolvedPath, content, contentSHA); err != nil {
			if isPreviewParentMissingError(err) {
				clearPreviewScope(state, scopeKey)
				scopeFolder, err = p.ensureScopeFolderLocked(ctx, state, scopeKey, principals)
				if err != nil {
					return "", true, err
				}
				record.Token = ""
				record.URL = ""
				record.Shared = nil
				if err := p.uploadPreviewFileLocked(ctx, record, scopeFolder.Token, resolvedPath, content, contentSHA); err != nil {
					return "", true, err
				}
			} else {
				return "", true, err
			}
		}
	}

	if record.URL == "" {
		url, err := p.api.QueryMetaURL(ctx, record.Token, previewFileType)
		if err != nil {
			return "", true, fmt.Errorf("query markdown preview url for %s: %w", resolvedPath, err)
		}
		record.URL = url
	}

	if err := ensurePreviewPermissions(ctx, p.api, record.Token, previewFileType, &record.Shared, principals); err != nil {
		if isPreviewResourceMissingError(err) {
			record.Token = ""
			record.URL = ""
			record.Shared = nil
			if err := p.uploadPreviewFileLocked(ctx, record, scopeFolder.Token, resolvedPath, content, contentSHA); err != nil {
				return "", true, err
			}
			if err := ensurePreviewPermissions(ctx, p.api, record.Token, previewFileType, &record.Shared, principals); err != nil {
				return "", true, err
			}
		} else {
			return "", true, err
		}
	}

	return record.URL, true, nil
}

func (p *DriveMarkdownPreviewer) uploadPreviewFileLocked(ctx context.Context, record *previewFileRecord, parentToken, resolvedPath string, content []byte, contentSHA string) error {
	fileToken, err := p.api.UploadFile(ctx, parentToken, previewFileName(resolvedPath, contentSHA), content)
	if err != nil {
		return fmt.Errorf("upload markdown preview for %s: %w", resolvedPath, err)
	}
	record.Token = fileToken
	record.URL = ""
	record.Shared = map[string]bool{}
	return nil
}

func (p *DriveMarkdownPreviewer) ensureScopeFolderLocked(ctx context.Context, state *previewState, scopeKey string, principals []previewPrincipal) (*previewFolderRecord, error) {
	scope := state.Scopes[scopeKey]
	if scope == nil {
		scope = &previewScopeRecord{}
		state.Scopes[scopeKey] = scope
	}

	for attempt := 0; attempt < 2; attempt++ {
		root, err := p.ensureRootFolderLocked(ctx, state)
		if err != nil {
			return nil, err
		}

		if scope.Folder == nil {
			scope.Folder = &previewFolderRecord{}
		}
		if scope.Folder.Token == "" {
			node, err := p.api.CreateFolder(ctx, previewScopeFolderName(scopeKey), root.Token)
			if err != nil {
				if isPreviewParentMissingError(err) {
					state.Root = nil
					continue
				}
				return nil, fmt.Errorf("create markdown preview folder for %s: %w", scopeKey, err)
			}
			scope.Folder.Token = node.Token
			scope.Folder.URL = node.URL
			scope.Folder.Shared = map[string]bool{}
		}

		if err := ensurePreviewPermissions(ctx, p.api, scope.Folder.Token, previewFolderType, &scope.Folder.Shared, principals); err != nil {
			if isPreviewResourceMissingError(err) {
				scope.Folder = nil
				continue
			}
			return nil, fmt.Errorf("authorize markdown preview folder for %s: %w", scopeKey, err)
		}
		return scope.Folder, nil
	}

	return nil, fmt.Errorf("create markdown preview folder for %s: exhausted retries", scopeKey)
}

func (p *DriveMarkdownPreviewer) ensureRootFolderLocked(ctx context.Context, state *previewState) (*previewFolderRecord, error) {
	if state.Root == nil {
		state.Root = &previewFolderRecord{}
	}
	if state.Root.Token != "" {
		return state.Root, nil
	}

	node, err := p.api.CreateFolder(ctx, p.config.RootFolderName, "")
	if err != nil {
		return nil, fmt.Errorf("create markdown preview root folder: %w", err)
	}
	state.Root.Token = node.Token
	state.Root.URL = node.URL
	return state.Root, nil
}

func (p *DriveMarkdownPreviewer) resolveMarkdownPath(rawTarget string, req MarkdownPreviewRequest) (string, bool, error) {
	target := strings.TrimSpace(rawTarget)
	if target == "" {
		return "", false, nil
	}
	if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") {
		target = strings.TrimPrefix(strings.TrimSuffix(target, ">"), "<")
	}
	if idx := strings.IndexAny(target, " \t\n"); idx >= 0 {
		target = target[:idx]
	}
	if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "#") {
		return "", false, nil
	}

	cleanTarget, _ := stripMarkdownLocationSuffix(target)
	if !strings.EqualFold(filepath.Ext(cleanTarget), ".md") {
		return "", false, nil
	}

	roots := previewAllowedRoots(req.ThreadCWD, req.WorkspaceRoot, p.config.ProcessCWD)
	candidates := previewPathCandidates(cleanTarget, roots)
	for _, candidate := range candidates {
		resolved, err := previewCanonicalPath(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", true, err
		}
		if !previewPathWithinAnyRoot(resolved, roots) {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", true, err
		}
		if info.IsDir() {
			continue
		}
		return resolved, true, nil
	}
	return "", false, nil
}

func (p *DriveMarkdownPreviewer) loadStateLocked() *previewState {
	if p.loaded {
		return p.state
	}
	p.loaded = true
	p.state = newPreviewState()
	if strings.TrimSpace(p.config.StatePath) == "" {
		return p.state
	}

	raw, err := os.ReadFile(p.config.StatePath)
	if err != nil {
		return p.state
	}
	var loaded previewState
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return p.state
	}
	p.state = normalizePreviewState(&loaded)
	return p.state
}

func (p *DriveMarkdownPreviewer) saveStateLocked() error {
	if strings.TrimSpace(p.config.StatePath) == "" || p.state == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p.config.StatePath), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(p.state, "", "  ")
	if err != nil {
		return err
	}
	tempPath := p.config.StatePath + ".tmp"
	if err := os.WriteFile(tempPath, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, p.config.StatePath)
}

func newPreviewState() *previewState {
	return &previewState{
		Scopes: map[string]*previewScopeRecord{},
		Files:  map[string]*previewFileRecord{},
	}
}

func normalizePreviewState(state *previewState) *previewState {
	if state == nil {
		return newPreviewState()
	}
	if state.Scopes == nil {
		state.Scopes = map[string]*previewScopeRecord{}
	}
	if state.Files == nil {
		state.Files = map[string]*previewFileRecord{}
	}
	if state.Root != nil && state.Root.Shared == nil {
		state.Root.Shared = map[string]bool{}
	}
	for _, scope := range state.Scopes {
		if scope == nil || scope.Folder == nil {
			continue
		}
		if scope.Folder.Shared == nil {
			scope.Folder.Shared = map[string]bool{}
		}
	}
	for key, file := range state.Files {
		if file == nil {
			continue
		}
		if file.Shared == nil {
			file.Shared = map[string]bool{}
		}
		if file.ScopeKey == "" {
			file.ScopeKey = previewRecordScopeKey(key)
		}
	}
	return state
}

func (p *DriveMarkdownPreviewer) Summary() (PreviewDriveSummary, error) {
	if p == nil {
		return PreviewDriveSummary{}, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return summarizePreviewState(p.loadStateLocked(), strings.TrimSpace(p.config.StatePath)), nil
}

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
	result := PreviewDriveCleanupResult{}
	keys := make([]string, 0, len(state.Files))
	for key := range state.Files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		record := state.Files[key]
		if record == nil {
			delete(state.Files, key)
			continue
		}
		lastUsedAt, ok := previewRecordLastUsedAt(record)
		if !ok {
			result.SkippedUnknownLastUsedCount++
			continue
		}
		if lastUsedAt.After(cutoff) {
			continue
		}
		if strings.TrimSpace(record.Token) != "" {
			err := p.api.DeleteFile(ctx, record.Token, previewFileType)
			if err != nil && !isPreviewResourceMissingError(err) {
				return PreviewDriveCleanupResult{}, err
			}
		}
		result.DeletedFileCount++
		if record.SizeBytes > 0 {
			result.DeletedEstimatedBytes += record.SizeBytes
		}
		delete(state.Files, key)
	}
	if err := p.saveStateLocked(); err != nil {
		return PreviewDriveCleanupResult{}, err
	}
	result.Summary = summarizePreviewState(state, strings.TrimSpace(p.config.StatePath))
	return result, nil
}

func (p *DriveMarkdownPreviewer) Reconcile(ctx context.Context) (PreviewDriveReconcileResult, error) {
	if p == nil {
		return PreviewDriveReconcileResult{}, nil
	}
	if p.api == nil {
		return PreviewDriveReconcileResult{}, fmt.Errorf("preview drive api is not available")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	state := p.loadStateLocked()
	result := PreviewDriveReconcileResult{
		Summary: summarizePreviewState(state, strings.TrimSpace(p.config.StatePath)),
	}
	if state.Root == nil || strings.TrimSpace(state.Root.Token) == "" {
		return result, nil
	}

	rootChildren, err := p.api.ListFiles(ctx, state.Root.Token)
	switch {
	case err == nil:
	case isPreviewResourceMissingError(err):
		result.RootMissing = true
		result.RemoteMissingScopeCount = len(state.Scopes)
		result.RemoteMissingFileCount = len(state.Files)
		return result, nil
	default:
		return PreviewDriveReconcileResult{}, err
	}

	knownScopeTokens := map[string]string{}
	filesByScope := map[string]map[string]*previewFileRecord{}
	for scopeKey, scope := range state.Scopes {
		if scope == nil || scope.Folder == nil {
			continue
		}
		if token := strings.TrimSpace(scope.Folder.Token); token != "" {
			knownScopeTokens[token] = scopeKey
		}
	}
	for _, record := range state.Files {
		if record == nil {
			continue
		}
		scopeKey := strings.TrimSpace(record.ScopeKey)
		if scopeKey == "" {
			continue
		}
		if filesByScope[scopeKey] == nil {
			filesByScope[scopeKey] = map[string]*previewFileRecord{}
		}
		if token := strings.TrimSpace(record.Token); token != "" {
			filesByScope[scopeKey][token] = record
		}
	}

	rootFolders := map[string]previewRemoteNode{}
	for _, node := range rootChildren {
		switch strings.TrimSpace(node.Type) {
		case previewFolderType:
			rootFolders[node.Token] = node
			if knownScopeTokens[node.Token] == "" {
				result.LocalOnlyScopeCount++
			}
		default:
			result.LocalOnlyFileCount++
		}
	}

	for scopeKey, scope := range state.Scopes {
		if scope == nil || scope.Folder == nil {
			continue
		}
		scopeToken := strings.TrimSpace(scope.Folder.Token)
		if scopeToken == "" {
			result.RemoteMissingScopeCount++
			continue
		}
		if _, ok := rootFolders[scopeToken]; !ok {
			result.RemoteMissingScopeCount++
			continue
		}
		if len(scope.Folder.Shared) > 0 {
			drift, err := previewPermissionDrift(ctx, p.api, scopeToken, previewFolderType, scope.Folder.Shared)
			if err != nil {
				if isPreviewResourceMissingError(err) {
					result.RemoteMissingScopeCount++
					continue
				}
				return PreviewDriveReconcileResult{}, err
			}
			if drift {
				result.PermissionDriftCount++
			}
		}

		scopeChildren, err := p.api.ListFiles(ctx, scopeToken)
		if err != nil {
			if isPreviewResourceMissingError(err) {
				result.RemoteMissingScopeCount++
				continue
			}
			return PreviewDriveReconcileResult{}, err
		}
		expectedFiles := filesByScope[scopeKey]
		remoteFiles := map[string]previewRemoteNode{}
		for _, node := range scopeChildren {
			switch strings.TrimSpace(node.Type) {
			case previewFileType:
				remoteFiles[node.Token] = node
				if expectedFiles[node.Token] == nil {
					result.LocalOnlyFileCount++
				}
			case previewFolderType:
				result.LocalOnlyScopeCount++
			default:
				result.LocalOnlyFileCount++
			}
		}
		for token, record := range expectedFiles {
			if _, ok := remoteFiles[token]; !ok {
				result.RemoteMissingFileCount++
				continue
			}
			if len(record.Shared) == 0 {
				continue
			}
			drift, err := previewPermissionDrift(ctx, p.api, token, previewFileType, record.Shared)
			if err != nil {
				if isPreviewResourceMissingError(err) {
					result.RemoteMissingFileCount++
					continue
				}
				return PreviewDriveReconcileResult{}, err
			}
			if drift {
				result.PermissionDriftCount++
			}
		}
	}

	return result, nil
}

func summarizePreviewState(state *previewState, statePath string) PreviewDriveSummary {
	state = normalizePreviewState(state)
	summary := PreviewDriveSummary{
		StatePath:  statePath,
		FileCount:  len(state.Files),
		ScopeCount: len(state.Scopes),
	}
	if state.Root != nil {
		summary.RootToken = state.Root.Token
		summary.RootURL = state.Root.URL
	}
	for _, record := range state.Files {
		if record == nil {
			continue
		}
		if record.SizeBytes > 0 {
			summary.EstimatedBytes += record.SizeBytes
		} else {
			summary.UnknownSizeFileCount++
		}
		if lastUsedAt, ok := previewRecordLastUsedAt(record); ok {
			value := lastUsedAt.UTC()
			if summary.OldestLastUsedAt == nil || value.Before(*summary.OldestLastUsedAt) {
				copyValue := value
				summary.OldestLastUsedAt = &copyValue
			}
			if summary.NewestLastUsedAt == nil || value.After(*summary.NewestLastUsedAt) {
				copyValue := value
				summary.NewestLastUsedAt = &copyValue
			}
		}
	}
	return summary
}

func previewRecordLastUsedAt(record *previewFileRecord) (time.Time, bool) {
	if record == nil {
		return time.Time{}, false
	}
	switch {
	case !record.LastUsedAt.IsZero():
		return record.LastUsedAt.UTC(), true
	case !record.CreatedAt.IsZero():
		return record.CreatedAt.UTC(), true
	default:
		return time.Time{}, false
	}
}

func previewRecordScopeKey(fileKey string) string {
	fileKey = strings.TrimSpace(fileKey)
	if fileKey == "" {
		return ""
	}
	parts := strings.SplitN(fileKey, "|", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func previewPermissionDrift(ctx context.Context, api previewDriveAPI, token, docType string, expected map[string]bool) (bool, error) {
	if api == nil || len(expected) == 0 {
		return false, nil
	}
	actual, err := api.ListPermissionMembers(ctx, token, docType)
	if err != nil {
		return false, err
	}
	for key, wanted := range expected {
		if !wanted {
			continue
		}
		if !actual[key] {
			return true, nil
		}
	}
	return false, nil
}

func clearPreviewScope(state *previewState, scopeKey string) {
	if state == nil {
		return
	}
	delete(state.Scopes, scopeKey)
	for key, record := range state.Files {
		if record == nil {
			delete(state.Files, key)
			continue
		}
		if strings.HasPrefix(key, scopeKey+"|") {
			delete(state.Files, key)
		}
	}
}

func ensurePreviewPermissions(ctx context.Context, api previewDriveAPI, token, docType string, shared *map[string]bool, principals []previewPrincipal) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("missing preview token for %s", docType)
	}
	if len(principals) == 0 {
		return nil
	}
	if *shared == nil {
		*shared = map[string]bool{}
	}

	available := 0
	var errs []string
	var firstErr error
	for _, principal := range principals {
		if principal.Key == "" || principal.MemberType == "" || principal.MemberID == "" || principal.Type == "" {
			continue
		}
		if (*shared)[principal.Key] {
			available++
			continue
		}
		if err := api.GrantPermission(ctx, token, docType, principal); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			errs = append(errs, fmt.Sprintf("%s => %v", principal.Key, err))
			continue
		}
		(*shared)[principal.Key] = true
		available++
	}
	if available > 0 {
		return nil
	}
	if firstErr != nil && isPreviewResourceMissingError(firstErr) {
		return firstErr
	}
	if len(errs) == 0 {
		return fmt.Errorf("no preview principals available")
	}
	return errors.New(strings.Join(errs, "; "))
}

func previewPrincipals(surfaceSessionID, chatID, actorUserID string) []previewPrincipal {
	seen := map[string]bool{}
	values := []previewPrincipal{}

	if userPrincipal, ok := previewUserPrincipal(actorUserID); ok {
		seen[userPrincipal.Key] = true
		values = append(values, userPrincipal)
	}
	if ref, ok := ParseSurfaceRef(surfaceSessionID); ok && ref.ScopeKind == ScopeKindChat {
		if chatPrincipal, ok := previewChatPrincipal(chatID); ok && !seen[chatPrincipal.Key] {
			seen[chatPrincipal.Key] = true
			values = append(values, chatPrincipal)
		}
	}
	return values
}

func previewUserPrincipal(actorUserID string) (previewPrincipal, bool) {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return previewPrincipal{}, false
	}
	memberType := "userid"
	switch {
	case strings.HasPrefix(actorUserID, "ou_"):
		memberType = "openid"
	case strings.HasPrefix(actorUserID, "on_"):
		memberType = "unionid"
	}
	return previewPrincipal{
		Key:        memberType + ":" + actorUserID,
		MemberType: memberType,
		MemberID:   actorUserID,
		Type:       "user",
	}, true
}

func previewChatPrincipal(chatID string) (previewPrincipal, bool) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return previewPrincipal{}, false
	}
	return previewPrincipal{
		Key:        "openchat:" + chatID,
		MemberType: "openchat",
		MemberID:   chatID,
		Type:       "chat",
	}, true
}

func previewScopeKey(gatewayID, surfaceSessionID, chatID, actorUserID string) string {
	if strings.TrimSpace(surfaceSessionID) != "" {
		return surfaceSessionID
	}
	gatewayID = normalizeGatewayID(gatewayID)
	if strings.TrimSpace(chatID) != "" {
		return SurfaceRef{
			Platform:  PlatformFeishu,
			GatewayID: gatewayID,
			ScopeKind: ScopeKindChat,
			ScopeID:   strings.TrimSpace(chatID),
		}.SurfaceID()
	}
	return SurfaceRef{
		Platform:  PlatformFeishu,
		GatewayID: gatewayID,
		ScopeKind: ScopeKindUser,
		ScopeID:   strings.TrimSpace(actorUserID),
	}.SurfaceID()
}

func previewScopeFolderName(scopeKey string) string {
	name := strings.NewReplacer(":", "-", "/", "-", "\\", "-", " ", "-").Replace(strings.TrimSpace(scopeKey))
	name = strings.Trim(name, "-")
	if name == "" {
		name = "feishu-preview-scope"
	}
	return limitNameBytes(name, 120)
}

func previewFileKey(scopeKey, resolvedPath, contentSHA string) string {
	return scopeKey + "|" + resolvedPath + "|" + contentSHA
}

func previewFileName(resolvedPath, contentSHA string) string {
	base := filepath.Base(resolvedPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	shortSHA := contentSHA
	if len(shortSHA) > 8 {
		shortSHA = shortSHA[:8]
	}
	name = limitNameBytes(name, 200-len(ext)-len(shortSHA)-2)
	if name == "" {
		name = "preview"
	}
	return name + "--" + shortSHA + ext
}

func limitNameBytes(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	for len(value) > limit {
		value = value[:len(value)-1]
	}
	return strings.TrimRight(value, "-_.")
}

func stripMarkdownLocationSuffix(target string) (string, string) {
	if idx := strings.Index(target, "#"); idx >= 0 {
		base := target[:idx]
		suffix := target[idx:]
		if matched, _ := regexp.MatchString(`^#L\d+(?:C\d+)?$`, suffix); matched {
			return base, suffix
		}
	}
	if matched := markdownLineSuffixPattern.FindStringSubmatch(target); len(matched) == 3 {
		return matched[1], matched[2]
	}
	return target, ""
}

func previewAllowedRoots(values ...string) []string {
	seen := map[string]bool{}
	roots := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		resolved, err := filepath.Abs(value)
		if err != nil {
			continue
		}
		resolved = filepath.Clean(resolved)
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		roots = append(roots, resolved)
	}
	return roots
}

func previewPathCandidates(target string, roots []string) []string {
	if filepath.IsAbs(target) {
		return []string{target}
	}
	candidates := make([]string, 0, len(roots))
	for _, root := range roots {
		candidates = append(candidates, filepath.Join(root, target))
	}
	return candidates
}

func previewCanonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if os.IsNotExist(err) {
		return "", err
	}
	return absolute, nil
}

func previewPathWithinAnyRoot(path string, roots []string) bool {
	if len(roots) == 0 {
		return false
	}
	for _, root := range roots {
		if previewPathWithinRoot(path, root) {
			return true
		}
	}
	return false
}

func previewPathWithinRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isPreviewResourceMissingError(err error) bool {
	var apiErr *driveAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case 1061003, 1061007, 1061041, 1061044, 1063005:
		return true
	default:
		return false
	}
}

func isPreviewParentMissingError(err error) bool {
	var apiErr *driveAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case 1061003, 1061007, 1061041, 1061044:
		return true
	default:
		return false
	}
}

type larkDrivePreviewAPI struct {
	client *lark.Client
}

func NewLarkDrivePreviewAPI(client *lark.Client) previewDriveAPI {
	if client == nil {
		return nil
	}
	return &larkDrivePreviewAPI{client: client}
}

func (a *larkDrivePreviewAPI) CreateFolder(ctx context.Context, name, parentToken string) (previewRemoteNode, error) {
	resp, err := a.client.Drive.V1.File.CreateFolder(ctx, larkdrive.NewCreateFolderFileReqBuilder().
		Body(larkdrive.NewCreateFolderFileReqBodyBuilder().
			Name(name).
			FolderToken(parentToken).
			Build()).
		Build())
	if err != nil {
		return previewRemoteNode{}, err
	}
	if !resp.Success() {
		return previewRemoteNode{}, &driveAPIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return previewRemoteNode{}, fmt.Errorf("missing create folder response data")
	}
	return previewRemoteNode{
		Token: stringValue(resp.Data.Token),
		URL:   stringValue(resp.Data.Url),
		Type:  previewFolderType,
	}, nil
}

func (a *larkDrivePreviewAPI) UploadFile(ctx context.Context, parentToken, fileName string, content []byte) (string, error) {
	resp, err := a.client.Drive.V1.File.UploadAll(ctx, larkdrive.NewUploadAllFileReqBuilder().
		Body(larkdrive.NewUploadAllFileReqBodyBuilder().
			FileName(fileName).
			ParentType("explorer").
			ParentNode(parentToken).
			Size(len(content)).
			File(bytes.NewReader(content)).
			Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", &driveAPIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return "", fmt.Errorf("missing upload file response data")
	}
	return stringValue(resp.Data.FileToken), nil
}

func (a *larkDrivePreviewAPI) QueryMetaURL(ctx context.Context, token, docType string) (string, error) {
	resp, err := a.client.Drive.V1.Meta.BatchQuery(ctx, larkdrive.NewBatchQueryMetaReqBuilder().
		MetaRequest(larkdrive.NewMetaRequestBuilder().
			RequestDocs([]*larkdrive.RequestDoc{
				larkdrive.NewRequestDocBuilder().
					DocToken(token).
					DocType(docType).
					Build(),
			}).
			WithUrl(true).
			Build()).
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", &driveAPIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil || len(resp.Data.Metas) == 0 || resp.Data.Metas[0] == nil {
		return "", fmt.Errorf("missing meta url for token %s", token)
	}
	return stringValue(resp.Data.Metas[0].Url), nil
}

func (a *larkDrivePreviewAPI) GrantPermission(ctx context.Context, token, docType string, principal previewPrincipal) error {
	resp, err := a.client.Drive.V1.PermissionMember.Create(ctx, larkdrive.NewCreatePermissionMemberReqBuilder().
		Token(token).
		Type(docType).
		BaseMember(larkdrive.NewBaseMemberBuilder().
			MemberType(principal.MemberType).
			MemberId(principal.MemberID).
			Perm(previewPermissionView).
			Type(principal.Type).
			Build()).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return &driveAPIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

func (a *larkDrivePreviewAPI) DeleteFile(ctx context.Context, token, docType string) error {
	resp, err := a.client.Drive.V1.File.Delete(ctx, larkdrive.NewDeleteFileReqBuilder().
		FileToken(token).
		Type(docType).
		Build())
	if err != nil {
		return err
	}
	if !resp.Success() {
		return &driveAPIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

func (a *larkDrivePreviewAPI) ListFiles(ctx context.Context, folderToken string) ([]previewRemoteNode, error) {
	values := []previewRemoteNode{}
	pageToken := ""
	for {
		req := larkdrive.NewListFileReqBuilder().
			FolderToken(folderToken).
			PageSize(200)
		if strings.TrimSpace(pageToken) != "" {
			req.PageToken(pageToken)
		}
		resp, err := a.client.Drive.V1.File.List(ctx, req.Build())
		if err != nil {
			return nil, err
		}
		if !resp.Success() {
			return nil, &driveAPIError{Code: resp.Code, Msg: resp.Msg}
		}
		if resp.Data != nil {
			for _, file := range resp.Data.Files {
				if file == nil {
					continue
				}
				values = append(values, previewRemoteNode{
					Token: stringValue(file.Token),
					URL:   stringValue(file.Url),
					Type:  strings.TrimSpace(stringValue(file.Type)),
				})
			}
			if resp.Data.HasMore != nil && *resp.Data.HasMore && strings.TrimSpace(stringValue(resp.Data.NextPageToken)) != "" {
				pageToken = stringValue(resp.Data.NextPageToken)
				continue
			}
		}
		break
	}
	return values, nil
}

func (a *larkDrivePreviewAPI) ListPermissionMembers(ctx context.Context, token, docType string) (map[string]bool, error) {
	resp, err := a.client.Drive.V1.PermissionMember.List(ctx, larkdrive.NewListPermissionMemberReqBuilder().
		Token(token).
		Type(docType).
		Build())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, &driveAPIError{Code: resp.Code, Msg: resp.Msg}
	}
	values := map[string]bool{}
	if resp.Data == nil {
		return values, nil
	}
	for _, item := range resp.Data.Items {
		if item == nil {
			continue
		}
		memberType := strings.TrimSpace(stringValue(item.MemberType))
		memberID := strings.TrimSpace(stringValue(item.MemberId))
		if memberType == "" || memberID == "" {
			continue
		}
		values[memberType+":"+memberID] = true
	}
	return values, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
