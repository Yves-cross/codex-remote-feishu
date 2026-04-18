package orchestrator

import (
	"fmt"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func (s *Service) buildCommandMenuView(surface *state.SurfaceConsoleRecord, raw string) control.FeishuCommandView {
	return control.FeishuCommandView{
		Menu: &control.FeishuCommandMenuView{
			Stage:   string(s.commandMenuStage(surface)),
			GroupID: parseCommandMenuView(raw),
		},
	}
}

func (s *Service) buildModeCommandView(surface *state.SurfaceConsoleRecord) control.FeishuCommandView {
	return s.buildModeCommandViewState(surface, control.FeishuCommandConfigView{})
}

func (s *Service) buildModeCommandViewState(surface *state.SurfaceConsoleRecord, cardState control.FeishuCommandConfigView) control.FeishuCommandView {
	return control.FeishuCommandView{
		Config: s.applyCommandConfigCardState(&control.FeishuCommandConfigView{
			CommandID:    control.FeishuCommandMode,
			CurrentValue: string(s.normalizeSurfaceProductMode(surface)),
		}, cardState),
	}
}

func (s *Service) buildAutoContinueCommandView(surface *state.SurfaceConsoleRecord) control.FeishuCommandView {
	return s.buildAutoContinueCommandViewState(surface, control.FeishuCommandConfigView{})
}

func (s *Service) buildAutoContinueCommandViewState(surface *state.SurfaceConsoleRecord, cardState control.FeishuCommandConfigView) control.FeishuCommandView {
	current := "off"
	if surface != nil && surface.AutoContinue.Enabled {
		current = "on"
	}
	return control.FeishuCommandView{
		Config: s.applyCommandConfigCardState(&control.FeishuCommandConfigView{
			CommandID:    control.FeishuCommandAutoContinue,
			CurrentValue: current,
		}, cardState),
	}
}

func (s *Service) buildReasoningCommandView(surface *state.SurfaceConsoleRecord) control.FeishuCommandView {
	return s.buildReasoningCommandViewState(surface, control.FeishuCommandConfigView{})
}

func (s *Service) buildReasoningCommandViewState(surface *state.SurfaceConsoleRecord, cardState control.FeishuCommandConfigView) control.FeishuCommandView {
	view := control.FeishuCommandView{
		Config: s.applyCommandConfigCardState(&control.FeishuCommandConfigView{CommandID: control.FeishuCommandReasoning}, cardState),
	}
	attachedInstanceID := ""
	if surface != nil {
		attachedInstanceID = strings.TrimSpace(surface.AttachedInstanceID)
	}
	inst := s.root.Instances[attachedInstanceID]
	if inst == nil {
		view.Config.RequiresAttachment = true
		return view
	}
	summary := s.resolveNextPromptSummary(inst, surface, "", "", state.ModelConfigRecord{})
	view.Config.EffectiveValue = strings.TrimSpace(summary.EffectiveReasoningEffort)
	view.Config.OverrideValue = strings.TrimSpace(summary.OverrideReasoningEffort)
	return view
}

func (s *Service) buildAccessCommandView(surface *state.SurfaceConsoleRecord) control.FeishuCommandView {
	return s.buildAccessCommandViewState(surface, control.FeishuCommandConfigView{})
}

func (s *Service) buildAccessCommandViewState(surface *state.SurfaceConsoleRecord, cardState control.FeishuCommandConfigView) control.FeishuCommandView {
	view := control.FeishuCommandView{
		Config: s.applyCommandConfigCardState(&control.FeishuCommandConfigView{CommandID: control.FeishuCommandAccess}, cardState),
	}
	attachedInstanceID := ""
	if surface != nil {
		attachedInstanceID = strings.TrimSpace(surface.AttachedInstanceID)
	}
	inst := s.root.Instances[attachedInstanceID]
	if inst == nil {
		view.Config.RequiresAttachment = true
		return view
	}
	summary := s.resolveNextPromptSummary(inst, surface, "", "", state.ModelConfigRecord{})
	view.Config.EffectiveValue = strings.TrimSpace(summary.EffectiveAccessMode)
	view.Config.OverrideValue = strings.TrimSpace(summary.OverrideAccessMode)
	return view
}

func (s *Service) buildModelCommandView(surface *state.SurfaceConsoleRecord) control.FeishuCommandView {
	return s.buildModelCommandViewState(surface, control.FeishuCommandConfigView{})
}

func (s *Service) buildModelCommandViewState(surface *state.SurfaceConsoleRecord, cardState control.FeishuCommandConfigView) control.FeishuCommandView {
	view := control.FeishuCommandView{
		Config: s.applyCommandConfigCardState(&control.FeishuCommandConfigView{CommandID: control.FeishuCommandModel}, cardState),
	}
	attachedInstanceID := ""
	if surface != nil {
		attachedInstanceID = strings.TrimSpace(surface.AttachedInstanceID)
	}
	inst := s.root.Instances[attachedInstanceID]
	if inst == nil {
		view.Config.RequiresAttachment = true
		return view
	}
	summary := s.resolveNextPromptSummary(inst, surface, "", "", state.ModelConfigRecord{})
	view.Config.EffectiveValue = strings.TrimSpace(summary.EffectiveModel)
	view.Config.OverrideValue = strings.TrimSpace(summary.OverrideModel)
	view.Config.OverrideExtraValue = strings.TrimSpace(summary.OverrideReasoningEffort)
	return view
}

func (s *Service) buildVerboseCommandView(surface *state.SurfaceConsoleRecord) control.FeishuCommandView {
	return s.buildVerboseCommandViewState(surface, control.FeishuCommandConfigView{})
}

func (s *Service) buildVerboseCommandViewState(surface *state.SurfaceConsoleRecord, cardState control.FeishuCommandConfigView) control.FeishuCommandView {
	current := state.SurfaceVerbosityNormal
	if surface != nil {
		current = state.NormalizeSurfaceVerbosity(surface.Verbosity)
	}
	return control.FeishuCommandView{
		Config: s.applyCommandConfigCardState(&control.FeishuCommandConfigView{
			CommandID:    control.FeishuCommandVerbose,
			CurrentValue: string(current),
		}, cardState),
	}
}

func (s *Service) applyCommandConfigCardState(base *control.FeishuCommandConfigView, cardState control.FeishuCommandConfigView) *control.FeishuCommandConfigView {
	if base == nil {
		base = &control.FeishuCommandConfigView{}
	}
	if strings.TrimSpace(cardState.FormDefaultValue) != "" {
		base.FormDefaultValue = strings.TrimSpace(cardState.FormDefaultValue)
	}
	if strings.TrimSpace(cardState.StatusKind) != "" {
		base.StatusKind = strings.TrimSpace(cardState.StatusKind)
	}
	if strings.TrimSpace(cardState.StatusText) != "" {
		base.StatusText = strings.TrimSpace(cardState.StatusText)
	}
	if cardState.Sealed {
		base.Sealed = true
	}
	return base
}

func (s *Service) commandCatalogFromView(surface *state.SurfaceConsoleRecord, view control.FeishuCommandView) control.FeishuDirectCommandCatalog {
	switch {
	case view.Menu != nil:
		return s.commandMenuCatalogFromView(surface, *view.Menu)
	case view.Config != nil:
		return s.commandConfigCatalogFromView(*view.Config)
	default:
		return control.FeishuDirectCommandCatalog{}
	}
}

func (s *Service) commandMenuCatalogFromView(surface *state.SurfaceConsoleRecord, view control.FeishuCommandMenuView) control.FeishuDirectCommandCatalog {
	stage := commandMenuStage(strings.TrimSpace(view.Stage))
	groupID := strings.TrimSpace(view.GroupID)
	if groupID == "" {
		return s.buildCommandMenuHomeCatalog(surface)
	}
	return s.buildCommandMenuGroupCatalog(surface, stage, groupID)
}

func (s *Service) commandConfigCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	switch strings.TrimSpace(view.CommandID) {
	case control.FeishuCommandMode:
		return modeCatalogFromView(view)
	case control.FeishuCommandAutoContinue:
		return autoContinueCatalogFromView(view)
	case control.FeishuCommandReasoning:
		return reasoningCatalogFromView(view)
	case control.FeishuCommandAccess:
		return accessCatalogFromView(view)
	case control.FeishuCommandModel:
		return modelCatalogFromView(view)
	case control.FeishuCommandVerbose:
		return verboseCatalogFromView(view)
	default:
		return control.FeishuDirectCommandCatalog{}
	}
}

func modeCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandMode)
	baseSummary := fmt.Sprintf("当前模式：`%s`。", displayPromptValue(strings.TrimSpace(view.CurrentValue), "未设置"))
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, baseSummary, view)
	}
	sections := []control.CommandCatalogSection{{
		Title: "立即切换",
		Entries: []control.CommandCatalogEntry{{
			Buttons: fixedChoiceButtonsFromOptions(def.Options, strings.TrimSpace(view.CurrentValue), "normal"),
		}},
	}}
	if form := commandFormWithViewDefault(control.FeishuCommandMode, view); form != nil {
		sections = append(sections, control.CommandCatalogSection{
			Title:   "手动输入",
			Entries: []control.CommandCatalogEntry{{Form: form}},
		})
	}
	return control.FeishuDirectCommandCatalog{
		Title:          def.Title,
		Summary:        commandSummaryWithFeedback(baseSummary, def, view),
		Interactive:    true,
		DisplayStyle:   control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:    commandBreadcrumbs(def.GroupID, def.Title),
		Sections:       sections,
		RelatedButtons: commandBackButtons(def.GroupID),
	}
}

func autoContinueCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandAutoContinue)
	statusText := "关闭"
	if strings.EqualFold(strings.TrimSpace(view.CurrentValue), "on") {
		statusText = "开启"
	}
	baseSummary := fmt.Sprintf("当前：`%s`。", statusText)
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, baseSummary, view)
	}
	sections := []control.CommandCatalogSection{{
		Title: "立即切换",
		Entries: []control.CommandCatalogEntry{{
			Buttons: fixedChoiceButtonsFromOptions(def.Options, strings.TrimSpace(view.CurrentValue), "on"),
		}},
	}}
	if form := commandFormWithViewDefault(control.FeishuCommandAutoContinue, view); form != nil {
		sections = append(sections, control.CommandCatalogSection{
			Title:   "手动输入",
			Entries: []control.CommandCatalogEntry{{Form: form}},
		})
	}
	return control.FeishuDirectCommandCatalog{
		Title:          def.Title,
		Summary:        commandSummaryWithFeedback(baseSummary, def, view),
		Interactive:    true,
		DisplayStyle:   control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:    commandBreadcrumbs(def.GroupID, def.Title),
		Sections:       sections,
		RelatedButtons: commandBackButtons(def.GroupID),
	}
}

func reasoningCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandReasoning)
	baseSummary := fmt.Sprintf("当前：`%s`；飞书覆盖：`%s`。", displayPromptValue(strings.TrimSpace(view.EffectiveValue), "未设置"), displayPromptValue(strings.TrimSpace(view.OverrideValue), "无"))
	if view.RequiresAttachment {
		return attachmentRequiredCatalogForDefinition(def, view)
	}
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, baseSummary, view)
	}
	sections := []control.CommandCatalogSection{{
		Title: "立即应用",
		Entries: []control.CommandCatalogEntry{{
			Buttons: choiceButtonsFromOptions(def.Options, strings.TrimSpace(view.OverrideValue), ""),
		}},
	}}
	if form := commandFormWithViewDefault(control.FeishuCommandReasoning, view); form != nil {
		sections = append(sections, control.CommandCatalogSection{
			Title:   "手动输入",
			Entries: []control.CommandCatalogEntry{{Form: form}},
		})
	}
	return control.FeishuDirectCommandCatalog{
		Title:          def.Title,
		Summary:        commandSummaryWithFeedback(baseSummary, def, view),
		Interactive:    true,
		DisplayStyle:   control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:    commandBreadcrumbs(def.GroupID, def.Title),
		Sections:       sections,
		RelatedButtons: commandBackButtons(def.GroupID),
	}
}

func accessCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandAccess)
	baseSummary := fmt.Sprintf("当前：`%s`；飞书覆盖：`%s`。", displayPromptValue(strings.TrimSpace(view.EffectiveValue), "未设置"), displayPromptValue(strings.TrimSpace(view.OverrideValue), "无"))
	if view.RequiresAttachment {
		return attachmentRequiredCatalogForDefinition(def, view)
	}
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, baseSummary, view)
	}
	sections := []control.CommandCatalogSection{{
		Title: "立即应用",
		Entries: []control.CommandCatalogEntry{{
			Buttons: choiceButtonsFromOptions(def.Options, strings.TrimSpace(view.OverrideValue), ""),
		}},
	}}
	if form := commandFormWithViewDefault(control.FeishuCommandAccess, view); form != nil {
		sections = append(sections, control.CommandCatalogSection{
			Title:   "手动输入",
			Entries: []control.CommandCatalogEntry{{Form: form}},
		})
	}
	return control.FeishuDirectCommandCatalog{
		Title:          def.Title,
		Summary:        commandSummaryWithFeedback(baseSummary, def, view),
		Interactive:    true,
		DisplayStyle:   control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:    commandBreadcrumbs(def.GroupID, def.Title),
		Sections:       sections,
		RelatedButtons: commandBackButtons(def.GroupID),
	}
}

func modelCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandModel)
	lines := []string{
		fmt.Sprintf("当前模型：`%s`；飞书覆盖：`%s`。", displayPromptValue(strings.TrimSpace(view.EffectiveValue), "未设置"), displayPromptValue(strings.TrimSpace(view.OverrideValue), "无")),
	}
	if strings.TrimSpace(view.OverrideExtraValue) != "" {
		lines = append(lines, fmt.Sprintf("附带推理覆盖：`%s`。", view.OverrideExtraValue))
	}
	baseSummary := strings.Join(lines, "\n")
	if view.RequiresAttachment {
		return attachmentRequiredCatalogForDefinition(def, view)
	}
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, baseSummary, view)
	}
	presetButtons := []control.CommandCatalogButton{
		choiceCommandButton("gpt-5.4", "/model gpt-5.4", strings.TrimSpace(view.OverrideValue) == "gpt-5.4", ""),
		choiceCommandButton("gpt-5.4-mini", "/model gpt-5.4-mini", strings.TrimSpace(view.OverrideValue) == "gpt-5.4-mini", ""),
	}
	manualEntry := control.CommandCatalogEntry{
		Form: commandFormWithViewDefault(control.FeishuCommandModel, view),
	}
	if strings.TrimSpace(view.OverrideValue) != "" || strings.TrimSpace(view.OverrideExtraValue) != "" {
		manualEntry.Buttons = append(manualEntry.Buttons, choiceCommandButton("清除覆盖", "/model clear", false, ""))
	}
	catalog := control.FeishuDirectCommandCatalog{
		Title:        def.Title,
		Interactive:  true,
		DisplayStyle: control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:  commandBreadcrumbs(def.GroupID, def.Title),
		Sections: []control.CommandCatalogSection{
			{
				Title: "常见模型",
				Entries: []control.CommandCatalogEntry{{
					Buttons: presetButtons,
				}},
			},
			{
				Title:   "手动输入",
				Entries: []control.CommandCatalogEntry{manualEntry},
			},
		},
		RelatedButtons: commandBackButtons(def.GroupID),
	}
	catalog.Summary = commandSummaryWithFeedback(baseSummary, def, view)
	return catalog
}

func attachmentRequiredCatalogForDefinition(def control.FeishuCommandDefinition, view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	baseSummary := "还没接管目标。先开始或继续工作，再回来调整这个参数。"
	return control.FeishuDirectCommandCatalog{
		Title:        def.Title,
		Summary:      commandSummaryWithFeedback(baseSummary, def, view),
		Interactive:  true,
		DisplayStyle: control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:  commandBreadcrumbs(def.GroupID, def.Title),
		Sections: []control.CommandCatalogSection{{
			Title: "开始 / 继续工作",
			Entries: []control.CommandCatalogEntry{
				recoveryEntry(control.FeishuCommandList),
				recoveryEntry(control.FeishuCommandUse),
				recoveryEntry(control.FeishuCommandStatus),
			},
		}},
		RelatedButtons: commandBackButtons(def.GroupID),
	}
}

func verboseCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandVerbose)
	current := strings.TrimSpace(view.CurrentValue)
	baseSummary := fmt.Sprintf("当前：`%s`。", displayPromptValue(current, "normal"))
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, baseSummary, view)
	}
	sections := []control.CommandCatalogSection{{
		Title: "立即切换",
		Entries: []control.CommandCatalogEntry{{
			Buttons: fixedChoiceButtonsFromOptions(def.Options, current, "normal"),
		}},
	}}
	if form := commandFormWithViewDefault(control.FeishuCommandVerbose, view); form != nil {
		sections = append(sections, control.CommandCatalogSection{
			Title:   "手动输入",
			Entries: []control.CommandCatalogEntry{{Form: form}},
		})
	}
	return control.FeishuDirectCommandCatalog{
		Title:          def.Title,
		Summary:        commandSummaryWithFeedback(baseSummary, def, view),
		Interactive:    true,
		DisplayStyle:   control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:    commandBreadcrumbs(def.GroupID, def.Title),
		Sections:       sections,
		RelatedButtons: commandBackButtons(def.GroupID),
	}
}

func commandFormWithViewDefault(commandID string, view control.FeishuCommandConfigView) *control.CommandCatalogForm {
	return control.FeishuCommandFormWithDefault(commandID, strings.TrimSpace(view.FormDefaultValue))
}

func commandSummaryWithFeedback(baseSummary string, def control.FeishuCommandDefinition, view control.FeishuCommandConfigView) string {
	lines := []string{}
	if feedback := commandFeedbackLine(view); feedback != "" {
		lines = append(lines, feedback)
	}
	if summary := strings.TrimSpace(baseSummary); summary != "" {
		lines = append(lines, summary)
	}
	if view.Sealed {
		if command := strings.TrimSpace(def.CanonicalSlash); command != "" {
			lines = append(lines, fmt.Sprintf("如需再次调整，请重新发送 `%s`。", command))
		}
	}
	return strings.Join(lines, "\n")
}

func commandFeedbackLine(view control.FeishuCommandConfigView) string {
	text := strings.TrimSpace(view.StatusText)
	if text == "" {
		return ""
	}
	switch strings.TrimSpace(view.StatusKind) {
	case "error":
		return "错误：" + text
	case "info":
		return "说明：" + text
	default:
		return "状态：" + text
	}
}

func sealedCommandCatalogForDefinition(def control.FeishuCommandDefinition, baseSummary string, view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	return control.FeishuDirectCommandCatalog{
		Title:        def.Title,
		Summary:      commandSummaryWithFeedback(baseSummary, def, view),
		Interactive:  false,
		DisplayStyle: control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:  commandBreadcrumbs(def.GroupID, def.Title),
	}
}

func fixedChoiceButtonsFromOptions(options []control.FeishuCommandOption, currentValue, primaryValue string) []control.CommandCatalogButton {
	buttons := make([]control.CommandCatalogButton, 0, len(options))
	currentValue = strings.TrimSpace(currentValue)
	for _, option := range options {
		value := strings.TrimSpace(option.Value)
		if value == "" {
			continue
		}
		style := ""
		if value == primaryValue {
			style = "primary"
		}
		buttons = append(buttons, control.CommandCatalogButton{
			Label:       strings.TrimSpace(option.Label),
			Kind:        control.CommandCatalogButtonRunCommand,
			CommandText: strings.TrimSpace(option.CommandText),
			Style:       style,
			Disabled:    currentValue != "" && currentValue == value,
		})
	}
	return buttons
}
