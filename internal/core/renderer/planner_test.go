package renderer

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/render"
)

func TestPlannerPreservesFullAssistantMarkdownAsSingleBlock(t *testing.T) {
	planner := NewPlanner()
	input := "我先列一下当前目录文件。\n\n```text\nREADME.md\nsrc\n```\n\n如果你要，我可以继续只列目录。"
	blocks := planner.PlanAssistantBlocks(
		"surface-1",
		"inst-1",
		"thread-1",
		"turn-1",
		"item-1",
		input,
	)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != render.BlockAssistantMarkdown {
		t.Fatalf("unexpected block kind: %#v", blocks[0])
	}
	if blocks[0].Text != input {
		t.Fatalf("unexpected block text: %#v", blocks[0])
	}
}
