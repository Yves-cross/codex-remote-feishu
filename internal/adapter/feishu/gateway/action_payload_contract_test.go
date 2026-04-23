package gateway

import "testing"

func TestActionPayloadKindReadsCanonicalKindKey(t *testing.T) {
	value := map[string]any{
		cardActionPayloadKeyKind: "  " + cardActionKindShowAllThreads + "  ",
	}
	if got := actionPayloadKind(value); got != cardActionKindShowAllThreads {
		t.Fatalf("actionPayloadKind() = %q, want %q", got, cardActionKindShowAllThreads)
	}
}
