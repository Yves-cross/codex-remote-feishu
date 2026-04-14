package control

import (
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/buildinfo"
)

func FeishuCommandGroups() []FeishuCommandGroup {
	groups := make([]FeishuCommandGroup, 0, len(feishuCommandGroups))
	for _, group := range feishuCommandGroups {
		groups = append(groups, group)
	}
	return groups
}

func FeishuCommandGroupByID(groupID string) (FeishuCommandGroup, bool) {
	for _, group := range feishuCommandGroups {
		if group.ID == groupID {
			return group, true
		}
	}
	return FeishuCommandGroup{}, false
}

func FeishuCommandDefinitions() []FeishuCommandDefinition {
	defs := make([]FeishuCommandDefinition, 0, len(feishuCommandSpecs))
	for _, spec := range feishuCommandSpecs {
		defs = append(defs, runtimeFeishuCommandDefinition(spec))
	}
	return defs
}

func FeishuCommandDefinitionByID(commandID string) (FeishuCommandDefinition, bool) {
	for _, spec := range feishuCommandSpecs {
		if spec.definition.ID == commandID {
			return runtimeFeishuCommandDefinition(spec), true
		}
	}
	return FeishuCommandDefinition{}, false
}

func FeishuCommandDefinitionsForGroup(groupID string) []FeishuCommandDefinition {
	defs := make([]FeishuCommandDefinition, 0, len(feishuCommandSpecs))
	for _, spec := range feishuCommandSpecs {
		if spec.definition.GroupID != groupID {
			continue
		}
		defs = append(defs, runtimeFeishuCommandDefinition(spec))
	}
	return defs
}

func FeishuCommandHelpCatalog() FeishuDirectCommandCatalog {
	return buildFeishuCommandCatalog(
		"Slash 命令帮助",
		"以下是当前主展示的 canonical slash command。历史 alias 仍可兼容，但不再作为新的主展示入口。",
		false,
	)
}

func FeishuCommandMenuCatalog() FeishuDirectCommandCatalog {
	return buildFeishuCommandCatalog(
		"命令目录",
		"这是同源的静态命令目录。真正的 `/menu` 首页会在 service 层按当前阶段动态重排。",
		true,
	)
}

func FeishuRecommendedMenus() []FeishuRecommendedMenu {
	order := []string{
		FeishuCommandMenu,
		FeishuCommandStop,
		FeishuCommandSteerAll,
		FeishuCommandNew,
		FeishuCommandReasoning,
		FeishuCommandModel,
		FeishuCommandAccess,
	}
	menus := make([]FeishuRecommendedMenu, 0, len(order))
	for _, commandID := range order {
		def, ok := FeishuCommandDefinitionByID(commandID)
		if !ok || def.RecommendedMenu == nil {
			continue
		}
		menu := *def.RecommendedMenu
		menus = append(menus, FeishuRecommendedMenu{
			Key:         strings.TrimSpace(menu.Key),
			Name:        strings.TrimSpace(menu.Name),
			Description: strings.TrimSpace(menu.Description),
		})
	}
	return menus
}

func buildFeishuCommandCatalog(title, summary string, interactive bool) FeishuDirectCommandCatalog {
	sections := make([]CommandCatalogSection, 0, len(feishuCommandGroups))
	for _, group := range feishuCommandGroups {
		entries := make([]CommandCatalogEntry, 0, len(feishuCommandSpecs))
		for _, spec := range feishuCommandSpecs {
			def := runtimeFeishuCommandDefinition(spec)
			if def.GroupID != group.ID {
				continue
			}
			if interactive && !def.ShowInMenu {
				continue
			}
			if !interactive && !def.ShowInHelp {
				continue
			}
			entry := CommandCatalogEntry{
				Title:       strings.TrimSpace(def.Title),
				Commands:    []string{def.CanonicalSlash},
				Description: def.Description,
				Examples:    append([]string(nil), def.Examples...),
			}
			if interactive {
				entry.Buttons = append(entry.Buttons, CommandCatalogButton{
					Label:       catalogButtonLabel(def),
					Kind:        CommandCatalogButtonRunCommand,
					CommandText: def.CanonicalSlash,
				})
			}
			entries = append(entries, entry)
		}
		if len(entries) == 0 {
			continue
		}
		sections = append(sections, CommandCatalogSection{
			Title:   group.Title,
			Entries: entries,
		})
	}
	return FeishuDirectCommandCatalog{
		Title:       title,
		Summary:     summary,
		Interactive: interactive,
		Sections:    sections,
	}
}

func catalogButtonLabel(def FeishuCommandDefinition) string {
	switch def.ArgumentKind {
	case FeishuCommandArgumentChoice, FeishuCommandArgumentText:
		return "打开"
	default:
		return strings.TrimSpace(def.Title)
	}
}

func cloneFeishuCommandDefinition(def FeishuCommandDefinition) FeishuCommandDefinition {
	cloned := def
	cloned.Examples = append([]string(nil), def.Examples...)
	if len(def.Options) > 0 {
		cloned.Options = append([]FeishuCommandOption(nil), def.Options...)
	}
	if def.RecommendedMenu != nil {
		menu := *def.RecommendedMenu
		cloned.RecommendedMenu = &menu
	}
	return cloned
}

func runtimeFeishuCommandDefinition(spec feishuCommandSpec) FeishuCommandDefinition {
	def := cloneFeishuCommandDefinition(spec.definition)
	switch def.ID {
	case FeishuCommandUpgrade:
		return runtimeUpgradeCommandDefinition(def)
	case FeishuCommandDebug:
		return runtimeDebugCommandDefinition(def)
	default:
		return def
	}
}

func runtimeUpgradeCommandDefinition(def FeishuCommandDefinition) FeishuCommandDefinition {
	policy := buildinfo.CurrentCapabilityPolicy()
	formHints := []string{"track", "latest"}
	examples := []string{"/upgrade latest"}
	options := []FeishuCommandOption{
		commandOption("/upgrade", "upgrade", "track", "track", "查看当前 track。"),
		commandOption("/upgrade", "upgrade", "latest", "latest", "检查或继续升级到当前 track 的最新 release。"),
	}
	if trackExample := preferredUpgradeTrackExample(policy.AllowedReleaseTracks); trackExample != "" {
		examples = append(examples, "/upgrade track "+trackExample)
	}
	for _, track := range policy.AllowedReleaseTracks {
		track = strings.TrimSpace(track)
		if track == "" {
			continue
		}
		options = append(options, commandOption("/upgrade track", "upgrade_track", track, "track "+track, "切换到 "+track+" track。"))
	}
	description := "查看升级状态、查看或切换当前 release track；`/upgrade latest` 检查或继续 release 升级。"
	if policy.AllowLocalUpgrade {
		formHints = append(formHints, "local")
		examples = append(examples, "/upgrade local")
		options = append(options, commandOption("/upgrade", "upgrade", "local", "local", "使用固定本地 artifact 发起升级。"))
		description += " `/upgrade local` 使用固定本地 artifact 发起升级。"
	}
	def.ArgumentFormHint = "track"
	def.ArgumentFormNote = "例如 " + strings.Join(formHints, "、") + "。"
	def.Description = description
	def.Examples = examples
	def.Options = options
	return def
}

func runtimeDebugCommandDefinition(def FeishuCommandDefinition) FeishuCommandDefinition {
	def.ArgumentFormNote = "例如 admin。"
	def.Description = "查看调试状态，或生成临时管理页外链。历史兼容的 `/debug track` 请改用 `/upgrade track`。"
	def.Examples = []string{"/debug", "/debug admin"}
	return def
}

func preferredUpgradeTrackExample(allowed []string) string {
	for _, candidate := range []string{"beta", "production", "alpha"} {
		for _, track := range allowed {
			if strings.EqualFold(strings.TrimSpace(track), candidate) {
				return candidate
			}
		}
	}
	return ""
}

func FeishuCommandForm(commandID string) (*CommandCatalogForm, bool) {
	def, ok := FeishuCommandDefinitionByID(commandID)
	if !ok {
		return nil, false
	}
	switch def.ArgumentKind {
	case FeishuCommandArgumentChoice, FeishuCommandArgumentText:
	default:
		return nil, false
	}
	submit := strings.TrimSpace(def.ArgumentSubmit)
	if submit == "" {
		submit = "执行"
	}
	label := strings.TrimSpace(def.ArgumentFormNote)
	if label == "" {
		label = "输入这条命令后面的参数。"
	}
	return &CommandCatalogForm{
		CommandID:   def.ID,
		CommandText: def.CanonicalSlash,
		SubmitLabel: submit,
		Field: CommandCatalogFormField{
			Name:        "command_args",
			Kind:        CommandCatalogFormFieldText,
			Label:       label,
			Placeholder: strings.TrimSpace(def.ArgumentFormHint),
		},
	}, true
}

func FeishuCommandFormWithDefault(commandID, defaultValue string) *CommandCatalogForm {
	form, ok := FeishuCommandForm(commandID)
	if !ok || form == nil {
		return nil
	}
	cloned := *form
	cloned.Field = form.Field
	cloned.Field.DefaultValue = strings.TrimSpace(defaultValue)
	return &cloned
}
