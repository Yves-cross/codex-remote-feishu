package control

import "testing"

func TestParseFeishuTextActionRecognizesDebugCommand(t *testing.T) {
	action, ok := ParseFeishuTextAction("/debug upgrade")
	if !ok {
		t.Fatal("expected /debug upgrade to be parsed")
	}
	if action.Kind != ActionDebugCommand {
		t.Fatalf("action kind = %q, want %q", action.Kind, ActionDebugCommand)
	}
	if action.Text != "/debug upgrade" {
		t.Fatalf("action text = %q, want %q", action.Text, "/debug upgrade")
	}
}

func TestFeishuCommandCatalogsHideKillInstanceFromVisibleEntries(t *testing.T) {
	cases := []struct {
		name    string
		catalog CommandCatalog
	}{
		{name: "help", catalog: FeishuCommandHelpCatalog()},
		{name: "menu", catalog: FeishuCommandMenuCatalog()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, section := range tc.catalog.Sections {
				for _, entry := range section.Entries {
					for _, command := range entry.Commands {
						if command == "/killinstance" {
							t.Fatalf("catalog still exposes /killinstance in commands: %#v", entry)
						}
					}
					for _, button := range entry.Buttons {
						if button.CommandText == "/killinstance" {
							t.Fatalf("catalog still exposes /killinstance in buttons: %#v", entry)
						}
					}
				}
			}
		})
	}
}

func TestParseFeishuLegacyKillInstanceCommandsAsRemoved(t *testing.T) {
	action, ok := ParseFeishuTextAction("/killinstance")
	if !ok {
		t.Fatal("expected /killinstance to be parsed")
	}
	if action.Kind != ActionRemovedCommand || action.Text != "/killinstance" {
		t.Fatalf("unexpected text action for /killinstance: %#v", action)
	}

	menu, ok := ParseFeishuMenuAction("kill_instance")
	if !ok {
		t.Fatal("expected kill_instance menu action to be parsed")
	}
	if menu.Kind != ActionRemovedCommand || menu.Text != "kill_instance" {
		t.Fatalf("unexpected menu action for kill_instance: %#v", menu)
	}
}
