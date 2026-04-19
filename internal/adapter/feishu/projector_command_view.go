package feishu

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

// FeishuDirectCommandCatalogFromView projects the UI-owned command view into the transition
// command catalog shape currently consumed by the Feishu renderer.
func FeishuDirectCommandCatalogFromView(view control.FeishuCommandView, ctx *control.FeishuUICommandContext) (control.FeishuDirectCommandCatalog, bool) {
	switch {
	case view.Menu != nil:
		return commandMenuCatalogFromView(*view.Menu, ctx), true
	case view.Config != nil:
		return commandConfigCatalogFromView(*view.Config), true
	default:
		return control.FeishuDirectCommandCatalog{}, false
	}
}

func commandMenuCatalogFromView(view control.FeishuCommandMenuView, ctx *control.FeishuUICommandContext) control.FeishuDirectCommandCatalog {
	stage := strings.TrimSpace(view.Stage)
	if stage == "" && ctx != nil {
		stage = strings.TrimSpace(ctx.MenuStage)
	}
	productMode := ""
	if ctx != nil {
		productMode = strings.TrimSpace(ctx.Surface.ProductMode)
	}
	groupID := strings.TrimSpace(view.GroupID)
	if groupID == "" {
		return control.BuildFeishuCommandMenuHomeCatalog()
	}
	return control.BuildFeishuCommandMenuGroupCatalog(groupID, productMode, stage)
}

func commandConfigCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
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
	summarySections := control.BuildFeishuCommandConfigSummarySections(def, view)
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, summarySections)
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
		Title:           def.Title,
		SummarySections: summarySections,
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:     control.FeishuCommandBreadcrumbs(def.GroupID, def.Title),
		Sections:        sections,
		RelatedButtons:  control.FeishuCommandBackButtons(def.GroupID),
	}
}

func autoContinueCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandAutoContinue)
	summarySections := control.BuildFeishuCommandConfigSummarySections(def, view)
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, summarySections)
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
		Title:           def.Title,
		SummarySections: summarySections,
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:     control.FeishuCommandBreadcrumbs(def.GroupID, def.Title),
		Sections:        sections,
		RelatedButtons:  control.FeishuCommandBackButtons(def.GroupID),
	}
}

func reasoningCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandReasoning)
	summarySections := control.BuildFeishuCommandConfigSummarySections(def, view)
	if view.RequiresAttachment {
		return control.BuildFeishuAttachmentRequiredCatalog(def, view)
	}
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, summarySections)
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
		Title:           def.Title,
		SummarySections: summarySections,
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:     control.FeishuCommandBreadcrumbs(def.GroupID, def.Title),
		Sections:        sections,
		RelatedButtons:  control.FeishuCommandBackButtons(def.GroupID),
	}
}

func accessCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandAccess)
	summarySections := control.BuildFeishuCommandConfigSummarySections(def, view)
	if view.RequiresAttachment {
		return control.BuildFeishuAttachmentRequiredCatalog(def, view)
	}
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, summarySections)
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
		Title:           def.Title,
		SummarySections: summarySections,
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:     control.FeishuCommandBreadcrumbs(def.GroupID, def.Title),
		Sections:        sections,
		RelatedButtons:  control.FeishuCommandBackButtons(def.GroupID),
	}
}

func modelCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandModel)
	summarySections := control.BuildFeishuCommandConfigSummarySections(def, view)
	if view.RequiresAttachment {
		return control.BuildFeishuAttachmentRequiredCatalog(def, view)
	}
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, summarySections)
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
		Breadcrumbs:  control.FeishuCommandBreadcrumbs(def.GroupID, def.Title),
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
		RelatedButtons: control.FeishuCommandBackButtons(def.GroupID),
	}
	catalog.SummarySections = summarySections
	return catalog
}

func verboseCatalogFromView(view control.FeishuCommandConfigView) control.FeishuDirectCommandCatalog {
	def, _ := control.FeishuCommandDefinitionByID(control.FeishuCommandVerbose)
	current := strings.TrimSpace(view.CurrentValue)
	summarySections := control.BuildFeishuCommandConfigSummarySections(def, view)
	if view.Sealed {
		return sealedCommandCatalogForDefinition(def, summarySections)
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
		Title:           def.Title,
		SummarySections: summarySections,
		Interactive:     true,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:     control.FeishuCommandBreadcrumbs(def.GroupID, def.Title),
		Sections:        sections,
		RelatedButtons:  control.FeishuCommandBackButtons(def.GroupID),
	}
}

func commandFormWithViewDefault(commandID string, view control.FeishuCommandConfigView) *control.CommandCatalogForm {
	return control.FeishuCommandFormWithDefault(commandID, strings.TrimSpace(view.FormDefaultValue))
}

func sealedCommandCatalogForDefinition(def control.FeishuCommandDefinition, summarySections []control.FeishuCardTextSection) control.FeishuDirectCommandCatalog {
	return control.FeishuDirectCommandCatalog{
		Title:           def.Title,
		SummarySections: append([]control.FeishuCardTextSection(nil), summarySections...),
		Interactive:     false,
		DisplayStyle:    control.CommandCatalogDisplayCompactButtons,
		Breadcrumbs:     control.FeishuCommandBreadcrumbs(def.GroupID, def.Title),
	}
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
			CommandText: strings.TrimSpace(option.CommandText),
			Style:       style,
			Disabled:    disabled,
		})
	}
	return buttons
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
