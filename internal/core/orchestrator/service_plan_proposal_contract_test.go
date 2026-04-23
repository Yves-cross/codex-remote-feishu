package orchestrator

import "testing"

func TestPlanProposalButtonUsesCanonicalPayloadShape(t *testing.T) {
	button := planProposalButton("直接执行", "proposal-1", "execute", "primary")
	value := button.CallbackValue
	if value["kind"] != "plan_proposal" {
		t.Fatalf("unexpected callback kind: %#v", value)
	}
	if value["picker_id"] != "proposal-1" {
		t.Fatalf("unexpected picker id: %#v", value)
	}
	if value["option_id"] != "execute" {
		t.Fatalf("unexpected option id: %#v", value)
	}
}
