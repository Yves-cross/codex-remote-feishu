package feishu

import "testing"

func TestRenderOperationCardLegacyEnvelopeFromOperationFields(t *testing.T) {
	payload := renderOperationCard(Operation{
		Kind:         OperationSendCard,
		CardTitle:    "当前状态",
		CardBody:     "这是正文",
		CardThemeKey: cardThemeInfo,
		CardElements: []map[string]any{{
			"tag":     "markdown",
			"content": "**附加内容**",
		}},
	}, cardEnvelopeLegacy)

	if payload["schema"] != nil {
		t.Fatalf("expected legacy envelope without schema, got %#v", payload)
	}
	header, _ := payload["header"].(map[string]any)
	title, _ := header["title"].(map[string]any)
	if title["content"] != "当前状态" {
		t.Fatalf("unexpected legacy header: %#v", payload)
	}
	elements, _ := payload["elements"].([]map[string]any)
	if len(elements) != 2 {
		t.Fatalf("expected body markdown plus extra element, got %#v", elements)
	}
}

func TestRenderOperationCardV2EnvelopeFromOperationFields(t *testing.T) {
	payload := renderOperationCard(Operation{
		Kind:         OperationSendCard,
		CardTitle:    "命令菜单",
		CardBody:     "当前在发送设置。",
		CardThemeKey: cardThemeInfo,
		CardElements: []map[string]any{{
			"tag":     "markdown",
			"content": "**发送设置**",
		}},
	}, cardEnvelopeV2)

	if payload["schema"] != "2.0" {
		t.Fatalf("expected v2 schema, got %#v", payload)
	}
	body, _ := payload["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 2 {
		t.Fatalf("expected body markdown plus extra element, got %#v", elements)
	}
	header, _ := payload["header"].(map[string]any)
	title, _ := header["title"].(map[string]any)
	if title["content"] != "命令菜单" {
		t.Fatalf("unexpected v2 header: %#v", payload)
	}
}

func TestRenderOperationCardPrefersStructuredDocument(t *testing.T) {
	payload := renderOperationCard(Operation{
		Kind:         OperationSendCard,
		CardTitle:    "legacy title",
		CardBody:     "legacy body",
		CardThemeKey: cardThemeError,
		card: newCardDocument(
			"doc title",
			cardThemeSuccess,
			cardMarkdownComponent{Content: "doc body"},
		),
	}, cardEnvelopeV2)

	header, _ := payload["header"].(map[string]any)
	title, _ := header["title"].(map[string]any)
	if title["content"] != "doc title" {
		t.Fatalf("expected structured card title to win, got %#v", payload)
	}
	if header["template"] != "green" {
		t.Fatalf("expected structured card theme to win, got %#v", payload)
	}
	body, _ := payload["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 1 || elements[0]["content"] != "doc body" {
		t.Fatalf("expected structured card body to win, got %#v", payload)
	}
}
