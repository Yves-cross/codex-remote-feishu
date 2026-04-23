package daemon

import "testing"

func TestUpgradeOwnerButtonUsesCanonicalPayloadShape(t *testing.T) {
	button := upgradeOwnerButton("确认升级", "flow-1", upgradeOwnerActionConfirm, "primary", false)
	value := button.CallbackValue
	if value["kind"] != "upgrade_owner_flow" {
		t.Fatalf("unexpected callback kind: %#v", value)
	}
	if value["picker_id"] != "flow-1" {
		t.Fatalf("unexpected picker id: %#v", value)
	}
	if value["option_id"] != upgradeOwnerActionConfirm {
		t.Fatalf("unexpected option id: %#v", value)
	}
}
