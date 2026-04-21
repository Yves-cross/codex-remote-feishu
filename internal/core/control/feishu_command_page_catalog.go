package control

import "strings"

func NormalizeFeishuCommandPageView(view FeishuCommandPageView) FeishuCommandPageView {
	def, _ := FeishuCommandDefinitionByID(strings.TrimSpace(view.CommandID))
	title := strings.TrimSpace(view.Title)
	if title == "" {
		title = strings.TrimSpace(def.Title)
	}
	displayStyle := view.DisplayStyle
	if displayStyle == "" {
		displayStyle = CommandCatalogDisplayCompactButtons
	}
	breadcrumbs := cloneCommandBreadcrumbs(view.Breadcrumbs)
	if len(breadcrumbs) == 0 && strings.TrimSpace(def.GroupID) != "" {
		breadcrumbs = FeishuCommandBreadcrumbs(def.GroupID, title)
	}
	bodySections := BuildFeishuCommandPageBodySections(view)
	noticeSections := BuildFeishuCommandPageNoticeSections(view)
	sections := cloneCommandCatalogSections(view.Sections)
	relatedButtons := cloneCommandCatalogButtons(view.RelatedButtons)
	interactive := view.Interactive
	if view.Sealed {
		interactive = false
		relatedButtons = nil
	}
	if len(relatedButtons) == 0 && strings.TrimSpace(def.GroupID) != "" && !view.Sealed {
		relatedButtons = FeishuCommandBackButtons(def.GroupID)
	}
	return FeishuCommandPageView{
		CommandID:       strings.TrimSpace(view.CommandID),
		Title:           title,
		MessageID:       strings.TrimSpace(view.MessageID),
		TrackingKey:     strings.TrimSpace(view.TrackingKey),
		ThemeKey:        strings.TrimSpace(view.ThemeKey),
		Patchable:       view.Patchable,
		Breadcrumbs:     breadcrumbs,
		SummarySections: cloneNormalizedFeishuCardSections(bodySections),
		BodySections:    bodySections,
		NoticeSections:  noticeSections,
		Interactive:     interactive,
		Sealed:          view.Sealed,
		DisplayStyle:    displayStyle,
		Sections:        sections,
		RelatedButtons:  relatedButtons,
	}
}

func FeishuCommandPageViewFromView(view FeishuCommandView, productMode, menuStage string) (FeishuCommandPageView, bool) {
	switch {
	case view.Menu != nil:
		return BuildFeishuCommandMenuPageView(*view.Menu, productMode, menuStage), true
	case view.Config != nil:
		return BuildFeishuCommandConfigPageView(*view.Config), true
	case view.Page != nil:
		return NormalizeFeishuCommandPageView(*view.Page), true
	default:
		return FeishuCommandPageView{}, false
	}
}

func BuildFeishuCommandPageSummarySections(view FeishuCommandPageView) []FeishuCardTextSection {
	return BuildFeishuCommandPageBodySections(view)
}

func BuildFeishuCommandPageBodySections(view FeishuCommandPageView) []FeishuCardTextSection {
	return cloneNormalizedFeishuCardSections(firstNonEmptyFeishuCardSections(view.BodySections, view.SummarySections))
}

func BuildFeishuCommandPageNoticeSections(view FeishuCommandPageView) []FeishuCardTextSection {
	sections := make([]FeishuCardTextSection, 0, len(view.NoticeSections)+1)
	if feedback, ok := commandPageFeedbackSection(view); ok {
		sections = append(sections, feedback)
	}
	sections = append(sections, cloneNormalizedFeishuCardSections(view.NoticeSections)...)
	if len(sections) == 0 {
		return nil
	}
	return sections
}

func commandPageFeedbackSection(view FeishuCommandPageView) (FeishuCardTextSection, bool) {
	text := normalizeCommandFeedbackText(view.StatusText)
	if text == "" {
		return FeishuCardTextSection{}, false
	}
	label := "状态"
	switch strings.TrimSpace(view.StatusKind) {
	case "error":
		label = "错误"
	case "info":
		label = "说明"
	}
	return FeishuCardTextSection{
		Label: label,
		Lines: []string{text},
	}, true
}

func FeishuCommandBreadcrumbsForCommand(commandID string, extraLabels ...string) []CommandCatalogBreadcrumb {
	def, ok := FeishuCommandDefinitionByID(strings.TrimSpace(commandID))
	if !ok {
		return nil
	}
	breadcrumbs := FeishuCommandBreadcrumbs(def.GroupID, def.Title)
	for _, label := range extraLabels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		breadcrumbs = append(breadcrumbs, CommandCatalogBreadcrumb{Label: label})
	}
	return breadcrumbs
}

func FeishuCommandBackToRootButtons(commandID string) []CommandCatalogButton {
	def, ok := FeishuCommandDefinitionByID(strings.TrimSpace(commandID))
	if !ok {
		return nil
	}
	command := strings.TrimSpace(def.CanonicalSlash)
	if command == "" {
		return nil
	}
	return []CommandCatalogButton{{
		Label:       "返回" + strings.TrimSpace(def.Title),
		Kind:        CommandCatalogButtonRunCommand,
		CommandText: command,
	}}
}

func splitFeishuCommandPageSummaryLines(text string) []string {
	lines := make([]string, 0, 4)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func cloneNormalizedFeishuCardSections(source []FeishuCardTextSection) []FeishuCardTextSection {
	if len(source) == 0 {
		return nil
	}
	out := make([]FeishuCardTextSection, 0, len(source))
	for _, section := range source {
		normalized := section.Normalized()
		if normalized.Label == "" && len(normalized.Lines) == 0 {
			continue
		}
		clonedLines := append([]string(nil), normalized.Lines...)
		out = append(out, FeishuCardTextSection{
			Label: normalized.Label,
			Lines: clonedLines,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmptyFeishuCardSections(values ...[]FeishuCardTextSection) []FeishuCardTextSection {
	for _, value := range values {
		if len(value) != 0 {
			return value
		}
	}
	return nil
}

func cloneCommandBreadcrumbs(source []CommandCatalogBreadcrumb) []CommandCatalogBreadcrumb {
	if len(source) == 0 {
		return nil
	}
	return append([]CommandCatalogBreadcrumb(nil), source...)
}

func cloneCommandCatalogButtons(source []CommandCatalogButton) []CommandCatalogButton {
	if len(source) == 0 {
		return nil
	}
	return append([]CommandCatalogButton(nil), source...)
}

func cloneCommandCatalogSections(source []CommandCatalogSection) []CommandCatalogSection {
	if len(source) == 0 {
		return nil
	}
	out := make([]CommandCatalogSection, 0, len(source))
	for _, section := range source {
		cloned := CommandCatalogSection{
			Title:   strings.TrimSpace(section.Title),
			Entries: make([]CommandCatalogEntry, 0, len(section.Entries)),
		}
		for _, entry := range section.Entries {
			cloned.Entries = append(cloned.Entries, CommandCatalogEntry{
				Title:       strings.TrimSpace(entry.Title),
				Commands:    append([]string(nil), entry.Commands...),
				Description: strings.TrimSpace(entry.Description),
				Examples:    append([]string(nil), entry.Examples...),
				Buttons:     cloneCommandCatalogButtons(entry.Buttons),
				Form:        cloneCommandCatalogForm(entry.Form),
			})
		}
		out = append(out, cloned)
	}
	return out
}

func cloneCommandCatalogForm(form *CommandCatalogForm) *CommandCatalogForm {
	if form == nil {
		return nil
	}
	cloned := *form
	cloned.Field = CommandCatalogFormField{
		Name:         strings.TrimSpace(form.Field.Name),
		Kind:         form.Field.Kind,
		Label:        strings.TrimSpace(form.Field.Label),
		Placeholder:  strings.TrimSpace(form.Field.Placeholder),
		DefaultValue: strings.TrimSpace(form.Field.DefaultValue),
		Options:      append([]CommandCatalogFormFieldOption(nil), form.Field.Options...),
	}
	return &cloned
}
