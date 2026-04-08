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
			return control.Action{}, false, err
		}
		if len(inputs) == 0 {
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
			return control.Action{}, false, err
		}
		path, mimeType, err := g.downloadImageFn(ctx, stringPtr(message.MessageId), imageKey)
		if err != nil {
			return control.Action{}, false, err
		}
		action.Kind = control.ActionImageMessage
		action.LocalPath = path
		action.MIMEType = mimeType
		g.recordSurfaceMessage(action.MessageID, surfaceSessionID)
		return action, true, nil
	default:
		return control.Action{}, false, nil
	}
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
	item := resp.Data.Items[0]
	content := ""
	if item.Body != nil {
		content = stringPtr(item.Body.Content)
	}
	return &gatewayMessage{
		MessageID:   stringPtr(item.MessageId),
		MessageType: stringPtr(item.MsgType),
		Content:     content,
		Deleted:     boolPtr(item.Deleted),
	}, nil
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
