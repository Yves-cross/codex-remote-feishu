package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	doc := streamingCardDocument("", "正文", cardThemeProgress, "", false)
	if _, ok := doc["header"]; ok {
		t.Fatalf("expected titleless streaming card to omit header, got %#v", doc["header"])
	}
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 2 || elements[0]["content"] != "正文" || elements[0]["element_id"] != "content" {
		t.Fatalf("unexpected streaming card body: %#v", doc)
	}
	if elements[1]["element_id"] != "loading" {
		t.Fatalf("expected dedicated loading element, got %#v", elements)
	}
}

func TestStreamingCardDocumentUsesBlankContentForNativeStreaming(t *testing.T) {
	doc := streamingCardDocument("", "", cardThemeProgress, "", true)
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 2 || elements[0]["content"] != "" {
		t.Fatalf("expected empty initial content for native streaming prefix matching, got %#v", doc)
	}
	config, _ := doc["config"].(map[string]any)
	streamingConfig, _ := config["streaming_config"].(map[string]any)
	if streamingConfig["print_strategy"] != "fast" {
		t.Fatalf("expected native streaming fast strategy, got %#v", streamingConfig)
	}
	if elements[1]["content"] != "<text_tag color='neutral'>...</text_tag>" {
		t.Fatalf("expected loading fallback marker, got %#v", elements[1])
	}
}

func TestStreamingCardDocumentUsesGIFLoadingElementWhenImageKeyPresent(t *testing.T) {
	doc := streamingCardDocument("", "正文", cardThemeProgress, "img-loading-1", true)
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 2 {
		t.Fatalf("unexpected element count: %#v", elements)
	}
	if elements[1]["tag"] != "column_set" || elements[1]["element_id"] != "loading" {
		t.Fatalf("expected gif loading image element, got %#v", elements[1])
	}
	columns, _ := elements[1]["columns"].([]map[string]any)
	if len(columns) != 1 {
		t.Fatalf("expected one auto-width column, got %#v", elements[1])
	}
	columnElements, _ := columns[0]["elements"].([]map[string]any)
	if len(columnElements) != 1 || columnElements[0]["tag"] != "img" || columnElements[0]["img_key"] != "img-loading-1" {
		t.Fatalf("expected column-scoped loading image, got %#v", elements[1])
	}
	if elements[1]["horizontal_spacing"] != "none" {
		t.Fatalf("expected loading column set to avoid extra spacing, got %#v", elements[1])
	}
	if columnElements[0]["custom_width"] != 8 || columnElements[0]["compact_width"] != true {
		t.Fatalf("expected app-tight loading image sizing, got %#v", columnElements[0])
	}
	if _, ok := columnElements[0]["mode"]; ok {
		t.Fatalf("expected loading image to avoid fit mode expansion, got %#v", columnElements[0])
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

func TestUpdateStreamCardTreatsAlreadyClosedAsIdempotent(t *testing.T) {
	var updateCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			updateCalls++
			writeJSON(t, w, map[string]any{"code": 300309, "msg": "ErrMsg: streaming mode is closed;"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1", Domain: server.URL})
	gateway.tenantAccessToken = "tenant-token"
	gateway.tenantTokenExpiresAt = timeNowPlusHour()
	gateway.streamSeq["card-stream-1"] = 7

	if err := gateway.updateStreamCard(t.Context(), "card-stream-1", "stale update", true); err != nil {
		t.Fatalf("expected already-closed update to be ignored, got %v", err)
	}
	if updateCalls != 1 {
		t.Fatalf("expected one update request, got %d", updateCalls)
	}
	if _, ok := gateway.streamSeq["card-stream-1"]; ok {
		t.Fatalf("expected closed stream card sequence to be forgotten")
	}
}

func TestCloseStreamCardHidesLoadingElementBeforeClosing(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case "/open-apis/cardkit/v1/cards/card-stream-1/elements/loading":
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1", Domain: server.URL})
	gateway.tenantAccessToken = "tenant-token"
	gateway.tenantTokenExpiresAt = timeNowPlusHour()
	gateway.streamSeq["card-stream-1"] = 1
	gateway.streamLoadingShown["card-stream-1"] = true

	if err := gateway.closeStreamCard(t.Context(), "card-stream-1", "最终答复"); err != nil {
		t.Fatalf("closeStreamCard returned error: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected content update, loading hide, settings close; got %#v", paths)
	}
}

func TestCloseStreamCardTreatsPreCloseAlreadyClosedAsIdempotent(t *testing.T) {
	var settingsCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			writeJSON(t, w, map[string]any{"code": 300309, "msg": "ErrMsg: streaming mode is closed;"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			settingsCalls++
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1", Domain: server.URL})
	gateway.tenantAccessToken = "tenant-token"
	gateway.tenantTokenExpiresAt = timeNowPlusHour()
	gateway.streamSeq["card-stream-1"] = 7

	if err := gateway.closeStreamCard(t.Context(), "card-stream-1", "最终答复"); err != nil {
		t.Fatalf("expected already-closed pre-close update to be ignored, got %v", err)
	}
	if settingsCalls != 0 {
		t.Fatalf("expected already-closed pre-close update to skip settings patch, got %d settings calls", settingsCalls)
	}
	if _, ok := gateway.streamSeq["card-stream-1"]; ok {
		t.Fatalf("expected closed stream card sequence to be forgotten")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func timeNowPlusHour() time.Time {
	return time.Now().Add(time.Hour)
}
