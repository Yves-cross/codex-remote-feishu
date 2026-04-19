package control

import "testing"

func TestStaticCommandCatalogsUseNonLegacyContracts(t *testing.T) {
	cases := []struct {
		name    string
		catalog FeishuDirectCommandCatalog
	}{
		{name: "help", catalog: FeishuCommandHelpCatalog()},
		{name: "menu", catalog: FeishuCommandMenuCatalog()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCommandCatalogUsesNonLegacyContracts(t, tc.catalog)
		})
	}
}

func TestDisplayCatalogBuilderUsesNonLegacyContracts(t *testing.T) {
	catalog := BuildFeishuCommandCatalogForDisplay(
		"Slash 命令帮助",
		"当前展示 canonical 命令。",
		false,
		"normal",
		"",
	)
	assertCommandCatalogUsesNonLegacyContracts(t, catalog)
}

func TestCommandViewCatalogBuildersUseNonLegacyContracts(t *testing.T) {
	t.Run("menu_home", func(t *testing.T) {
		assertCommandCatalogUsesNonLegacyContracts(t, BuildFeishuCommandMenuHomeCatalog())
	})

	t.Run("menu_group", func(t *testing.T) {
		assertCommandCatalogUsesNonLegacyContracts(t, BuildFeishuCommandMenuGroupCatalog("current_work", "normal", "normal_working"))
	})

	t.Run("attachment_required", func(t *testing.T) {
		def, ok := FeishuCommandDefinitionByID(FeishuCommandReasoning)
		if !ok {
			t.Fatalf("expected builtin command definition")
		}
		catalog := BuildFeishuAttachmentRequiredCatalog(def, FeishuCommandConfigView{
			CommandID:          def.ID,
			RequiresAttachment: true,
		})
		if len(catalog.SummarySections) == 0 {
			t.Fatalf("expected attachment-required catalog to expose summary sections: %#v", catalog)
		}
		assertCommandCatalogUsesNonLegacyContracts(t, catalog)
	})
}

func assertCommandCatalogUsesNonLegacyContracts(t *testing.T, catalog FeishuDirectCommandCatalog) {
	t.Helper()
	if catalog.LegacySummaryMarkdown {
		t.Fatalf("expected non-legacy summary contract: %#v", catalog)
	}
	for _, section := range catalog.Sections {
		for _, entry := range section.Entries {
			if entry.LegacyMarkdown {
				t.Fatalf("expected non-legacy entry contract: %#v", entry)
			}
		}
	}
}
