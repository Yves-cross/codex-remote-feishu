package control

import "strings"

const (
	FeishuUIInlineReplaceFreshnessDaemonLifecycle = "daemon_lifecycle"
	FeishuUIInlineReplaceViewSessionSurfaceState  = "surface_state_rederived"
	FeishuUIInlineReplaceOwnerController          = "feishu_ui_controller"
)

// FeishuUIInlineReplacePolicy makes the current inline-replace lifecycle
// strategy explicit without changing user-visible behavior.
type FeishuUIInlineReplacePolicy struct {
	Owner                   string
	ReplaceCurrentCard      bool
	RequiresDaemonFreshness bool
	DaemonFreshness         string
	RequiresViewSession     bool
	ViewSessionStrategy     string
}

func InlineCardReplacementPolicy(action Action) (FeishuUIInlineReplacePolicy, bool) {
	if _, ok := FeishuUIIntentFromAction(action); !ok {
		return FeishuUIInlineReplacePolicy{}, false
	}
	return FeishuUIInlineReplacePolicy{
		Owner:                   FeishuUIInlineReplaceOwnerController,
		ReplaceCurrentCard:      true,
		RequiresDaemonFreshness: true,
		DaemonFreshness:         FeishuUIInlineReplaceFreshnessDaemonLifecycle,
		RequiresViewSession:     false,
		ViewSessionStrategy:     FeishuUIInlineReplaceViewSessionSurfaceState,
	}, true
}

func AllowsInlineCardReplacement(action Action) bool {
	policy, ok := InlineCardReplacementPolicy(action)
	if !ok || !policy.ReplaceCurrentCard {
		return false
	}
	if !policy.RequiresDaemonFreshness {
		return true
	}
	return action.Inbound != nil && strings.TrimSpace(action.Inbound.CardDaemonLifecycleID) != ""
}
