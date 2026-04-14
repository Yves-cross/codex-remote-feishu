package state

import (
	"time"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

type ActiveTargetPickerRecord struct {
	PickerID             string
	OwnerUserID          string
	Source               control.TargetPickerRequestSource
	SelectedWorkspaceKey string
	SelectedSessionValue string
	CreatedAt            time.Time
	ExpiresAt            time.Time
}
