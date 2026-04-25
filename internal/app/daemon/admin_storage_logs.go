package daemon

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type logsStorageStatusResponse struct {
	RootDir      string     `json:"rootDir"`
	FileCount    int        `json:"fileCount"`
	TotalBytes   int64      `json:"totalBytes"`
	LatestFileAt *time.Time `json:"latestFileAt,omitempty"`
}

type logsStorageCleanupRequest struct {
	OlderThanHours int `json:"olderThanHours,omitempty"`
}

type logsStorageCleanupResponse struct {
	RootDir            string `json:"rootDir"`
	OlderThanHours     int    `json:"olderThanHours"`
	DeletedFiles       int    `json:"deletedFiles"`
	DeletedBytes       int64  `json:"deletedBytes"`
	RemainingFileCount int    `json:"remainingFileCount"`
	RemainingBytes     int64  `json:"remainingBytes"`
}

type logsStorageEntry struct {
	Path    string
	Size    int64
	ModTime time.Time
}

type logsStorageScan struct {
	FileCount    int
	TotalBytes   int64
	LatestFileAt time.Time
	Entries      []logsStorageEntry
}

func (a *App) handleLogsStorageStatus(w http.ResponseWriter, _ *http.Request) {
	rootDir := strings.TrimSpace(a.headlessRuntime.Paths.LogsDir)
	scan, err := scanLogsStorage(rootDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    "logs_storage_scan_failed",
			Message: "failed to scan logs directory",
			Details: err.Error(),
		})
		return
	}
	resp := logsStorageStatusResponse{
		RootDir:    rootDir,
		FileCount:  scan.FileCount,
		TotalBytes: scan.TotalBytes,
	}
	if !scan.LatestFileAt.IsZero() {
		latest := scan.LatestFileAt
		resp.LatestFileAt = &latest
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleLogsStorageCleanup(w http.ResponseWriter, r *http.Request) {
	req := logsStorageCleanupRequest{OlderThanHours: defaultImageStagingCleanupHours}
	if r.Body != nil && r.Body != http.NoBody {
		defer r.Body.Close()
		if body, err := io.ReadAll(r.Body); err != nil {
			writeAPIError(w, http.StatusBadRequest, apiError{
				Code:    "invalid_request",
				Message: "failed to read logs cleanup payload",
				Details: err.Error(),
			})
			return
		} else if len(strings.TrimSpace(string(body))) > 0 {
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			if err := decodeJSONBody(r, &req); err != nil && !errors.Is(err, io.EOF) {
				writeAPIError(w, http.StatusBadRequest, apiError{
					Code:    "invalid_request",
					Message: "failed to decode logs cleanup payload",
					Details: err.Error(),
				})
				return
			}
		}
	}
	if req.OlderThanHours <= 0 {
		req.OlderThanHours = defaultImageStagingCleanupHours
	}

	rootDir := strings.TrimSpace(a.headlessRuntime.Paths.LogsDir)
	scan, err := scanLogsStorage(rootDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    "logs_storage_scan_failed",
			Message: "failed to scan logs directory",
			Details: err.Error(),
		})
		return
	}

	cutoff := time.Now().Add(-time.Duration(req.OlderThanHours) * time.Hour)
	resp := logsStorageCleanupResponse{
		RootDir:        rootDir,
		OlderThanHours: req.OlderThanHours,
	}
	for _, entry := range scan.Entries {
		if entry.ModTime.After(cutoff) {
			continue
		}
		if err := os.Remove(entry.Path); err != nil && !os.IsNotExist(err) {
			writeAPIError(w, http.StatusInternalServerError, apiError{
				Code:    "logs_storage_cleanup_failed",
				Message: "failed to remove log file",
				Details: err.Error(),
			})
			return
		}
		resp.DeletedFiles++
		resp.DeletedBytes += entry.Size
	}
	_ = pruneEmptyDirectories(rootDir)

	remaining, err := scanLogsStorage(rootDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apiError{
			Code:    "logs_storage_scan_failed",
			Message: "failed to rescan logs directory",
			Details: err.Error(),
		})
		return
	}
	resp.RemainingFileCount = remaining.FileCount
	resp.RemainingBytes = remaining.TotalBytes
	writeJSON(w, http.StatusOK, resp)
}

func scanLogsStorage(rootDir string) (logsStorageScan, error) {
	var scan logsStorageScan
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return scan, nil
	}
	if _, err := os.Stat(rootDir); err != nil {
		if os.IsNotExist(err) {
			return scan, nil
		}
		return scan, err
	}
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d == nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entry := logsStorageEntry{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		scan.FileCount++
		scan.TotalBytes += entry.Size
		if entry.ModTime.After(scan.LatestFileAt) {
			scan.LatestFileAt = entry.ModTime
		}
		scan.Entries = append(scan.Entries, entry)
		return nil
	})
	return scan, err
}
