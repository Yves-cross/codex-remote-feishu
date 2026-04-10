package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/kxn/codex-remote-feishu/internal/core/agentproto"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func (g *LiveGateway) parseMessageEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) (control.Action, bool, error) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return control.Action{}, false, nil
	}
	message := event.Event.Message
	chatID := stringPtr(message.ChatId)
	chatType := stringPtr(message.ChatType)
	senderUserID := userIDFromMessage(event.Event.Sender)
	surfaceSessionID := surfaceIDForInbound(g.config.GatewayID, chatID, chatType, senderUserID)
	action := control.Action{
		GatewayID:        g.config.GatewayID,
		SurfaceSessionID: surfaceSessionID,
		ChatID:           chatID,
		ActorUserID:      senderUserID,
		MessageID:        stringPtr(message.MessageId),
		Inbound:          inboundMetaFromMessageEvent(event),
	}

	switch strings.ToLower(stringPtr(message.MessageType)) {
	case "text":
		text, err := parseTextContent(stringPtr(message.Content))
		if err != nil {
			logInboundMessageParseFailed(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "parse_text_content", err)
			return control.Action{}, false, err
		}
		commandAction, handled := parseTextAction(text)
		if handled {
			commandAction.GatewayID = g.config.GatewayID
			commandAction.SurfaceSessionID = surfaceSessionID
			commandAction.ChatID = chatID
			commandAction.ActorUserID = action.ActorUserID
			commandAction.MessageID = action.MessageID
			commandAction.Inbound = action.Inbound
			return commandAction, true, nil
		}
		inputs := []agentproto.Input{{Type: agentproto.InputText, Text: text}}
		inputs = append(g.quotedInputs(ctx, message), inputs...)
		action.Kind = control.ActionTextMessage
		action.Text = text
		action.Inputs = inputs
		g.recordSurfaceMessage(action.MessageID, surfaceSessionID)
		return action, true, nil
	case "post":
		inputs, text, err := g.parsePostInputs(ctx, action.MessageID, stringPtr(message.Content))
		if err != nil {
			logInboundMessageParseFailed(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "parse_post_content", err)
			return control.Action{}, false, err
		}
		if len(inputs) == 0 {
			logInboundMessageIgnored(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "empty_post_inputs")
			return control.Action{}, false, nil
		}
		action.Kind = control.ActionTextMessage
		action.Text = text
		action.Inputs = append(g.quotedInputs(ctx, message), inputs...)
		g.recordSurfaceMessage(action.MessageID, surfaceSessionID)
		return action, true, nil
	case "image":
		imageKey, err := parseImageKey(stringPtr(message.Content))
		if err != nil {
			logInboundMessageParseFailed(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "parse_image_content", err)
			return control.Action{}, false, err
		}
		path, mimeType, err := g.downloadImageFn(ctx, stringPtr(message.MessageId), imageKey)
		if err != nil {
			logInboundMessageParseFailed(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "download_image", err)
			return control.Action{}, false, err
		}
		action.Kind = control.ActionImageMessage
		action.LocalPath = path
		action.MIMEType = mimeType
		g.recordSurfaceMessage(action.MessageID, surfaceSessionID)
		return action, true, nil
	case "merge_forward":
		text, err := g.parseMergeForwardEventContent(ctx, message)
		if err != nil {
			logInboundMessageParseFailed(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "parse_merge_forward_content", err)
			return control.Action{}, false, err
		}
		merged := mergeForwardTextInput(text)
		if merged.Text == "" {
			logInboundMessageIgnored(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "empty_merge_forward_content")
			return control.Action{}, false, nil
		}
		action.Kind = control.ActionTextMessage
		action.Text = text
		action.Inputs = append(g.quotedInputs(ctx, message), merged)
		g.recordSurfaceMessage(action.MessageID, surfaceSessionID)
		return action, true, nil
	default:
		logInboundMessageIgnored(g.config.GatewayID, surfaceSessionID, action.Inbound, message, "unsupported_message_type")
		return control.Action{}, false, nil
	}
}

func logInboundMessageIgnored(gatewayID, surfaceSessionID string, inbound *control.ActionInboundMeta, message *larkim.EventMessage, reason string) {
	log.Printf(
		"feishu inbound message ignored: gateway=%s surface=%s message=%s type=%s chat=%s chat_type=%s thread=%s root=%s parent=%s event=%s request=%s reason=%s preview=%q",
		strings.TrimSpace(gatewayID),
		strings.TrimSpace(surfaceSessionID),
		strings.TrimSpace(stringPtr(message.MessageId)),
		strings.ToLower(strings.TrimSpace(stringPtr(message.MessageType))),
		strings.TrimSpace(stringPtr(message.ChatId)),
		strings.TrimSpace(stringPtr(message.ChatType)),
		strings.TrimSpace(stringPtr(message.ThreadId)),
		strings.TrimSpace(stringPtr(message.RootId)),
		strings.TrimSpace(stringPtr(message.ParentId)),
		inboundMetaValue(inbound, func(meta *control.ActionInboundMeta) string { return meta.EventID }),
		inboundMetaValue(inbound, func(meta *control.ActionInboundMeta) string { return meta.RequestID }),
		strings.TrimSpace(reason),
		inboundMessagePreview(message),
	)
}

func logInboundMessageParseFailed(gatewayID, surfaceSessionID string, inbound *control.ActionInboundMeta, message *larkim.EventMessage, reason string, err error) {
	log.Printf(
		"feishu inbound message parse failed: gateway=%s surface=%s message=%s type=%s chat=%s chat_type=%s thread=%s root=%s parent=%s event=%s request=%s reason=%s err=%v preview=%q",
		strings.TrimSpace(gatewayID),
		strings.TrimSpace(surfaceSessionID),
		strings.TrimSpace(stringPtr(message.MessageId)),
		strings.ToLower(strings.TrimSpace(stringPtr(message.MessageType))),
		strings.TrimSpace(stringPtr(message.ChatId)),
		strings.TrimSpace(stringPtr(message.ChatType)),
		strings.TrimSpace(stringPtr(message.ThreadId)),
		strings.TrimSpace(stringPtr(message.RootId)),
		strings.TrimSpace(stringPtr(message.ParentId)),
		inboundMetaValue(inbound, func(meta *control.ActionInboundMeta) string { return meta.EventID }),
		inboundMetaValue(inbound, func(meta *control.ActionInboundMeta) string { return meta.RequestID }),
		strings.TrimSpace(reason),
		err,
		inboundMessagePreview(message),
	)
}

func inboundMetaValue(meta *control.ActionInboundMeta, pick func(*control.ActionInboundMeta) string) string {
	if meta == nil || pick == nil {
		return ""
	}
	return strings.TrimSpace(pick(meta))
}

func inboundMessagePreview(message *larkim.EventMessage) string {
	if message == nil {
		return ""
	}
	messageType := strings.ToLower(strings.TrimSpace(stringPtr(message.MessageType)))
	rawContent := strings.TrimSpace(stringPtr(message.Content))
	switch messageType {
	case "text":
		text, err := parseTextContent(rawContent)
		if err == nil {
			return trimLogPreview(text)
		}
	case "post":
		var content feishuPostContent
		if err := json.Unmarshal([]byte(rawContent), &content); err == nil {
			textParts := make([]string, 0, len(content.Content)+1)
			if title := strings.TrimSpace(content.Title); title != "" {
				textParts = append(textParts, title)
			}
			for _, paragraph := range content.Content {
				var segment strings.Builder
				for _, node := range paragraph {
					switch strings.ToLower(strings.TrimSpace(node.Tag)) {
					case "text":
						segment.WriteString(node.Text)
					case "a":
						if text := strings.TrimSpace(node.Text); text != "" {
							segment.WriteString(text)
						}
					case "at":
						if text := strings.TrimSpace(node.Text); text != "" {
							segment.WriteString(text)
						}
					case "emotion":
						if emoji := strings.TrimSpace(node.EmojiType); emoji != "" {
							segment.WriteString(":" + emoji + ":")
						}
					case "code_block":
						if text := strings.TrimSpace(node.Text); text != "" {
							segment.WriteString(text)
						}
					}
				}
				if text := strings.TrimSpace(segment.String()); text != "" {
					textParts = append(textParts, text)
				}
			}
			if len(textParts) > 0 {
				return trimLogPreview(strings.Join(textParts, "\n\n"))
			}
		}
	case "merge_forward":
		text, err := parseMergeForwardContent(rawContent)
		if err == nil {
			return trimLogPreview(text)
		}
	}
	return trimLogPreview(rawContent)
}

func trimLogPreview(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	const maxPreviewRunes = 160
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxPreviewRunes {
		return text
	}
	return string(runes[:maxPreviewRunes]) + "..."
}

func (g *LiveGateway) parseMessageRecalledEvent(event *larkim.P2MessageRecalledV1) (control.Action, bool) {
	if event == nil || event.Event == nil || event.Event.MessageId == nil {
		return control.Action{}, false
	}
	messageID := strings.TrimSpace(*event.Event.MessageId)
	if messageID == "" {
		return control.Action{}, false
	}
	g.mu.Lock()
	surfaceSessionID := g.messages[messageID]
	g.mu.Unlock()
	if surfaceSessionID == "" {
		return control.Action{}, false
	}
	return control.Action{
		Kind:             control.ActionMessageRecalled,
		GatewayID:        g.config.GatewayID,
		SurfaceSessionID: surfaceSessionID,
		ChatID:           strings.TrimSpace(stringPtr(event.Event.ChatId)),
		TargetMessageID:  messageID,
		Inbound:          inboundMetaFromMessageRecalledEvent(event),
	}, true
}

func (g *LiveGateway) parseMessageReactionCreatedEvent(event *larkim.P2MessageReactionCreatedV1) (control.Action, bool) {
	if event == nil || event.Event == nil || event.Event.MessageId == nil || event.Event.ReactionType == nil {
		return control.Action{}, false
	}
	messageID := strings.TrimSpace(*event.Event.MessageId)
	if messageID == "" {
		return control.Action{}, false
	}
	reactionType := strings.TrimSpace(stringPtr(event.Event.ReactionType.EmojiType))
	if reactionType == "" {
		return control.Action{}, false
	}
	actorUserID := userIDFromLarkUserID(event.Event.UserId)
	if actorUserID == "" {
		return control.Action{}, false
	}
	g.mu.Lock()
	surfaceSessionID := g.messages[messageID]
	g.mu.Unlock()
	if surfaceSessionID == "" {
		return control.Action{}, false
	}
	return control.Action{
		Kind:             control.ActionReactionCreated,
		GatewayID:        g.config.GatewayID,
		SurfaceSessionID: surfaceSessionID,
		ActorUserID:      actorUserID,
		ReactionType:     reactionType,
		TargetMessageID:  messageID,
		Inbound:          inboundMetaFromMessageReactionCreatedEvent(event),
	}, true
}

func (g *LiveGateway) parseMenuEvent(event *larkapplication.P2BotMenuV6) (control.Action, bool) {
	if event == nil || event.Event == nil || event.Event.EventKey == nil {
		return control.Action{}, false
	}
	rawKey := *event.Event.EventKey
	action, ok := menuAction(rawKey)
	if !ok {
		log.Printf("feishu bot menu ignored: raw_key=%q normalized=%q", rawKey, normalizeMenuEventKey(rawKey))
		return control.Action{}, false
	}
	log.Printf("feishu bot menu handled: raw_key=%q normalized=%q action=%s", rawKey, normalizeMenuEventKey(rawKey), action.Kind)
	operatorID := operatorUserID(event.Event.Operator)
	action.GatewayID = g.config.GatewayID
	action.SurfaceSessionID = surfaceIDForInbound(g.config.GatewayID, "", "p2p", operatorID)
	action.ActorUserID = operatorID
	action.Inbound = inboundMetaFromMenuEvent(event)
	return action, true
}

func parseTextContent(rawContent string) (string, error) {
	var content feishuTextContent
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		return "", err
	}
	return content.Text, nil
}

func parseImageKey(rawContent string) (string, error) {
	var content struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		return "", err
	}
	if strings.TrimSpace(content.ImageKey) == "" {
		return "", fmt.Errorf("missing image_key")
	}
	return strings.TrimSpace(content.ImageKey), nil
}

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

func (g *LiveGateway) quotedInputs(ctx context.Context, message *larkim.EventMessage) []agentproto.Input {
	if message == nil || g.fetchMessageFn == nil {
		return nil
	}
	targetMessageID := strings.TrimSpace(stringPtr(message.ParentId))
	if targetMessageID == "" {
		targetMessageID = strings.TrimSpace(stringPtr(message.RootId))
	}
	if targetMessageID == "" {
		return nil
	}
	referenced, err := g.fetchMessageFn(ctx, targetMessageID)
	if err != nil {
		log.Printf("feishu quote fetch ignored: message=%s err=%v", targetMessageID, err)
		return nil
	}
	return g.inputsFromReferencedMessage(ctx, referenced)
}

func (g *LiveGateway) inputsFromReferencedMessage(ctx context.Context, referenced *gatewayMessage) []agentproto.Input {
	if referenced == nil || referenced.Deleted {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(referenced.MessageType)) {
	case "text":
		text, err := parseTextContent(referenced.Content)
		if err != nil {
			log.Printf("feishu quote text parse ignored: message=%s err=%v", referenced.MessageID, err)
			return nil
		}
		if wrapped := quotedTextInput(text); wrapped.Text != "" {
			return []agentproto.Input{wrapped}
		}
		return nil
	case "post":
		inputs, text, err := g.parsePostInputs(ctx, referenced.MessageID, referenced.Content)
		if err != nil {
			log.Printf("feishu quote post parse ignored: message=%s err=%v", referenced.MessageID, err)
			return nil
		}
		quoted := make([]agentproto.Input, 0, len(inputs)+1)
		if wrapped := quotedTextInput(text); wrapped.Text != "" {
			quoted = append(quoted, wrapped)
		}
		for _, input := range inputs {
			if input.Type == agentproto.InputLocalImage || input.Type == agentproto.InputRemoteImage {
				quoted = append(quoted, input)
			}
		}
		return quoted
	case "image":
		imageKey, err := parseImageKey(referenced.Content)
		if err != nil {
			log.Printf("feishu quote image parse ignored: message=%s err=%v", referenced.MessageID, err)
			return nil
		}
		path, mimeType, err := g.downloadImageFn(ctx, referenced.MessageID, imageKey)
		if err != nil {
			log.Printf("feishu quote image download ignored: message=%s err=%v", referenced.MessageID, err)
			return nil
		}
		return []agentproto.Input{{Type: agentproto.InputLocalImage, Path: path, MIMEType: mimeType}}
	case "merge_forward":
		text, err := g.summarizeMergeForwardGatewayMessage(ctx, referenced)
		if err != nil {
			log.Printf("feishu quote merge_forward parse ignored: message=%s err=%v", referenced.MessageID, err)
			return nil
		}
		if wrapped := mergeForwardTextInput(text); wrapped.Text != "" {
			return []agentproto.Input{wrapped}
		}
		return nil
	default:
		return nil
	}
}

func quotedTextInput(text string) agentproto.Input {
	text = strings.TrimSpace(text)
	if text == "" {
		return agentproto.Input{}
	}
	return agentproto.Input{
		Type: agentproto.InputText,
		Text: "<被引用内容>\n" + text + "\n</被引用内容>",
	}
}

func mergeForwardTextInput(text string) agentproto.Input {
	text = strings.TrimSpace(text)
	if text == "" {
		return agentproto.Input{}
	}
	return agentproto.Input{
		Type: agentproto.InputText,
		Text: "<转发聊天记录>\n" + text + "\n</转发聊天记录>",
	}
}

func (g *LiveGateway) parsePostInputs(ctx context.Context, messageID, rawContent string) ([]agentproto.Input, string, error) {
	var content feishuPostContent
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		return nil, "", err
	}
	inputs := make([]agentproto.Input, 0, len(content.Content)+1)
	textParts := make([]string, 0, len(content.Content)+1)
	if title := strings.TrimSpace(content.Title); title != "" {
		inputs = append(inputs, agentproto.Input{Type: agentproto.InputText, Text: title})
		textParts = append(textParts, title)
	}
	for _, paragraph := range content.Content {
		var segment strings.Builder
		flushText := func() {
			text := strings.TrimSpace(segment.String())
			segment.Reset()
			if text == "" {
				return
			}
			inputs = append(inputs, agentproto.Input{Type: agentproto.InputText, Text: text})
			textParts = append(textParts, text)
		}
		for _, node := range paragraph {
			switch strings.ToLower(strings.TrimSpace(node.Tag)) {
			case "text":
				segment.WriteString(node.Text)
			case "a":
				switch {
				case strings.TrimSpace(node.Text) != "" && strings.TrimSpace(node.Href) != "":
					segment.WriteString(strings.TrimSpace(node.Text) + " (" + strings.TrimSpace(node.Href) + ")")
				case strings.TrimSpace(node.Text) != "":
					segment.WriteString(node.Text)
				case strings.TrimSpace(node.Href) != "":
					segment.WriteString(strings.TrimSpace(node.Href))
				}
			case "at":
				switch {
				case strings.TrimSpace(node.Text) != "":
					segment.WriteString(node.Text)
				case strings.TrimSpace(node.UserName) != "":
					segment.WriteString("@" + strings.TrimSpace(node.UserName))
				case strings.TrimSpace(node.UserID) != "":
					segment.WriteString("@" + strings.TrimSpace(node.UserID))
				}
			case "emotion":
				if emoji := strings.TrimSpace(node.EmojiType); emoji != "" {
					segment.WriteString(":" + emoji + ":")
				}
			case "code_block":
				code := strings.TrimSpace(node.Text)
				if code == "" {
					continue
				}
				if segment.Len() > 0 {
					segment.WriteString("\n")
				}
				if language := strings.TrimSpace(node.Language); language != "" {
					segment.WriteString("```" + language + "\n" + code + "\n```")
				} else {
					segment.WriteString("```\n" + code + "\n```")
				}
			case "img", "media":
				if strings.TrimSpace(node.ImageKey) == "" {
					continue
				}
				flushText()
				path, mimeType, err := g.downloadImageFn(ctx, messageID, strings.TrimSpace(node.ImageKey))
				if err != nil {
					return nil, "", err
				}
				inputs = append(inputs, agentproto.Input{Type: agentproto.InputLocalImage, Path: path, MIMEType: mimeType})
			}
		}
		flushText()
	}
	return inputs, strings.Join(textParts, "\n\n"), nil
}

func (g *LiveGateway) fetchMessage(ctx context.Context, messageID string) (*gatewayMessage, error) {
	resp, err := g.client.Im.V1.Message.Get(ctx, larkim.NewGetMessageReqBuilder().
		MessageId(messageID).
		Build())
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("get message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 || resp.Data.Items[0] == nil {
		return nil, fmt.Errorf("get message failed: empty response")
	}
	items := make([]*gatewayMessage, 0, len(resp.Data.Items))
	index := make(map[string]*gatewayMessage, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		if item == nil {
			continue
		}
		content := ""
		if item.Body != nil {
			content = stringPtr(item.Body.Content)
		}
		msg := &gatewayMessage{
			MessageID:      stringPtr(item.MessageId),
			MessageType:    stringPtr(item.MsgType),
			Content:        content,
			Deleted:        boolPtr(item.Deleted),
			UpperMessageID: stringPtr(item.UpperMessageId),
		}
		if item.Sender != nil {
			msg.SenderID = stringPtr(item.Sender.Id)
			msg.SenderType = stringPtr(item.Sender.SenderType)
		}
		items = append(items, msg)
		if msg.MessageID != "" {
			index[msg.MessageID] = msg
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("get message failed: empty response")
	}
	root := index[messageID]
	if root == nil {
		root = items[0]
	}
	for _, item := range items {
		if item == nil || item == root {
			continue
		}
		parentID := strings.TrimSpace(item.UpperMessageID)
		if parentID == "" {
			continue
		}
		parent := index[parentID]
		if parent == nil {
			continue
		}
		parent.Children = append(parent.Children, item)
	}
	return root, nil
}

func (g *LiveGateway) downloadImage(ctx context.Context, messageID, imageKey string) (string, string, error) {
	resp, err := g.client.Im.V1.MessageResource.Get(ctx, larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(imageKey).
		Type("image").
		Build())
	if err != nil {
		return "", "", err
	}
	if !resp.Success() {
		return "", "", fmt.Errorf("download image failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	dir := g.config.TempDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	file, err := os.CreateTemp(dir, "codex-remote-image-*")
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	bytes, err := io.ReadAll(resp.File)
	if err != nil {
		return "", "", err
	}
	if _, err := file.Write(bytes); err != nil {
		return "", "", err
	}
	if err := file.Close(); err != nil {
		return "", "", err
	}
	mimeType := http.DetectContentType(bytes)
	target := file.Name()
	if ext := mimeExtension(mimeType); ext != "" && !strings.HasSuffix(target, ext) {
		renamed := target + ext
		if err := os.Rename(target, renamed); err == nil {
			target = renamed
		}
	}
	return target, mimeType, nil
}

func boolPtr(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}
