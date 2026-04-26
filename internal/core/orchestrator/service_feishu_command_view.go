package orchestrator

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/state"
)

func (s *Service) buildCommandMenuView(surface *state.SurfaceConsoleRecord, raw string) control.FeishuCatalogView {
	return control.FeishuCatalogView{
		Menu: &control.FeishuCatalogMenuView{
			Stage:   string(s.commandMenuStage(surface)),
			GroupID: parseCommandMenuView(raw),
		},
	}
}

func (s *Service) buildConfigCommandView(surface *state.SurfaceConsoleRecord, commandID string) control.FeishuCatalogView {
	return s.buildConfigCommandViewState(surface, commandID, control.FeishuCatalogConfigView{})
}

func (s *Service) buildConfigCommandViewState(
	surface *state.SurfaceConsoleRecord,
	commandID string,
	cardState control.FeishuCatalogConfigView,
) control.FeishuCatalogView {
	flow, ok := control.FeishuConfigFlowDefinitionByCommandID(commandID)
	if !ok {
		return control.FeishuCatalogView{}
	}

	view := control.FeishuCatalogView{
		Config: s.applyCommandConfigCardState(&control.FeishuCatalogConfigView{CommandID: flow.CommandID}, cardState),
	}

	attachedInstanceID := ""
	if surface != nil {
		attachedInstanceID = strings.TrimSpace(surface.AttachedInstanceID)
	}
	inst := s.root.Instances[attachedInstanceID]
	if flow.RequiresAttachment && inst == nil {
		view.Config.RequiresAttachment = true
		return view
	}

	var summary control.PromptRouteSummary
	if flow.UsesPromptSummary() && inst != nil {
		summary = s.resolveNextPromptSummary(inst, surface, "", "", state.ModelConfigRecord{})
	}

	view.Config.CurrentValue = s.resolveConfigFlowValue(surface, summary, flow.CurrentValueKey)
	view.Config.EffectiveValue = s.resolveConfigFlowValue(surface, summary, flow.EffectiveValueKey)
	view.Config.OverrideValue = s.resolveConfigFlowValue(surface, summary, flow.OverrideValueKey)
	view.Config.OverrideExtraValue = s.resolveConfigFlowValue(surface, summary, flow.OverrideExtraValueKey)
	return view
}

func (s *Service) resolveConfigFlowValue(
	surface *state.SurfaceConsoleRecord,
	summary control.PromptRouteSummary,
	key control.FeishuConfigFlowValueKey,
) string {
	switch key {
	case control.FeishuConfigFlowValueSurfaceProductMode:
		return string(s.normalizeSurfaceProductMode(surface))
	case control.FeishuConfigFlowValueSurfaceAutoWhip:
		if surface != nil && surface.AutoWhip.Enabled {
			return "on"
		}
		return "off"
	case control.FeishuConfigFlowValueSurfaceAutoContinue:
		if surface != nil && surface.AutoContinue.Enabled {
			return "on"
		}
		return "off"
	case control.FeishuConfigFlowValueSurfacePlanMode:
		current := state.PlanModeSettingOff
		if surface != nil {
			current = state.NormalizePlanModeSetting(surface.PlanMode)
		}
		return string(current)
	case control.FeishuConfigFlowValueSurfaceVerbosity:
		current := state.SurfaceVerbosityNormal
		if surface != nil {
			current = state.NormalizeSurfaceVerbosity(surface.Verbosity)
		}
		return string(current)
	case control.FeishuConfigFlowValuePromptEffectiveReasoning:
		return strings.TrimSpace(summary.EffectiveReasoningEffort)
	case control.FeishuConfigFlowValuePromptOverrideReasoning:
		return strings.TrimSpace(summary.OverrideReasoningEffort)
	case control.FeishuConfigFlowValuePromptEffectiveAccess:
		return strings.TrimSpace(summary.EffectiveAccessMode)
	case control.FeishuConfigFlowValuePromptOverrideAccess:
		return strings.TrimSpace(summary.OverrideAccessMode)
	case control.FeishuConfigFlowValuePromptObservedThreadPlan:
		return strings.TrimSpace(summary.ObservedThreadPlanMode)
	case control.FeishuConfigFlowValuePromptEffectiveModel:
		return strings.TrimSpace(summary.EffectiveModel)
	case control.FeishuConfigFlowValuePromptOverrideModel:
		return strings.TrimSpace(summary.OverrideModel)
	default:
		return ""
	}
}

func (s *Service) applyCommandConfigCardState(base *control.FeishuCatalogConfigView, cardState control.FeishuCatalogConfigView) *control.FeishuCatalogConfigView {
	if base == nil {
		base = &control.FeishuCatalogConfigView{}
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

func (s *Service) commandPageFromView(surface *state.SurfaceConsoleRecord, view control.FeishuCatalogView) control.FeishuPageView {
	productMode := ""
	stage := ""
	if surface != nil {
		productMode = string(s.normalizeSurfaceProductMode(surface))
		stage = string(s.commandMenuStage(surface))
	}
	page, ok := control.FeishuPageViewFromView(view, productMode, stage)
	if !ok {
		return control.FeishuPageView{}
	}
	return page
}
