package state

import "time"

type PathPickerMode string

const (
	PathPickerModeDirectory PathPickerMode = "directory"
	PathPickerModeFile      PathPickerMode = "file"
)

type ActivePathPickerRecord struct {
	PickerID     string
	OwnerUserID  string
	Mode         PathPickerMode
	Title        string
	RootPath     string
	CurrentPath  string
	SelectedPath string
	ConfirmLabel string
	CancelLabel  string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ConsumerKind string
	ConsumerMeta map[string]string
}
