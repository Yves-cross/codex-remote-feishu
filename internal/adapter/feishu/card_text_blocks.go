package feishu

import "strings"

func cardPlainTextBlockElement(content string) map[string]any {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	return map[string]any{
		"tag": "div",
		"text": map[string]any{
			"tag":     "plain_text",
			"content": content,
		},
	}
}
