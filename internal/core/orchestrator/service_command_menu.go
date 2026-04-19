package orchestrator

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

type commandMenuStage string

const (
	commandMenuStageDetached      commandMenuStage = commandMenuStage(control.FeishuCommandMenuStageDetached)
	commandMenuStageNormalWorking commandMenuStage = commandMenuStage(control.FeishuCommandMenuStageNormalWorking)
	commandMenuStageVSCodeWorking commandMenuStage = commandMenuStage(control.FeishuCommandMenuStageVSCodeWorking)
)

func (s *Service) buildCommandMenuCatalog(surface *state.SurfaceConsoleRecord, raw string) control.FeishuDirectCommandCatalog {
	return s.commandCatalogFromView(surface, s.buildCommandMenuView(surface, raw))
}

func parseCommandMenuView(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) < 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(fields[1]))
}

func (s *Service) commandMenuStage(surface *state.SurfaceConsoleRecord) commandMenuStage {
	if surface == nil || strings.TrimSpace(surface.AttachedInstanceID) == "" {
		return commandMenuStageDetached
	}
	if s.normalizeSurfaceProductMode(surface) == state.ProductModeVSCode {
		return commandMenuStageVSCodeWorking
	}
	return commandMenuStageNormalWorking
}

func (s *Service) buildCommandMenuHomeCatalog(surface *state.SurfaceConsoleRecord) control.FeishuDirectCommandCatalog {
	return control.BuildFeishuCommandMenuHomeCatalog()
}

func (s *Service) buildCommandHelpCatalog(surface *state.SurfaceConsoleRecord) control.FeishuDirectCommandCatalog {
	return control.BuildFeishuCommandCatalogForDisplay(
		"Slash 命令帮助",
		"以下是当前主展示的 canonical slash command。历史 alias 仍可兼容，但不再作为新的主展示入口。",
		false,
		string(s.normalizeSurfaceProductMode(surface)),
		"",
	)
}

func (s *Service) buildCommandMenuGroupCatalog(surface *state.SurfaceConsoleRecord, stage commandMenuStage, groupID string) control.FeishuDirectCommandCatalog {
	return control.BuildFeishuCommandMenuGroupCatalog(groupID, string(s.normalizeSurfaceProductMode(surface)), string(stage))
}

func (s *Service) buildModeCatalog(surface *state.SurfaceConsoleRecord) control.FeishuDirectCommandCatalog {
	return s.commandCatalogFromView(surface, s.buildModeCommandView(surface))
}

func (s *Service) buildAutoContinueCatalog(surface *state.SurfaceConsoleRecord) control.FeishuDirectCommandCatalog {
	return s.commandCatalogFromView(surface, s.buildAutoContinueCommandView(surface))
}

func (s *Service) buildReasoningCatalog(surface *state.SurfaceConsoleRecord) control.FeishuDirectCommandCatalog {
	return s.commandCatalogFromView(surface, s.buildReasoningCommandView(surface))
}

func (s *Service) buildAccessCatalog(surface *state.SurfaceConsoleRecord) control.FeishuDirectCommandCatalog {
	return s.commandCatalogFromView(surface, s.buildAccessCommandView(surface))
}

func (s *Service) buildModelCatalog(surface *state.SurfaceConsoleRecord) control.FeishuDirectCommandCatalog {
	return s.commandCatalogFromView(surface, s.buildModelCommandView(surface))
}

func choiceCommandButton(label, commandText string, disabled bool, style string) control.CommandCatalogButton {
	return control.CommandCatalogButton{
		Label:       label,
		Kind:        control.CommandCatalogButtonRunCommand,
		CommandText: commandText,
		Style:       style,
		Disabled:    disabled,
	}
}

func choiceButtonsFromOptions(options []control.FeishuCommandOption, currentOverride, primaryValue string) []control.CommandCatalogButton {
	buttons := make([]control.CommandCatalogButton, 0, len(options))
	currentOverride = strings.TrimSpace(currentOverride)
	for _, option := range options {
		value := strings.TrimSpace(option.Value)
		if value == "" {
			continue
		}
		style := ""
		if value == primaryValue {
			style = "primary"
		}
		disabled := false
		switch value {
		case "clear":
			disabled = currentOverride == ""
		default:
			disabled = currentOverride != "" && currentOverride == value
		}
		label := strings.TrimSpace(option.Label)
		if disabled && value != "clear" {
			label += "（当前）"
			style = "primary"
		}
		buttons = append(buttons, control.CommandCatalogButton{
			Label:       label,
			Kind:        control.CommandCatalogButtonRunCommand,
			CommandText: option.CommandText,
			Style:       style,
			Disabled:    disabled,
		})
	}
	return buttons
}

func (s *Service) startCommandCapture(surface *state.SurfaceConsoleRecord, action control.Action) []control.UIEvent {
	if surface == nil {
		return nil
	}
	if surface.ActiveRequestCapture != nil {
		return notice(surface, "request_capture_waiting_text", "当前正在等待你发送一条文字处理意见，请先发送文本或重新处理确认卡片。")
	}
	if pending := activePendingRequest(surface); pending != nil {
		return notice(surface, "request_pending", pendingRequestNoticeText(activePendingRequest(surface)))
	}
	switch action.CommandID {
	case control.FeishuCommandModel:
		clearSurfaceCommandCapture(surface)
		return []control.UIEvent{s.commandViewEvent(surface, s.buildModelCommandView(surface))}
	default:
		return notice(surface, "command_capture_unsupported", "这个命令暂不支持 capture/apply 输入。")
	}
}

func (s *Service) cancelCommandCapture(surface *state.SurfaceConsoleRecord, action control.Action) []control.UIEvent {
	if surface == nil {
		return nil
	}
	clearSurfaceCommandCapture(surface)
	switch action.CommandID {
	case control.FeishuCommandModel:
		return []control.UIEvent{s.commandViewEvent(surface, s.buildModelCommandView(surface))}
	default:
		return nil
	}
}

func (s *Service) consumeCapturedCommandInput(surface *state.SurfaceConsoleRecord, text string) []control.UIEvent {
	if surface == nil {
		return nil
	}
	capture := surface.ActiveCommandCapture
	if commandCaptureExpired(s.now(), capture) {
		clearSurfaceCommandCapture(surface)
		return notice(surface, "command_capture_expired", "上一条命令输入已过期，请重新打开 `/model` 卡片后再提交。")
	}
	clearSurfaceCommandCapture(surface)
	switch capture.CommandID {
	case control.FeishuCommandModel:
		text = strings.TrimSpace(text)
		if text == "" {
			return notice(surface, "command_capture_empty", "没有收到可用输入，请重新打开模型卡片后提交。")
		}
		return s.handleModelCommand(surface, control.Action{
			Kind:             control.ActionModelCommand,
			GatewayID:        surface.GatewayID,
			SurfaceSessionID: surface.SurfaceSessionID,
			ChatID:           surface.ChatID,
			ActorUserID:      surface.ActorUserID,
			Text:             "/model " + text,
		})
	default:
		return notice(surface, "command_capture_unsupported", "当前命令输入已失效，请重新打开命令卡片。")
	}
}
