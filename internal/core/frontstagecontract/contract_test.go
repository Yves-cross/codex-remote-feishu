package frontstagecontract

import "testing"

func TestNormalizeFrameDerivesDefaultActionPolicyAndSealedState(t *testing.T) {
	tests := []struct {
		name       string
		phase      Phase
		policy     ActionPolicy
		wantPolicy ActionPolicy
		wantSealed bool
	}{
		{name: "editing stays interactive", phase: PhaseEditing, wantPolicy: ActionPolicyInteractive, wantSealed: false},
		{name: "waiting dispatch seals", phase: PhaseWaitingDispatch, wantPolicy: ActionPolicyReadOnly, wantSealed: true},
		{name: "processing defaults readonly", phase: PhaseProcessing, wantPolicy: ActionPolicyReadOnly, wantSealed: false},
		{name: "processing keeps cancel only", phase: PhaseProcessing, policy: ActionPolicyCancelOnly, wantPolicy: ActionPolicyCancelOnly, wantSealed: false},
		{name: "terminal seals", phase: PhaseSucceeded, wantPolicy: ActionPolicyReadOnly, wantSealed: true},
		{name: "cancelled seals", phase: PhaseCancelled, wantPolicy: ActionPolicyReadOnly, wantSealed: true},
		{name: "expired seals", phase: PhaseExpired, wantPolicy: ActionPolicyReadOnly, wantSealed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := NormalizeFrame(Frame{
				OwnerKind:    OwnerCardRequest,
				Phase:        tt.phase,
				ActionPolicy: tt.policy,
			})
			if frame.ActionPolicy != tt.wantPolicy {
				t.Fatalf("action policy = %q, want %q", frame.ActionPolicy, tt.wantPolicy)
			}
			if got := SealedForPhase(frame.Phase); got != tt.wantSealed {
				t.Fatalf("sealed = %v, want %v", got, tt.wantSealed)
			}
		})
	}
}

func TestNormalizeFrameRejectsCancelOnlyOutsideProcessing(t *testing.T) {
	frame := NormalizeFrame(Frame{
		OwnerKind:    OwnerCardRequest,
		Phase:        PhaseWaitingDispatch,
		ActionPolicy: ActionPolicyCancelOnly,
	})
	if frame.ActionPolicy != ActionPolicyReadOnly {
		t.Fatalf("expected non-processing cancel_only to fall back to read_only, got %q", frame.ActionPolicy)
	}
}

func TestActionHelpersDistinguishInteractiveAndCancelOnly(t *testing.T) {
	if !AllowsPrimaryInput(ActionPolicyInteractive) {
		t.Fatal("expected interactive policy to allow primary input")
	}
	if AllowsPrimaryInput(ActionPolicyCancelOnly) {
		t.Fatal("did not expect cancel_only policy to allow primary input")
	}
	if !AllowsCancelAction(ActionPolicyCancelOnly) {
		t.Fatal("expected cancel_only policy to allow cancel action")
	}
}
