package frontstagecontract

import "testing"

func TestNormalizeRequestControlTokenTreatsSeparatorsEqually(t *testing.T) {
	want := NormalizeRequestControlToken(RequestControlCancelTurn)
	inputs := []string{
		" cancel_turn ",
		"cancel-turn",
		"cancel turn",
		"CANCEL_TURN",
	}
	for _, input := range inputs {
		if got := NormalizeRequestControlToken(input); got != want {
			t.Fatalf("NormalizeRequestControlToken(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeRequestControlTokenSupportsStepOptions(t *testing.T) {
	want := NormalizeRequestControlToken(RequestPromptOptionStepNext)
	if got := NormalizeRequestControlToken("step-next"); got != want {
		t.Fatalf("NormalizeRequestControlToken(step-next) = %q, want %q", got, want)
	}
}
