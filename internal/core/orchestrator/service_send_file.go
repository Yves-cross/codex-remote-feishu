package orchestrator

import (
	"path/filepath"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

const sendFilePathPickerConsumerKind = "send_file"

type sendFilePathPickerConsumer struct{}

func (sendFilePathPickerConsumer) PathPickerConfirmed(_ *Service, surface *state.SurfaceConsoleRecord, result control.PathPickerResult) []control.UIEvent {
	if surface == nil {
		return nil
	}
	selectedPath := strings.TrimSpace(result.SelectedPath)
	if selectedPath == "" {
		return notice(surface, "send_file_invalid", "未选中文件，请重新选择。")
	}
	return []control.UIEvent{{
		Kind:             control.UIEventDaemonCommand,
		GatewayID:        surface.GatewayID,
		SurfaceSessionID: surface.SurfaceSessionID,
		DaemonCommand: &control.DaemonCommand{
			Kind:             control.DaemonCommandSendIMFile,
			GatewayID:        surface.GatewayID,
			SurfaceSessionID: surface.SurfaceSessionID,
			LocalPath:        selectedPath,
		},
	}}
}

func (sendFilePathPickerConsumer) PathPickerCancelled(_ *Service, surface *state.SurfaceConsoleRecord, _ control.PathPickerResult) []control.UIEvent {
	return notice(surface, "send_file_cancelled", "已取消发送文件。")
}

func (s *Service) openSendFilePicker(surface *state.SurfaceConsoleRecord) []control.UIEvent {
	if surface == nil {
		return nil
	}
	if s.normalizeSurfaceProductMode(surface) != state.ProductModeNormal {
		return notice(surface, "send_file_normal_only", "当前处于 vscode 模式，暂不支持从飞书选择文件发送。请先 `/mode normal`。")
	}
	if strings.TrimSpace(surface.AttachedInstanceID) == "" {
		return notice(surface, "send_file_requires_workspace", "当前还没有接管工作区。请先 `/list` 选择工作区，然后再发送文件。")
	}
	workspaceKey := s.surfaceCurrentWorkspaceKey(surface)
	if workspaceKey == "" {
		return notice(surface, "send_file_requires_workspace", "当前还没有可用的工作区路径，请重新 `/list` 选择工作区后再试。")
	}
	if inst := s.root.Instances[surface.AttachedInstanceID]; inst != nil {
		if root := strings.TrimSpace(inst.WorkspaceRoot); root != "" {
			workspaceKey = root
		}
	}
	return s.openPathPicker(surface, surface.ActorUserID, control.PathPickerRequest{
		Mode:         control.PathPickerModeFile,
		Title:        "选择要发送的文件",
		RootPath:     workspaceKey,
		InitialPath:  filepath.Clean(workspaceKey),
		ConfirmLabel: "发送到当前聊天",
		CancelLabel:  "取消",
		ConsumerKind: sendFilePathPickerConsumerKind,
	})
}
