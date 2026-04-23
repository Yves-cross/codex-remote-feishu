package projector

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestWorkspaceSelectionPromptRecoverableOptionUsesWorkspaceThreadsAction(t *testing.T) {
	prompt := workspaceSelectionPromptFromView(control.FeishuWorkspaceSelectionView{
		Entries: []control.FeishuWorkspaceSelectionEntry{{
			WorkspaceKey:    "ws-1",
			WorkspaceLabel:  "Workspace 1",
			RecoverableOnly: true,
		}},
	}, nil)
	if len(prompt.Options) != 1 {
		t.Fatalf("expected one option, got %#v", prompt.Options)
	}
	if prompt.Options[0].ActionKind != "show_workspace_threads" {
		t.Fatalf("unexpected recoverable action kind: %#v", prompt.Options[0])
	}
}
