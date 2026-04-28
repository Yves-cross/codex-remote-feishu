package feishu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestStreamingCardDocumentUsesTinyGIFLoadingElementWhenImageKeyPresent(t *testing.T) {
	doc := streamingCardDocument("", "正文", cardThemeProgress, "img-loading-1", true)
	body, _ := doc["body"].(map[string]any)
	elements, _ := body["elements"].([]map[string]any)
	if len(elements) != 2 {
		t.Fatalf("unexpected element count: %#v", elements)
	}
	if elements[1]["tag"] != "img" || elements[1]["element_id"] != "loading" {
		t.Fatalf("expected gif loading image element, got %#v", elements[1])
	}
	if elements[1]["img_key"] != "img-loading-1" || elements[1]["mode"] != "tiny" || elements[1]["transparent"] != true {
		t.Fatalf("expected tiny mode loading image, got %#v", elements[1])
	}
	if _, ok := elements[1]["custom_width"]; ok {
		t.Fatalf("expected loading image to avoid custom width fallback, got %#v", elements[1])
	}
	if _, ok := elements[1]["scale_type"]; ok {
		t.Fatalf("expected loading image to avoid crop scale type, got %#v", elements[1])
	}
	if _, ok := elements[1]["size"]; ok {
		t.Fatalf("expected loading image to avoid crop-only size field, got %#v", elements[1])
	}
}

func TestStreamLoadingDotsGIFIsSquareTinyThreeDotAnimation(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(streamLoadingDotsGIFBase64)
	if err != nil {
		t.Fatalf("decode loading gif base64: %v", err)
	}
	decoded, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode loading gif: %v", err)
	}
	if decoded.Config.Width != 48 || decoded.Config.Height != 48 {
		t.Fatalf("expected 48x48 high-density tiny icon gif, got %dx%d", decoded.Config.Width, decoded.Config.Height)
	}
	if len(decoded.Image) != 3 {
		t.Fatalf("expected three animation frames, got %d", len(decoded.Image))
	}
	expectedDelays := []int{17, 16, 17}
	for i, delay := range expectedDelays {
		if decoded.Delay[i] != delay {
			t.Fatalf("frame %d expected original loading gif delay %d, got %d", i, delay, decoded.Delay[i])
		}
	}
	centers := []struct {
		x int
		y int
	}{
		{x: 10, y: 24},
		{x: 24, y: 24},
		{x: 38, y: 24},
	}
	for frameIndex, frame := range decoded.Image {
		for _, center := range centers {
			_, _, _, alpha := frame.At(center.x, center.y).RGBA()
			if alpha == 0 {
				t.Fatalf("frame %d expected visible dot at %d,%d", frameIndex, center.x, center.y)
			}
		}
		red, _, blue, alpha := frame.At(centers[frameIndex].x, centers[frameIndex].y).RGBA()
		if alpha == 0 || uint8(red>>8) > 80 || uint8(blue>>8) < 200 {
			t.Fatalf("frame %d expected active blue dot at %#v", frameIndex, centers[frameIndex])
		}
	}
}

func TestStreamLoadingImageKeyPersistsAcrossGatewayRestarts(t *testing.T) {
	tempDir := t.TempDir()
	first := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1", TempDir: tempDir})
	var uploadCalls int
	first.uploadImageBytesFn = func(context.Context, []byte) (string, error) {
		uploadCalls++
		return "img-loading-cached", nil
	}
	if key := first.streamLoadingImageKeyOrEmpty(t.Context()); key != "img-loading-cached" {
		t.Fatalf("expected uploaded image key, got %q", key)
	}
	if uploadCalls != 1 {
		t.Fatalf("expected one upload, got %d", uploadCalls)
	}
	cachePath := filepath.Join(tempDir, "stream-loading-image-key.json")
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected image key cache at %s: %v", cachePath, err)
	}

	second := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1", TempDir: tempDir})
	second.uploadImageBytesFn = func(context.Context, []byte) (string, error) {
		t.Fatal("expected cached stream loading image key to avoid upload")
		return "", nil
	}
	if key := second.streamLoadingImageKeyOrEmpty(t.Context()); key != "img-loading-cached" {
		t.Fatalf("expected cached image key, got %q", key)
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

func TestUpdateStreamCardReopensAlreadyClosedStreamAndRetries(t *testing.T) {
	var updateCalls int
	var settingsCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			updateCalls++
			if updateCalls == 1 {
				writeJSON(t, w, map[string]any{"code": 300309, "msg": "ErrMsg: streaming mode is closed;"})
				return
			}
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			settingsCalls++
			assertStreamCardSettingsMode(t, r, true)
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
	gateway.streamLoadingShown["card-stream-1"] = true

	if err := gateway.updateStreamCard(t.Context(), "card-stream-1", "stale update", true); err != nil {
		t.Fatalf("expected already-closed update to reopen and retry, got %v", err)
	}
	if updateCalls != 2 {
		t.Fatalf("expected original update and retry, got %d", updateCalls)
	}
	if settingsCalls != 1 {
		t.Fatalf("expected one reopen settings request, got %d", settingsCalls)
	}
	if seq := gateway.streamSeq["card-stream-1"]; seq != 10 {
		t.Fatalf("expected sequence to advance through update/reopen/retry, got %d", seq)
	}
}

func TestUpdateStreamCardReopensStreamingTimeoutAndRetries(t *testing.T) {
	var updateCalls int
	var settingsCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			updateCalls++
			if updateCalls == 1 {
				writeJSON(t, w, map[string]any{"code": 200850, "msg": "ErrMsg: card streaming timeout;"})
				return
			}
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			settingsCalls++
			assertStreamCardSettingsMode(t, r, true)
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
	gateway.streamLoadingShown["card-stream-1"] = true

	if err := gateway.updateStreamCard(t.Context(), "card-stream-1", "late update", true); err != nil {
		t.Fatalf("expected timed-out update to reopen and retry, got %v", err)
	}
	if updateCalls != 2 {
		t.Fatalf("expected original update and retry, got %d", updateCalls)
	}
	if settingsCalls != 1 {
		t.Fatalf("expected one reopen settings request, got %d", settingsCalls)
	}
	if seq := gateway.streamSeq["card-stream-1"]; seq != 10 {
		t.Fatalf("expected sequence to advance through update/reopen/retry, got %d", seq)
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

func TestCloseStreamCardReopensPreCloseAlreadyClosedAndRetries(t *testing.T) {
	var contentCalls int
	var settingsCalls int
	var loadingCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			contentCalls++
			if contentCalls == 1 {
				writeJSON(t, w, map[string]any{"code": 300309, "msg": "ErrMsg: streaming mode is closed;"})
				return
			}
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/loading":
			loadingCalls++
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			settingsCalls++
			assertStreamCardSettingsMode(t, r, settingsCalls == 1)
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
	gateway.streamLoadingShown["card-stream-1"] = true

	if err := gateway.closeStreamCard(t.Context(), "card-stream-1", "最终答复"); err != nil {
		t.Fatalf("expected already-closed pre-close update to reopen and retry, got %v", err)
	}
	if contentCalls != 2 {
		t.Fatalf("expected original content update and retry, got %d", contentCalls)
	}
	if loadingCalls != 1 {
		t.Fatalf("expected loading element hide after retry, got %d", loadingCalls)
	}
	if settingsCalls != 2 {
		t.Fatalf("expected reopen and close settings patches, got %d", settingsCalls)
	}
	if _, ok := gateway.streamSeq["card-stream-1"]; ok {
		t.Fatalf("expected stream card sequence to be forgotten after close")
	}
}

func TestCloseStreamCardReopensPreCloseStreamingTimeoutAndRetries(t *testing.T) {
	var contentCalls int
	var settingsCalls int
	var loadingCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			contentCalls++
			if contentCalls == 1 {
				writeJSON(t, w, map[string]any{"code": 200850, "msg": "ErrMsg: card streaming timeout;"})
				return
			}
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/loading":
			loadingCalls++
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			settingsCalls++
			assertStreamCardSettingsMode(t, r, settingsCalls == 1)
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
	gateway.streamLoadingShown["card-stream-1"] = true

	if err := gateway.closeStreamCard(t.Context(), "card-stream-1", "最终答复"); err != nil {
		t.Fatalf("expected timed-out pre-close update to reopen and retry, got %v", err)
	}
	if contentCalls != 2 {
		t.Fatalf("expected original content update and retry, got %d", contentCalls)
	}
	if loadingCalls != 1 {
		t.Fatalf("expected loading element hide after retry, got %d", loadingCalls)
	}
	if settingsCalls != 2 {
		t.Fatalf("expected reopen and close settings patches, got %d", settingsCalls)
	}
	if _, ok := gateway.streamSeq["card-stream-1"]; ok {
		t.Fatalf("expected timed-out stream card sequence to be forgotten")
	}
	if _, ok := gateway.streamLoadingShown["card-stream-1"]; ok {
		t.Fatalf("expected timed-out stream card loading state to be forgotten")
	}
}

func TestUpdateStreamCardElementReopensStreamingTimeoutAndRetries(t *testing.T) {
	var elementCalls int
	var settingsCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/loading":
			elementCalls++
			if elementCalls == 1 {
				writeJSON(t, w, map[string]any{"code": 200850, "msg": "ErrMsg: card streaming timeout;"})
				return
			}
			writeJSON(t, w, map[string]any{"code": 0, "msg": "ok"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			settingsCalls++
			assertStreamCardSettingsMode(t, r, true)
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
	gateway.streamLoadingShown["card-stream-1"] = true

	err := gateway.updateStreamCardElement(
		t.Context(),
		"card-stream-1",
		"loading",
		streamCardLoadingElement("", false),
		"loading",
	)
	if err != nil {
		t.Fatalf("expected timed-out element update to reopen and retry, got %v", err)
	}
	if elementCalls != 2 {
		t.Fatalf("expected original loading element request and retry, got %d", elementCalls)
	}
	if settingsCalls != 1 {
		t.Fatalf("expected one reopen settings request, got %d", settingsCalls)
	}
	if seq := gateway.streamSeq["card-stream-1"]; seq != 10 {
		t.Fatalf("expected sequence to advance through element/reopen/retry, got %d", seq)
	}
}

func TestUpdateStreamCardForgetsStreamWhenReopenRetryStillTimesOut(t *testing.T) {
	var updateCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/elements/content/content":
			updateCalls++
			writeJSON(t, w, map[string]any{"code": 200850, "msg": "ErrMsg: card streaming timeout;"})
		case r.URL.Path == "/open-apis/cardkit/v1/cards/card-stream-1/settings":
			assertStreamCardSettingsMode(t, r, true)
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
	gateway.streamLoadingShown["card-stream-1"] = true

	if err := gateway.updateStreamCard(t.Context(), "card-stream-1", "late update", true); err != nil {
		t.Fatalf("expected repeated terminal update to be ignored after one retry, got %v", err)
	}
	if updateCalls != 2 {
		t.Fatalf("expected exactly one retry after reopen, got %d update calls", updateCalls)
	}
	if _, ok := gateway.streamSeq["card-stream-1"]; ok {
		t.Fatalf("expected repeatedly timed-out stream card sequence to be forgotten")
	}
	if _, ok := gateway.streamLoadingShown["card-stream-1"]; ok {
		t.Fatalf("expected repeatedly timed-out stream card loading state to be forgotten")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func assertStreamCardSettingsMode(t *testing.T, r *http.Request, want bool) {
	t.Helper()
	var payload struct {
		Settings string `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("decode settings payload: %v", err)
	}
	var settings struct {
		Config struct {
			StreamingMode bool `json:"streaming_mode"`
		} `json:"config"`
	}
	if err := json.Unmarshal([]byte(payload.Settings), &settings); err != nil {
		t.Fatalf("decode settings string: %v", err)
	}
	if settings.Config.StreamingMode != want {
		t.Fatalf("expected streaming_mode=%v, got %v in %s", want, settings.Config.StreamingMode, payload.Settings)
	}
}

func timeNowPlusHour() time.Time {
	return time.Now().Add(time.Hour)
}
