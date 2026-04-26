package control

import "strings"

func BuildFeishuCommandMenuHomePageView() FeishuPageView {
	return BuildFeishuCommandMenuHomePageViewForProductMode("")
}

func BuildFeishuCommandMenuHomePageViewForProductMode(productMode string) FeishuPageView {
	return BuildFeishuCommandMenuHomePageViewForContext(CatalogContext{ProductMode: productMode})
}

func BuildFeishuCommandMenuHomePageViewForContext(ctx CatalogContext) FeishuPageView {
	ctx = NormalizeCatalogContext(ctx)
	return FeishuPageView{
		CommandID:    FeishuCommandMenu,
		Title:        "命令菜单",
		Interactive:  true,
		DisplayStyle: CommandCatalogDisplayCompactButtons,
		Breadcrumbs:  FeishuCommandBreadcrumbs("", ""),
		Sections: []CommandCatalogSection{{
			Title:   "",
			Entries: buildFeishuCommandMenuGroupEntries(ctx.ProductMode),
		}},
	}
}

func BuildFeishuCommandMenuPageView(view FeishuCatalogMenuView, productMode, menuStage string) FeishuPageView {
	return BuildFeishuCommandMenuPageViewForContext(view, CatalogContext{
		ProductMode: productMode,
		MenuStage:   menuStage,
	})
}

func BuildFeishuCommandMenuPageViewForContext(view FeishuCatalogMenuView, ctx CatalogContext) FeishuPageView {
	ctx = NormalizeCatalogContext(ctx)
	groupID := strings.TrimSpace(view.GroupID)
	if groupID == "" {
		return BuildFeishuCommandMenuHomePageViewForContext(ctx)
	}
	stage := strings.TrimSpace(view.Stage)
	if stage == "" {
		stage = strings.TrimSpace(ctx.MenuStage)
	}
	if ctx.ProductMode == "normal" && groupID == FeishuCommandGroupSwitchTarget {
		return BuildFeishuWorkspaceRootPageView(true)
	}
	ctx.MenuStage = stage
	return BuildFeishuCommandMenuGroupPageViewForContext(groupID, ctx)
}

func BuildFeishuCommandMenuGroupPageView(groupID, productMode, menuStage string) FeishuPageView {
	return BuildFeishuCommandMenuGroupPageViewForContext(groupID, CatalogContext{
		ProductMode: productMode,
		MenuStage:   menuStage,
	})
}

func BuildFeishuCommandMenuGroupPageViewForContext(groupID string, ctx CatalogContext) FeishuPageView {
	ctx = NormalizeCatalogContext(ctx)
	if _, ok := FeishuCommandGroupByID(groupID); !ok {
		return BuildFeishuCommandMenuHomePageViewForContext(ctx)
	}
	entries := make([]CommandCatalogEntry, 0, 6)
	for _, current := range ResolveFeishuCommandDisplayGroup(groupID, true, ctx) {
		entries = append(entries, buildFeishuCommandMenuEntry(current.Definition))
	}
	return FeishuPageView{
		CommandID:    FeishuCommandMenu,
		Title:        "命令菜单",
		Interactive:  true,
		DisplayStyle: CommandCatalogDisplayCompactButtons,
		Breadcrumbs:  FeishuCommandBreadcrumbs(groupID, ""),
		Sections: []CommandCatalogSection{{
			Title:   "",
			Entries: entries,
		}},
		RelatedButtons: []CommandCatalogButton{{
			Label:       "返回上一层",
			Kind:        CommandCatalogButtonAction,
			CommandText: FeishuCommandMenuCommandText(""),
		}},
	}
}

func BuildFeishuAttachmentRequiredPageView(def FeishuCommandDefinition, view FeishuCatalogConfigView) FeishuPageView {
	bodySections := BuildFeishuCommandConfigBodySections(def, view)
	noticeSections := BuildFeishuCommandConfigNoticeSections(def, view)
	return NormalizeFeishuPageView(FeishuPageView{
		CommandID:       strings.TrimSpace(def.ID),
		Title:           strings.TrimSpace(def.Title),
		SummarySections: append([]FeishuCardTextSection(nil), bodySections...),
		BodySections:    append([]FeishuCardTextSection(nil), bodySections...),
		NoticeSections:  append([]FeishuCardTextSection(nil), noticeSections...),
		Interactive:     true,
		DisplayStyle:    CommandCatalogDisplayCompactButtons,
		Breadcrumbs:     FeishuCommandBreadcrumbs(def.GroupID, def.Title),
		Sections: []CommandCatalogSection{{
			Title:   "开始 / 继续工作",
			Entries: buildFeishuRecoveryEntries(),
		}},
		RelatedButtons: FeishuCommandBackButtons(def.GroupID),
	})
}

func FeishuCommandBreadcrumbs(groupID, title string) []CommandCatalogBreadcrumb {
	breadcrumbs := []CommandCatalogBreadcrumb{{Label: "菜单首页"}}
	if group, ok := FeishuCommandGroupByID(groupID); ok {
		breadcrumbs = append(breadcrumbs, CommandCatalogBreadcrumb{Label: group.Title})
	}
	if title = strings.TrimSpace(title); title != "" {
		breadcrumbs = append(breadcrumbs, CommandCatalogBreadcrumb{Label: title})
	}
	return breadcrumbs
}

func FeishuCommandBackButtons(groupID string) []CommandCatalogButton {
	if _, ok := FeishuCommandGroupByID(groupID); ok {
		return []CommandCatalogButton{{
			Label:       "返回上一层",
			Kind:        CommandCatalogButtonAction,
			CommandText: FeishuCommandMenuCommandText(groupID),
		}}
	}
	return nil
}

func FeishuCommandMenuCommandText(view string) string {
	if strings.TrimSpace(view) == "" {
		return "/menu"
	}
	return "/menu " + strings.TrimSpace(view)
}

func buildFeishuCommandMenuGroupEntries(productMode string) []CommandCatalogEntry {
	entries := make([]CommandCatalogEntry, 0, len(FeishuCommandGroups()))
	for _, group := range FeishuCommandGroups() {
		commandText := FeishuCommandMenuCommandText(group.ID)
		if normalizeFeishuCommandProductMode(productMode) == "normal" && group.ID == FeishuCommandGroupSwitchTarget {
			commandText = "/workspace"
		}
		entries = append(entries, CommandCatalogEntry{
			Title:       strings.TrimSpace(group.Title),
			Description: "",
			Buttons: []CommandCatalogButton{{
				Label:       feishuSubmenuButtonLabel(group.Title),
				Kind:        CommandCatalogButtonAction,
				CommandText: commandText,
			}},
		})
	}
	return entries
}

func buildFeishuRecoveryEntries() []CommandCatalogEntry {
	return []CommandCatalogEntry{
		buildFeishuRecoveryEntry(FeishuCommandList),
		buildFeishuRecoveryEntry(FeishuCommandUse),
		buildFeishuRecoveryEntry(FeishuCommandStatus),
	}
}

func buildFeishuRecoveryEntry(commandID string) CommandCatalogEntry {
	def, ok := FeishuCommandDefinitionByID(commandID)
	if !ok {
		return CommandCatalogEntry{}
	}
	return buildFeishuCommandMenuEntry(def)
}

func buildFeishuCommandMenuEntry(def FeishuCommandDefinition) CommandCatalogEntry {
	return buildFeishuCommandCatalogEntry(def, feishuCommandMenuButtonLabel(def))
}

func buildFeishuCommandCatalogEntry(def FeishuCommandDefinition, buttonLabel string) CommandCatalogEntry {
	command := strings.TrimSpace(def.CanonicalSlash)
	entry := CommandCatalogEntry{
		Title:       strings.TrimSpace(def.Title),
		Description: strings.TrimSpace(def.Description),
		Examples:    append([]string(nil), def.Examples...),
	}
	if command != "" {
		entry.Commands = []string{command}
	}
	if buttonLabel = strings.TrimSpace(buttonLabel); buttonLabel != "" && command != "" {
		entry.Buttons = append(entry.Buttons, CommandCatalogButton{
			Label:       buttonLabel,
			Kind:        CommandCatalogButtonAction,
			CommandText: command,
		})
	}
	return entry
}

func feishuCommandMenuButtonLabel(def FeishuCommandDefinition) string {
	title := strings.TrimSpace(def.Title)
	command := strings.TrimSpace(def.CanonicalSlash)
	switch {
	case title == "":
		return command
	case command == "":
		return title
	default:
		return title + " " + command
	}
}

func feishuSubmenuButtonLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "进入"
	}
	return label
}
