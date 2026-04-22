package feishu

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func pageBody(view control.FeishuPageView) string {
	return ""
}

func pageElements(view control.FeishuPageView, daemonLifecycleID string) []map[string]any {
	view = control.NormalizeFeishuPageView(view)
	elements := make([]map[string]any, 0, len(view.Sections)*3+len(view.SummarySections)*2+len(view.NoticeSections)*2+3)
	if breadcrumb := commandCatalogBreadcrumbMarkdown(view.Breadcrumbs); breadcrumb != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": breadcrumb,
		})
	}
	bodySections := control.BuildFeishuPageBodySections(view)
	if len(bodySections) != 0 {
		elements = appendCardTextSections(elements, bodySections)
	} else {
		elements = appendCardTextSections(elements, cloneNormalizedCardSections(view.SummarySections))
	}
	hasBusinessContent := len(bodySections) != 0
	for _, section := range view.Sections {
		title := strings.TrimSpace(section.Title)
		if title != "" {
			elements = append(elements, map[string]any{
				"tag":     "markdown",
				"content": "**" + title + "**",
			})
		}
		for _, entry := range section.Entries {
			renderedCompactButtons := false
			if view.DisplayStyle == control.CommandCatalogDisplayCompactButtons && view.Interactive && len(entry.Buttons) > 0 {
				elements = append(elements, pageCompactButtonElements(entry.Buttons, daemonLifecycleID)...)
				renderedCompactButtons = true
				if entry.Form == nil {
					continue
				}
			}
			elements = appendCardTextSections(elements, commandCatalogEntryFallbackSections(entry))
			if view.Interactive && len(entry.Buttons) > 0 && !renderedCompactButtons {
				if group := cardButtonGroupElement(pageButtons(entry.Buttons, daemonLifecycleID)); len(group) != 0 {
					elements = append(elements, group)
				}
			}
			if view.Interactive && entry.Form != nil {
				if formElement, ok := pageFormElement(*entry.Form, daemonLifecycleID); ok {
					elements = append(elements, formElement)
				}
			}
		}
		hasBusinessContent = true
	}
	if noticeSections := control.BuildFeishuPageNoticeSections(view); len(noticeSections) != 0 {
		if hasBusinessContent {
			elements = append(elements, cardDividerElement())
		}
		elements = appendCardTextSections(elements, noticeSections)
	}
	if len(view.RelatedButtons) > 0 {
		elements = appendCardFooterButtonGroup(elements, pageButtons(view.RelatedButtons, daemonLifecycleID))
	}
	return elements
}

func pageFormElement(form control.CommandCatalogForm, daemonLifecycleID string) (map[string]any, bool) {
	actionKind, ok := control.ActionKindForFeishuCommandID(strings.TrimSpace(form.CommandID))
	if !ok {
		return nil, false
	}
	field := form.Field
	formName := commandCatalogFormName(form)
	submitValue := stampActionValue(
		actionPayloadPageSubmit(
			string(actionKind),
			control.FeishuActionArgumentText(form.CommandText),
			strings.TrimSpace(field.Name),
		),
		daemonLifecycleID,
	)
	submitButton := cardFormSubmitButtonElement(firstNonEmpty(strings.TrimSpace(form.SubmitLabel), "执行"), submitValue)
	if len(submitButton) != 0 {
		submitButton["name"] = commandCatalogSubmitButtonName(formName)
	}
	return map[string]any{
		"tag":  "form",
		"name": formName,
		"elements": []map[string]any{
			commandCatalogFormFieldElement(field),
			submitButton,
		},
	}, true
}

func pageButtons(buttons []control.CommandCatalogButton, daemonLifecycleID string) []map[string]any {
	actions := make([]map[string]any, 0, len(buttons))
	defaultType := "default"
	if len(buttons) == 1 {
		defaultType = "primary"
	}
	for _, button := range buttons {
		label := strings.TrimSpace(button.Label)
		payload := map[string]any{}
		switch button.Kind {
		case "", control.CommandCatalogButtonRunCommand:
			commandText := strings.TrimSpace(button.CommandText)
			if commandText == "" {
				continue
			}
			action, ok := control.ParseFeishuTextAction(commandText)
			if !ok {
				continue
			}
			if label == "" {
				label = commandText
			}
			payload = actionPayloadPageAction(string(action.Kind), control.FeishuActionArgumentText(action.Text))
		case control.CommandCatalogButtonOpenURL:
			openURL := strings.TrimSpace(button.OpenURL)
			if openURL == "" {
				continue
			}
			if label == "" {
				label = openURL
			}
			buttonType := defaultType
			if style := strings.TrimSpace(button.Style); style != "" {
				buttonType = style
			}
			actions = append(actions, cardOpenURLButtonElement(label, buttonType, button.OpenURL, button.Disabled, ""))
			continue
		case control.CommandCatalogButtonCallbackAction:
			if len(button.CallbackValue) == 0 {
				continue
			}
			payload = cloneActionPayload(button.CallbackValue)
		default:
			continue
		}
		if label == "" {
			continue
		}
		buttonType := defaultType
		if style := strings.TrimSpace(button.Style); style != "" {
			buttonType = style
		}
		actions = append(actions, cardCallbackButtonElement(label, buttonType, stampActionValue(payload, daemonLifecycleID), button.Disabled, ""))
	}
	return actions
}

func pageCompactButtonElements(buttons []control.CommandCatalogButton, daemonLifecycleID string) []map[string]any {
	elements := make([]map[string]any, 0, len(buttons))
	for _, button := range buttons {
		actions := pageButtons([]control.CommandCatalogButton{button}, daemonLifecycleID)
		if len(actions) == 0 {
			continue
		}
		if group := cardButtonGroupElement(actions); len(group) != 0 {
			elements = append(elements, group)
		}
	}
	return elements
}
