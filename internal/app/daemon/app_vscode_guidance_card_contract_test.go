package daemon

import "testing"

func TestVSCodeMigrationOwnerButtonUsesCanonicalPayloadShape(t *testing.T) {
	button := vscodeMigrationOwnerButton("迁移并重新接入", "flow-1")
	value := button.CallbackValue
	if value["kind"] != "vscode_migrate_owner_flow" {
		t.Fatalf("unexpected callback kind: %#v", value)
	}
	if value["picker_id"] != "flow-1" {
		t.Fatalf("unexpected picker id: %#v", value)
	}
	if value["option_id"] != vscodeMigrationOwnerActionRun {
		t.Fatalf("unexpected option id: %#v", value)
	}
}
