package feishu

import (
	"context"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestApplySendStreamCardCreatesCardEntityAndSendsCardReference(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.createStreamCardFn = func(ctx context.Context, operation Operation) (string, error) {
		if operation.CardBody != "第一段" {
			t.Fatalf("unexpected stream body: %#v", operation)
		}
		return "card-stream-1", nil
	}
	var sentContent string
	gateway.createMessageFn = func(ctx context.Context, receiveIDType, receiveID, msgType, content string) (*larkim.CreateMessageResp, error) {
		if receiveIDType != "chat_id" || receiveID != "oc-chat-1" || msgType != "interactive" {
			t.Fatalf("unexpected send target/type: %s %s %s", receiveIDType, receiveID, msgType)
		}
		sentContent = content
		return &larkim.CreateMessageResp{
			ApiResp: &larkcore.ApiResp{},
			CodeError: larkcore.CodeError{
				Code: 0,
				Msg:  "ok",
			},
			Data: &larkim.CreateMessageRespData{MessageId: stringRef("om-stream-1")},
		}, nil
	}

	ops := []Operation{{
		Kind:      OperationSendStreamCard,
		GatewayID: "app-1",
		ChatID:    "oc-chat-1",
		CardBody:  "第一段",
	}}
	if err := gateway.Apply(t.Context(), ops); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if ops[0].MessageID != "om-stream-1" || ops[0].StreamCardID != "card-stream-1" {
		t.Fatalf("stream identifiers not written back: %#v", ops[0])
	}
	if sentContent != `{"data":{"card_id":"card-stream-1"},"type":"card"}` {
		t.Fatalf("unexpected stream card reference content: %s", sentContent)
	}
}

func TestStreamingCardDocumentOmitsHeaderWhenTitleEmpty(t *testing.T) {
	doc := streamingCardDocument("", "正文", cardThemeProgress)
	if _, ok := doc["header"]; ok {
		t.Fatalf("expected titleless streaming card to omit header, got %#v", doc["header"])
	}
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 1 || elements[0]["content"] != "正文" || elements[0]["element_id"] != "content" {
		t.Fatalf("unexpected streaming card body: %#v", doc)
	}
}

func TestStreamingCardDocumentUsesBlankContentForNativeStreaming(t *testing.T) {
	doc := streamingCardDocument("", "", cardThemeProgress)
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 1 || elements[0]["content"] != " " {
		t.Fatalf("expected blank content placeholder for native streaming, got %#v", doc)
	}
	config, _ := doc["config"].(map[string]any)
	streamingConfig, _ := config["streaming_config"].(map[string]any)
	if streamingConfig["print_strategy"] != "delay" {
		t.Fatalf("expected native streaming delay strategy, got %#v", streamingConfig)
	}
}

func TestApplyUpdateStreamCardRequiresCardID(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	err := gateway.Apply(t.Context(), []Operation{{
		Kind:      OperationUpdateStreamCard,
		GatewayID: "app-1",
		MessageID: "om-stream-1",
		CardBody:  "正文",
	}})
	if err == nil {
		t.Fatalf("expected missing card id error")
	}
}

func TestApplyCloseStreamCardUsesCardKitClose(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	var closedCardID string
	var closedText string
	gateway.closeStreamCardFn = func(ctx context.Context, cardID, text string) error {
		closedCardID = cardID
		closedText = text
		return nil
	}
	err := gateway.Apply(t.Context(), []Operation{{
		Kind:         OperationCloseStreamCard,
		GatewayID:    "app-1",
		MessageID:    "om-stream-1",
		StreamCardID: "card-stream-1",
		CardBody:     "最终答复",
	}})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if closedCardID != "card-stream-1" || closedText != "最终答复" {
		t.Fatalf("unexpected close call: card=%q text=%q", closedCardID, closedText)
	}
}
