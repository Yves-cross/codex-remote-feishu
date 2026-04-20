package control

import "strings"

func BuildFeishuCommandPageCatalog(view FeishuCommandPageView) FeishuDirectCommandCatalog {
	def, _ := FeishuCommandDefinitionByID(strings.TrimSpace(view.CommandID))
	title := strings.TrimSpace(view.Title)
	if title == "" {
		title = strings.TrimSpace(def.Title)
	}
	displayStyle := view.DisplayStyle
	if displayStyle == "" {
		displayStyle = CommandCatalogDisplayCompactButtons
	}
	breadcrumbs := append([]CommandCatalogBreadcrumb(nil), view.Breadcrumbs...)
	if len(breadcrumbs) == 0 && strings.TrimSpace(def.GroupID) != "" {
		breadcrumbs = FeishuCommandBreadcrumbs(def.GroupID, title)
	}
	sections := append([]CommandCatalogSection(nil), view.Sections...)
	relatedButtons := append([]CommandCatalogButton(nil), view.RelatedButtons...)
	if len(relatedButtons) == 0 && strings.TrimSpace(def.GroupID) != "" {
		relatedButtons = FeishuCommandBackButtons(def.GroupID)
	}
	return FeishuDirectCommandCatalog{
		Title:           title,
		MessageID:       strings.TrimSpace(view.MessageID),
		TrackingKey:     strings.TrimSpace(view.TrackingKey),
		ThemeKey:        strings.TrimSpace(view.ThemeKey),
		Patchable:       view.Patchable,
		SummarySections: BuildFeishuCommandPageSummarySections(view),
		Interactive:     view.Interactive,
		DisplayStyle:    displayStyle,
		Breadcrumbs:     breadcrumbs,
		Sections:        sections,
		RelatedButtons:  relatedButtons,
	}
}

func FeishuCommandPageViewFromCatalog(commandID string, catalog FeishuDirectCommandCatalog, breadcrumbs []CommandCatalogBreadcrumb, relatedButtons []CommandCatalogButton) FeishuCommandPageView {
	view := FeishuCommandPageView{
		CommandID:       strings.TrimSpace(commandID),
		Title:           strings.TrimSpace(catalog.Title),
		MessageID:       strings.TrimSpace(catalog.MessageID),
		TrackingKey:     strings.TrimSpace(catalog.TrackingKey),
		ThemeKey:        strings.TrimSpace(catalog.ThemeKey),
		Patchable:       catalog.Patchable,
		Breadcrumbs:     append([]CommandCatalogBreadcrumb(nil), breadcrumbs...),
		SummarySections: append([]FeishuCardTextSection(nil), catalog.SummarySections...),
		Interactive:     catalog.Interactive,
		DisplayStyle:    catalog.DisplayStyle,
		Sections:        append([]CommandCatalogSection(nil), catalog.Sections...),
		RelatedButtons:  append([]CommandCatalogButton(nil), relatedButtons...),
	}
	if view.DisplayStyle == "" {
		view.DisplayStyle = CommandCatalogDisplayDefault
	}
	if len(view.SummarySections) == 0 {
		lines := splitFeishuCommandPageSummaryLines(catalog.Summary)
		if len(lines) != 0 {
			view.SummarySections = []FeishuCardTextSection{{Lines: lines}}
		}
	}
	if view.CommandID == "" {
		view.CommandID = strings.TrimSpace(commandID)
	}
	return view
}

func BuildFeishuCommandPageSummarySections(view FeishuCommandPageView) []FeishuCardTextSection {
	sections := make([]FeishuCardTextSection, 0, len(view.SummarySections)+1)
	if feedback, ok := commandPageFeedbackSection(view); ok {
		sections = append(sections, feedback)
	}
	for _, section := range view.SummarySections {
		normalized := section.Normalized()
		if normalized.Label == "" && len(normalized.Lines) == 0 {
			continue
		}
		sections = append(sections, normalized)
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
