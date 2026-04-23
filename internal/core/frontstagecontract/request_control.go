package frontstagecontract

import "strings"

const (
	RequestControlCancelTurn    = "cancel_turn"
	RequestControlCancelRequest = "cancel_request"
	RequestControlSkipOptional  = "skip_optional"

	RequestPromptOptionStepPrevious = "step_previous"
	RequestPromptOptionStepNext     = "step_next"
)

func NormalizeRequestControlToken(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}
