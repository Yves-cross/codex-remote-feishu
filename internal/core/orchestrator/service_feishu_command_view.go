package orchestrator

import (
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

func (s *Service) buildPlanCommandView(surface *state.SurfaceConsoleRecord) control.FeishuCommandView {
	return s.buildPlanCommandViewState(surface, control.FeishuCommandConfigView{})
}

func (s *Service) buildPlanCommandViewState(surface *state.SurfaceConsoleRecord, cardState control.FeishuCommandConfigView) control.FeishuCommandView {
	current := state.PlanModeSettingOff
	if surface != nil {
		current = state.NormalizePlanModeSetting(surface.PlanMode)
	}
	view := control.FeishuCommandView{
		Config: s.applyCommandConfigCardState(&control.FeishuCommandConfigView{
			CommandID:    control.FeishuCommandPlan,
			CurrentValue: string(current),
		}, cardState),
	}
	attachedInstanceID := ""
	if surface != nil {
		attachedInstanceID = strings.TrimSpace(surface.AttachedInstanceID)
	}
	inst := s.root.Instances[attachedInstanceID]
	if inst == nil || surface == nil {
		return view
	}
	summary := s.resolveNextPromptSummary(inst, surface, "", "", state.ModelConfigRecord{})
	view.Config.EffectiveValue = strings.TrimSpace(summary.ObservedThreadPlanMode)
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

func (s *Service) commandPageFromView(surface *state.SurfaceConsoleRecord, view control.FeishuCommandView) control.FeishuCommandPageView {
	productMode := ""
	stage := ""
	if surface != nil {
		productMode = string(s.normalizeSurfaceProductMode(surface))
		stage = string(s.commandMenuStage(surface))
	}
	page, ok := control.FeishuCommandPageViewFromView(view, productMode, stage)
	if !ok {
		return control.FeishuCommandPageView{}
	}
	return page
}
