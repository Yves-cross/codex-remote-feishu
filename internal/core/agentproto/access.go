package agentproto

import "strings"

const (
	AccessModeFullAccess = "full_access"
	AccessModeConfirm    = "confirm"
)

func NormalizeAccessMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "full", "fullaccess", "full_access", "full-access":
		return AccessModeFullAccess
	case "confirm", "approval", "approve", "ask", "on-request", "on_request":
		return AccessModeConfirm
	default:
		return ""
	}
}

func EffectiveAccessMode(value string) string {
	if normalized := NormalizeAccessMode(value); normalized != "" {
		return normalized
	}
	return AccessModeFullAccess
}

func ApprovalPolicyForAccessMode(value string) string {
	switch EffectiveAccessMode(value) {
	case AccessModeConfirm:
		return "on-request"
	default:
		return "never"
	}
}

func ThreadSandboxForAccessMode(value string) string {
	switch EffectiveAccessMode(value) {
	case AccessModeConfirm:
		return "workspace-write"
	default:
		return "danger-full-access"
	}
}

func TurnSandboxPolicyForAccessMode(value string) map[string]any {
	switch EffectiveAccessMode(value) {
	case AccessModeConfirm:
		return map[string]any{"type": "workspaceWrite"}
	default:
		return map[string]any{"type": "dangerFullAccess"}
	}
}

func DisplayAccessMode(value string) string {
	switch EffectiveAccessMode(value) {
	case AccessModeConfirm:
		return "confirm"
	default:
		return "full access"
	}
}

func DisplayAccessModeShort(value string) string {
	switch EffectiveAccessMode(value) {
	case AccessModeConfirm:
		return "confirm"
	default:
		return "full"
	}
}
