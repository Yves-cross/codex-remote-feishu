package feishu

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/eventcontract"
	"github.com/kxn/codex-remote-feishu/internal/core/render"
)

func TestProjectFinalAssistantBlockPreservesExplicitRemoteMarkdownLinks(t *testing.T) {
	projector := NewProjector()
	ops := projector.ProjectEvent("chat-1", eventcontract.Event{
		Kind:            eventcontract.KindBlockCommitted,
		SourceMessageID: "msg-remote-link",
		Block: &render.Block{
			Kind:        render.BlockAssistantMarkdown,
			Text:        "查看 [issue 227](https://github.com/kxn/codex-remote-feishu/issues/227)。",
			ThreadID:    "thread-1",
			ThreadTitle: "droid · 修复登录流程",
			ThemeKey:    "thread-1",
			Final:       true,
		},
	})
	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	want := "查看 [issue 227](https://github.com/kxn/codex-remote-feishu/issues/227)。"
	if ops[0].CardBody != want {
		t.Fatalf("unexpected final markdown body: %#v", ops[0])
	}
	elements := renderedMarkdownElementContents(t, ops[0])
	if len(elements) == 0 || elements[0] != want {
		t.Fatalf("unexpected rendered markdown elements: %#v", elements)
	}
}

func TestProjectFinalAssistantBlockNeutralizesUnsupportedLocalMarkdownLinks(t *testing.T) {
	projector := NewProjector()
	ops := projector.ProjectEvent("chat-1", eventcontract.Event{
		Kind:            eventcontract.KindBlockCommitted,
		SourceMessageID: "msg-local-link",
		Block: &render.Block{
			Kind:        render.BlockAssistantMarkdown,
			Text:        "先看 [Guide](./docs/guide.md:12)，再看 [RFC](https://example.com/rfc)。",
			ThreadID:    "thread-1",
			ThreadTitle: "droid · 修复登录流程",
			ThemeKey:    "thread-1",
			Final:       true,
		},
	})
	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	want := "先看 Guide (`./docs/guide.md:12`)，再看 [RFC](https://example.com/rfc)。"
	if ops[0].CardBody != want {
		t.Fatalf("unexpected final markdown body: %#v", ops[0])
	}
	if ops[0].FinalSourceBody() != "先看 [Guide](./docs/guide.md:12)，再看 [RFC](https://example.com/rfc)。" {
		t.Fatalf("expected final source body to retain pre-render markdown, got %#v", ops[0])
	}
	elements := renderedMarkdownElementContents(t, ops[0])
	if len(elements) == 0 || elements[0] != want {
		t.Fatalf("unexpected rendered markdown elements: %#v", elements)
	}
}

func TestProjectFinalAssistantBlockSkipsCodeWhenNormalizingLocalLinks(t *testing.T) {
	projector := NewProjector()
	ops := projector.ProjectEvent("chat-1", eventcontract.Event{
		Kind:            eventcontract.KindBlockCommitted,
		SourceMessageID: "msg-code-link",
		Block: &render.Block{
			Kind: render.BlockAssistantMarkdown,
			Text: "外部 [Guide](./docs/guide.md:12)\n\n" +
				"inline `[Inline](./docs/inline.md:3)`\n\n" +
				"```md\n[Keep](./docs/keep.md:8)\n```\n\n" +
				"最后 [RFC](https://example.com/rfc)。",
			ThreadID:    "thread-1",
			ThreadTitle: "droid · 修复登录流程",
			ThemeKey:    "thread-1",
			Final:       true,
		},
	})
	if len(ops) != 1 || ops[0].Kind != OperationSendCard {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	want := "外部 Guide (`./docs/guide.md:12`)\n\n" +
		"inline `[Inline](./docs/inline.md:3)`\n\n" +
		"```md\n[Keep](./docs/keep.md:8)\n```\n\n" +
		"最后 [RFC](https://example.com/rfc)。"
	if ops[0].CardBody != want {
		t.Fatalf("unexpected final markdown body: %#v", ops[0])
	}
	elements := renderedMarkdownElementContents(t, ops[0])
	if len(elements) == 0 || elements[0] != want {
		t.Fatalf("unexpected rendered markdown elements: %#v", elements)
	}
}

func renderedMarkdownElementContents(t *testing.T, operation Operation) []string {
	t.Helper()
	payload := renderOperationCard(operation, operation.effectiveCardEnvelope())
	assertRenderedCardPayloadBasicInvariants(t, payload)
	body, _ := payload["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	values := make([]string, 0, len(elements))
	for _, element := range elements {
		if cardStringValue(element["tag"]) != "markdown" {
			continue
		}
		values = append(values, cardStringValue(element["content"]))
	}
	return values
}
