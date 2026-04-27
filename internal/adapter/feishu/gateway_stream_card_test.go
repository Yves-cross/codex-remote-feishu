package feishu

import (
	"context"
	"sync/atomic"
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
		if operation.StreamLoadingText != "." {
			t.Fatalf("unexpected stream loading text: %#v", operation)
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
		Kind:              OperationSendStreamCard,
		GatewayID:         "app-1",
		ChatID:            "oc-chat-1",
		CardBody:          "第一段",
		StreamLoadingText: ".",
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
	doc := streamingCardDocument("", "正文", ".", cardThemeProgress)
	if _, ok := doc["header"]; ok {
		t.Fatalf("expected titleless streaming card to omit header, got %#v", doc["header"])
	}
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 2 || elements[0]["content"] != "正文" || elements[0]["element_id"] != "content" || elements[1]["content"] != "." || elements[1]["element_id"] != "loading" {
		t.Fatalf("unexpected streaming card body: %#v", doc)
	}
}

func TestStreamingCardDocumentUsesBlankContentForNativeStreaming(t *testing.T) {
	doc := streamingCardDocument("", "", ".", cardThemeProgress)
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 2 || elements[0]["content"] != "" || elements[1]["content"] != "." {
		t.Fatalf("expected empty initial content for native streaming prefix matching, got %#v", doc)
	}
	config, _ := doc["config"].(map[string]any)
	streamingConfig, _ := config["streaming_config"].(map[string]any)
	if streamingConfig["print_strategy"] != "fast" {
		t.Fatalf("expected native streaming fast strategy, got %#v", streamingConfig)
	}
	printFrequency, _ := streamingConfig["print_frequency_ms"].(map[string]int)
	if printFrequency["default"] != 70 {
		t.Fatalf("expected native streaming frequency to match official default, got %#v", streamingConfig)
	}
}

func TestShouldReopenStreamCard(t *testing.T) {
	if !shouldReopenStreamCard(200850) || !shouldReopenStreamCard(300309) {
		t.Fatalf("expected timeout/closed stream card codes to trigger reopen")
	}
	if shouldReopenStreamCard(12345) {
		t.Fatalf("unexpected reopen for unrelated error code")
	}
}

func TestApplyUpdateStreamCardRequiresCardID(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	err := gateway.Apply(t.Context(), []Operation{{
		Kind:              OperationUpdateStreamCard,
		GatewayID:         "app-1",
		MessageID:         "om-stream-1",
		CardBody:          "正文",
		StreamLoadingText: ".",
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

func TestApplyUpdateStreamCardSerializesSameCard(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	var active int32
	var maxActive int32
	var calls int32
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})
	gateway.updateStreamCardFn = func(ctx context.Context, cardID, text, loadingText string) error {
		nowActive := atomic.AddInt32(&active, 1)
		defer atomic.AddInt32(&active, -1)
		for {
			currentMax := atomic.LoadInt32(&maxActive)
			if nowActive <= currentMax || atomic.CompareAndSwapInt32(&maxActive, currentMax, nowActive) {
				break
			}
		}
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			close(firstStarted)
			<-releaseFirst
		case 2:
			close(secondStarted)
		default:
			t.Fatalf("unexpected extra stream update call")
		}
		return nil
	}

	runApply := func(body string) <-chan error {
		done := make(chan error, 1)
		go func() {
			done <- gateway.Apply(context.Background(), []Operation{{
				Kind:              OperationUpdateStreamCard,
				GatewayID:         "app-1",
				MessageID:         "om-stream-1",
				StreamCardID:      "card-stream-1",
				CardBody:          body,
				StreamLoadingText: ".",
			}})
		}()
		return done
	}

	firstDone := runApply("第一段")
	secondDone := runApply("第二段")

	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for first stream update")
	}

	select {
	case <-secondStarted:
		t.Fatalf("expected second stream update to wait for same-card lock")
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first Apply returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for first Apply to finish")
	}

	select {
	case <-secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for second stream update to start")
	}

	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second Apply returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for second Apply to finish")
	}

	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("expected same-card updates to run serially, max active=%d", got)
	}
}
