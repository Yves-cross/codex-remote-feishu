package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func parseMergeForwardContent(rawContent string) (string, error) {
	rawContent = strings.TrimSpace(rawContent)
	if rawContent == "" {
		return "", fmt.Errorf("empty merge_forward content")
	}
	if !looksLikeJSONObject(rawContent) {
		return rawContent, nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(rawContent), &decoded); err != nil {
		return "", err
	}
	lines := make([]string, 0, 8)
	seen := map[string]struct{}{}
	appendLine := func(text string) {
		text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		lines = append(lines, text)
	}
	collectMergeForwardLines(decoded, appendLine)
	if len(lines) == 0 {
		return "", fmt.Errorf("empty merge_forward content")
	}
	const maxLines = 24
	if len(lines) > maxLines {
		remaining := len(lines) - maxLines
		lines = append(lines[:maxLines], fmt.Sprintf("...（其余 %d 条省略）", remaining))
	}
	return strings.Join(lines, "\n"), nil
}

func (g *LiveGateway) parseMergeForwardEventContent(ctx context.Context, message *larkim.EventMessage) (string, error) {
	if message == nil {
		return "", fmt.Errorf("nil merge_forward message")
	}
	if g.fetchMessageFn != nil {
		messageID := strings.TrimSpace(stringPtr(message.MessageId))
		if messageID != "" {
			referenced, err := g.fetchMessageFn(ctx, messageID)
			if err == nil && referenced != nil && strings.EqualFold(strings.TrimSpace(referenced.MessageType), "merge_forward") {
				text, err := g.summarizeMergeForwardGatewayMessage(ctx, referenced)
				if err == nil {
					return text, nil
				}
			}
		}
	}
	return parseMergeForwardContent(stringPtr(message.Content))
}

func (g *LiveGateway) summarizeMergeForwardGatewayMessage(ctx context.Context, message *gatewayMessage) (string, error) {
	if message == nil {
		return "", fmt.Errorf("nil merge_forward message")
	}
	if len(message.Children) == 0 {
		return parseMergeForwardContent(message.Content)
	}
	lines := make([]string, 0, len(message.Children)+1)
	seen := map[string]struct{}{}
	appendLine := func(text string) {
		text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		lines = append(lines, text)
	}
	if title := mergeForwardTitle(message.Content); title != "" {
		appendLine(title)
	}
	for _, child := range message.Children {
		text, err := g.summarizeGatewayMessageWithSpeaker(ctx, child)
		if err != nil {
			continue
		}
		appendLine(text)
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n"), nil
	}
	return parseMergeForwardContent(message.Content)
}

func (g *LiveGateway) summarizeGatewayMessageWithSpeaker(ctx context.Context, message *gatewayMessage) (string, error) {
	text, err := g.summarizeGatewayMessage(ctx, message)
	if err != nil {
		return "", err
	}
	label := gatewayMessageSpeakerLabel(message)
	if label == "" {
		return text, nil
	}
	return label + ": " + text, nil
}

func (g *LiveGateway) summarizeGatewayMessage(ctx context.Context, message *gatewayMessage) (string, error) {
	if message == nil || message.Deleted {
		return "", fmt.Errorf("empty gateway message")
	}
	switch strings.ToLower(strings.TrimSpace(message.MessageType)) {
	case "text":
		return parseTextContent(message.Content)
	case "post":
		_, text, err := g.parsePostInputs(ctx, message.MessageID, message.Content)
		if err != nil {
			return "", err
		}
		return text, nil
	case "image":
		return "[图片]", nil
	case "file":
		if name := parseFileName(message.Content); name != "" {
			return "[文件] " + name, nil
		}
		return "[文件]", nil
	case "merge_forward":
		return g.summarizeMergeForwardGatewayMessage(ctx, message)
	default:
		text, err := parseMergeForwardContent(message.Content)
		if err == nil {
			return text, nil
		}
		return "", fmt.Errorf("unsupported message type: %s", message.MessageType)
	}
}

func gatewayMessageSpeakerLabel(message *gatewayMessage) string {
	if message == nil {
		return ""
	}
	senderID := strings.TrimSpace(message.SenderID)
	senderType := strings.ToLower(strings.TrimSpace(message.SenderType))
	switch senderType {
	case "user":
		if senderID == "" {
			return "用户"
		}
		return "用户(" + senderID + ")"
	case "app":
		if senderID == "" {
			return "应用"
		}
		return "应用(" + senderID + ")"
	case "anonymous":
		if senderID == "" {
			return "匿名"
		}
		return "匿名(" + senderID + ")"
	case "unknown":
		if senderID == "" {
			return "未知发送者"
		}
		return "未知发送者(" + senderID + ")"
	default:
		if senderType != "" && senderID != "" {
			return senderType + "(" + senderID + ")"
		}
		if senderID != "" {
			return "发送者(" + senderID + ")"
		}
		if senderType != "" {
			return senderType
		}
		return ""
	}
}

func mergeForwardTitle(rawContent string) string {
	rawContent = strings.TrimSpace(rawContent)
	if rawContent == "" || strings.EqualFold(rawContent, "Merged and Forwarded Message") {
		return ""
	}
	if !looksLikeJSONObject(rawContent) {
		return rawContent
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(rawContent), &payload); err != nil {
		return ""
	}
	return firstJSONString(payload, "title", "topic", "chat_name", "chat_title")
}

func collectMergeForwardLines(value any, appendLine func(string)) {
	switch current := value.(type) {
	case map[string]any:
		title := firstJSONString(current, "title", "topic", "chat_name", "chat_title")
		speaker := firstJSONString(current, "sender_name", "user_name", "name", "from_name", "sender")
		text := firstJSONString(current, "text", "message", "summary", "description", "desc")
		if text == "" {
			content := strings.TrimSpace(firstJSONString(current, "content"))
			if content != "" && !looksLikeJSONObject(content) {
				text = content
			}
		}
		if title != "" {
			appendLine(title)
		}
		if text != "" {
			if speaker != "" && !strings.EqualFold(speaker, text) {
				appendLine(speaker + ": " + text)
			} else {
				appendLine(text)
			}
		} else if len(linesFromMessageIDs(current)) > 0 {
			for _, line := range linesFromMessageIDs(current) {
				appendLine(line)
			}
		}
		for _, key := range []string{"items", "messages", "message_list", "children", "content"} {
			child, ok := current[key]
			if !ok {
				continue
			}
			collectMergeForwardLines(child, appendLine)
		}
		for key, child := range current {
			switch key {
			case "title", "topic", "chat_name", "chat_title",
				"sender_name", "user_name", "name", "from_name", "sender",
				"text", "message", "summary", "description", "desc",
				"content", "items", "messages", "message_list", "children",
				"message_id_list":
				continue
			}
			collectMergeForwardLines(child, appendLine)
		}
	case []any:
		for _, item := range current {
			collectMergeForwardLines(item, appendLine)
		}
	case string:
		text := strings.TrimSpace(current)
		if text == "" {
			return
		}
		if looksLikeJSONObject(text) {
			var nested any
			if err := json.Unmarshal([]byte(text), &nested); err == nil {
				collectMergeForwardLines(nested, appendLine)
				return
			}
		}
		appendLine(text)
	}
}

func linesFromMessageIDs(payload map[string]any) []string {
	raw, ok := payload["message_id_list"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("包含 %d 条转发消息", len(items))}
}

func looksLikeJSONObject(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return true
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return true
	}
	return false
}

func firstJSONString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch current := value.(type) {
		case string:
			if text := strings.TrimSpace(current); text != "" {
				return text
			}
		}
	}
	return ""
}

func parseFileName(rawContent string) string {
	var payload struct {
		FileName string `json:"file_name"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal([]byte(rawContent), &payload); err != nil {
		return ""
	}
	if name := strings.TrimSpace(payload.FileName); name != "" {
		return name
	}
	return strings.TrimSpace(payload.Name)
}
