package control

import "testing"

func TestInlineCardReplacementPolicy(t *testing.T) {
	policy, ok := InlineCardReplacementPolicy(Action{
		Kind: ActionShowCommandMenu,
		Text: "/menu send_settings",
	})
	if !ok {
		t.Fatal("expected inline replacement policy for menu navigation")
	}
	if !policy.ReplaceCurrentCard || !policy.RequiresDaemonFreshness || policy.DaemonFreshness != FeishuUIInlineReplaceFreshnessDaemonLifecycle {
		t.Fatalf("unexpected daemon freshness policy: %#v", policy)
	}
	if policy.RequiresViewSession || policy.ViewSessionStrategy != FeishuUIInlineReplaceViewSessionSurfaceState {
		t.Fatalf("unexpected view/session policy: %#v", policy)
	}

	if _, ok := InlineCardReplacementPolicy(Action{
		Kind: ActionModeCommand,
		Text: "/mode vscode",
	}); ok {
		t.Fatal("expected parameter apply to stay append-only")
	}
}

func TestInlineCardReplacementPolicyActionSet(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		want   bool
	}{
		{
			name:   "menu navigation",
			action: Action{Kind: ActionShowCommandMenu, Text: "/menu send_settings"},
			want:   true,
		},
		{
			name:   "bare mode",
			action: Action{Kind: ActionModeCommand, Text: "/mode"},
			want:   true,
		},
		{
			name:   "parameter apply",
			action: Action{Kind: ActionModeCommand, Text: "/mode vscode"},
			want:   false,
		},
		{
			name:   "scoped thread expansion",
			action: Action{Kind: ActionShowScopedThreads},
			want:   true,
		},
		{
			name:   "workspace thread expansion",
			action: Action{Kind: ActionShowWorkspaceThreads},
			want:   true,
		},
		{
			name:   "workspace list expand",
			action: Action{Kind: ActionShowAllWorkspaces},
			want:   true,
		},
		{
			name:   "workspace list collapse",
			action: Action{Kind: ActionShowRecentWorkspaces},
			want:   true,
		},
		{
			name:   "thread workspace expand",
			action: Action{Kind: ActionShowAllThreadWorkspaces},
			want:   true,
		},
		{
			name:   "thread workspace collapse",
			action: Action{Kind: ActionShowRecentThreadWorkspaces},
			want:   true,
		},
		{
			name:   "thread return action",
			action: Action{Kind: ActionShowAllThreads},
			want:   true,
		},
		{
			name:   "thread attach action",
			action: Action{Kind: ActionUseThread},
			want:   false,
		},
		{
			name:   "path picker navigation",
			action: Action{Kind: ActionPathPickerEnter, PickerID: "picker-1", PickerEntry: "subdir"},
			want:   true,
		},
		{
			name:   "workspace attach action",
			action: Action{Kind: ActionAttachWorkspace},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := InlineCardReplacementPolicy(tt.action)
			if ok != tt.want {
				t.Fatalf("InlineCardReplacementPolicy(%#v) ok = %v, want %v", tt.action, ok, tt.want)
			}
		})
	}
}

func TestAllowsInlineCardReplacementRequiresDaemonFreshness(t *testing.T) {
	action := Action{
		Kind: ActionShowCommandMenu,
		Text: "/menu send_settings",
	}
	if AllowsInlineCardReplacement(action) {
		t.Fatal("expected unstamped navigation to stay async")
	}

	action.Inbound = &ActionInboundMeta{CardDaemonLifecycleID: "life-1"}
	if !AllowsInlineCardReplacement(action) {
		t.Fatal("expected stamped navigation to allow inline replacement")
	}
}

func TestAllowsInlineCardReplacementForPathPickerNavigation(t *testing.T) {
	action := Action{
		Kind:     ActionPathPickerEnter,
		PickerID: "picker-1",
		Inbound:  &ActionInboundMeta{CardDaemonLifecycleID: "life-1"},
	}
	if !AllowsInlineCardReplacement(action) {
		t.Fatal("expected inline replacement for path picker navigation")
	}
}
