package control

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
)

func TestNormalizeCatalogContextDefaultsToCodexDetached(t *testing.T) {
	ctx := NormalizeCatalogContext(CatalogContext{})
	if ctx.Backend != agentproto.BackendCodex {
		t.Fatalf("Backend = %q, want %q", ctx.Backend, agentproto.BackendCodex)
	}
	if ctx.ProductMode != "normal" {
		t.Fatalf("ProductMode = %q, want normal", ctx.ProductMode)
	}
	if ctx.MenuStage != string(FeishuCommandMenuStageDetached) {
		t.Fatalf("MenuStage = %q, want %q", ctx.MenuStage, FeishuCommandMenuStageDetached)
	}
	if ctx.AttachedKind != string(CatalogAttachedKindDetached) {
		t.Fatalf("AttachedKind = %q, want %q", ctx.AttachedKind, CatalogAttachedKindDetached)
	}
	if !ctx.Capabilities.ThreadsRefresh || !ctx.Capabilities.TurnSteer || !ctx.Capabilities.RequestRespond || !ctx.Capabilities.ResumeByThreadID || !ctx.Capabilities.VSCodeMode {
		t.Fatalf("expected codex default capabilities, got %#v", ctx.Capabilities)
	}
	if ctx.Capabilities.SessionCatalog || ctx.Capabilities.RequiresCWDForResume {
		t.Fatalf("unexpected codex-only capabilities: %#v", ctx.Capabilities)
	}
}
