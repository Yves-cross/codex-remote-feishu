package control

import "testing"

func TestStaticCommandCatalogsExplicitlyMarkLegacyMarkdownContracts(t *testing.T) {
	cases := []struct {
		name    string
		catalog FeishuDirectCommandCatalog
	}{
		{name: "help", catalog: FeishuCommandHelpCatalog()},
		{name: "menu", catalog: FeishuCommandMenuCatalog()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.catalog.LegacySummaryMarkdown {
				t.Fatalf("expected static catalog summary contract to be explicit: %#v", tc.catalog)
			}
			for _, section := range tc.catalog.Sections {
				for _, entry := range section.Entries {
					if !entry.LegacyMarkdown {
						t.Fatalf("expected static catalog entry contract to be explicit: %#v", entry)
					}
				}
			}
		})
	}
}

func TestDisplayCatalogBuilderExplicitlyMarksLegacyMarkdownContracts(t *testing.T) {
	catalog := BuildFeishuCommandCatalogForDisplay(
		"Slash 命令帮助",
		"当前展示 canonical 命令。",
		false,
		"normal",
		"",
	)
	if !catalog.LegacySummaryMarkdown {
		t.Fatalf("expected display catalog summary contract to be explicit: %#v", catalog)
	}
	for _, section := range catalog.Sections {
		for _, entry := range section.Entries {
			if !entry.LegacyMarkdown {
				t.Fatalf("expected display catalog entry contract to be explicit: %#v", entry)
			}
		}
	}
}
