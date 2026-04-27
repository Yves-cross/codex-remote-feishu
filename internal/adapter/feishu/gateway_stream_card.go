package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const streamCardTokenRefreshSkew = time.Minute

const streamLoadingDotsWhiteGIFBase64 = "R0lGODlhGAAIAPcfMQAAACQAAEgAAGwAAJAAALQAANgAAPwAAAAkACQkAEgkAGwkAJAkALQkANgkAPwkAABIACRIAEhIAGxIAJBIALRIANhIAPxIAABsACRsAEhsAGxsAJBsALRsANhsAPxsAACQACSQAEiQAGyQAJCQALSQANiQAPyQAAC0ACS0AEi0AGy0AJC0ALS0ANi0APy0AADYACTYAEjYAGzYAJDYALTYANjYAPzYAAD8ACT8AEj8AGz8AJD8ALT8ANj8APz8AAAAVSQAVUgAVWwAVZAAVbQAVdgAVfwAVQAkVSQkVUgkVWwkVZAkVbQkVdgkVfwkVQBIVSRIVUhIVWxIVZBIVbRIVdhIVfxIVQBsVSRsVUhsVWxsVZBsVbRsVdhsVfxsVQCQVSSQVUiQVWyQVZCQVbSQVdiQVfyQVQC0VSS0VUi0VWy0VZC0VbS0Vdi0Vfy0VQDYVSTYVUjYVWzYVZDYVbTYVdjYVfzYVQD8VST8VUj8VWz8VZD8VbT8Vdj8Vfz8VQAAqiQAqkgAqmwAqpAAqrQAqtgAqvwAqgAkqiQkqkgkqmwkqpAkqrQkqtgkqvwkqgBIqiRIqkhIqmxIqpBIqrRIqthIqvxIqgBsqiRsqkhsqmxsqpBsqrRsqthsqvxsqgCQqiSQqkiQqmyQqpCQqrSQqtiQqvyQqgC0qiS0qki0qmy0qpC0qrS0qti0qvy0qgDYqiTYqkjYqmzYqpDYqrTYqtjYqvzYqgD8qiT8qkj8qmz8qpD8qrT8qtj8qvz8qgAA/yQA/0gA/2wA/5AA/7QA/9gA//wA/wAk/yQk/0gk/2wk/5Ak/7Qk/9gk//wk/wBI/yRI/0hI/2xI/5BI/7RI/9hI//xI/wBs/yRs/0hs/2xs/5Bs/7Rs/9hs//xs/wCQ/ySQ/0iQ/2yQ/5CQ/7SQ/9iQ//yQ/wC0/yS0/0i0/2y0/5C0/7S0/9i0//y0/wDY/yTY/0jY/2zY/5DY/7TY/9jY//zY/wD8/yT8/0j8/2z8/5D8/7T8/9j8//z8/yH/C05FVFNDQVBFMi4wAwEAAAAh+QQEGQAfACwAAAAAGAAIAAAIIgD/CRxIsKDBgwgTFtzCUKHDLZu2OFTIcNPEixgzatxIMCAAIfkEBRkAAAAsBAACAAkAAwAACBQAAQgcuKXgQIJktpA6CGDLpi0BAQAh+QQFGQAAACwLAAIACgADAAAIFQDJABhIkMyWg5sIDjx4UOHCTVsCAgA7"

type feishuTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int64  `json:"expire"`
}

type feishuCardCreateResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		CardID string `json:"card_id"`
	} `json:"data"`
}

type feishuGenericResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (g *LiveGateway) feishuOpenAPIBase() string {
	domain := strings.TrimRight(strings.TrimSpace(g.config.Domain), "/")
	if domain == "" || domain == "feishu" {
		return "https://open.feishu.cn/open-apis"
	}
	if domain == "lark" {
		return "https://open.larksuite.com/open-apis"
	}
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return domain + "/open-apis"
	}
	return "https://open.feishu.cn/open-apis"
}

func (g *LiveGateway) tenantToken(ctx context.Context) (string, error) {
	g.tokenMu.Lock()
	token := strings.TrimSpace(g.tenantAccessToken)
	expiresAt := g.tenantTokenExpiresAt
	g.tokenMu.Unlock()
	if token != "" && time.Now().Before(expiresAt.Add(-streamCardTokenRefreshSkew)) {
		return token, nil
	}
	if strings.TrimSpace(g.config.AppID) == "" || strings.TrimSpace(g.config.AppSecret) == "" {
		return "", fmt.Errorf("feishu tenant token failed: missing app credentials")
	}
	body, err := json.Marshal(map[string]string{
		"app_id":     strings.TrimSpace(g.config.AppID),
		"app_secret": strings.TrimSpace(g.config.AppSecret),
	})
	if err != nil {
		return "", err
	}
	var parsed feishuTokenResponse
	if err := g.doStreamCardJSON(ctx, "auth.v3.tenant_access_token.internal", http.MethodPost, g.feishuOpenAPIBase()+"/auth/v3/tenant_access_token/internal", "", body, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != 0 || strings.TrimSpace(parsed.TenantAccessToken) == "" {
		return "", fmt.Errorf("feishu tenant token failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
	}
	expiresIn := time.Duration(parsed.Expire) * time.Second
	if expiresIn <= 0 {
		expiresIn = 2 * time.Hour
	}
	g.tokenMu.Lock()
	g.tenantAccessToken = strings.TrimSpace(parsed.TenantAccessToken)
	g.tenantTokenExpiresAt = time.Now().Add(expiresIn)
	g.tokenMu.Unlock()
	return strings.TrimSpace(parsed.TenantAccessToken), nil
}

func (g *LiveGateway) createStreamCard(ctx context.Context, operation Operation) (string, error) {
	token, err := g.tenantToken(ctx)
	if err != nil {
		return "", err
	}
	loadingImageKey := g.streamLoadingImageKeyOrEmpty(ctx)
	cardJSON, err := json.Marshal(streamingCardDocument(operation.CardTitle, operation.CardBody, operation.StreamLoadingText, loadingImageKey, operation.CardThemeKey))
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{
		"type": "card_json",
		"data": string(cardJSON),
	})
	if err != nil {
		return "", err
	}
	var parsed feishuCardCreateResponse
	if err := g.doStreamCardJSON(ctx, "cardkit.v1.cards.create", http.MethodPost, g.feishuOpenAPIBase()+"/cardkit/v1/cards", token, payload, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != 0 || strings.TrimSpace(parsed.Data.CardID) == "" {
		return "", fmt.Errorf("feishu stream card create failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
	}
	cardID := strings.TrimSpace(parsed.Data.CardID)
	g.mu.Lock()
	g.streamSeq[cardID] = 1
	g.streamText[cardID] = strings.TrimSpace(operation.CardBody)
	g.streamLoadingText[cardID] = strings.TrimSpace(operation.StreamLoadingText)
	g.mu.Unlock()
	return cardID, nil
}

func (g *LiveGateway) updateStreamCard(ctx context.Context, cardID, text, loadingText string) error {
	token, err := g.tenantToken(ctx)
	if err != nil {
		return err
	}
	text = strings.TrimSpace(text)
	loadingText = strings.TrimSpace(loadingText)
	lastText, lastLoadingText := g.streamCardState(cardID)
	if text != lastText {
		if err := g.putStreamCardElementContentWithReopen(ctx, token, cardID, "content", text); err != nil {
			return err
		}
		g.setStreamCardText(cardID, text)
	}
	if loadingText != lastLoadingText {
		loadingImageKey := g.streamLoadingImageKeyOrEmpty(ctx)
		if err := g.putStreamCardElementWithReopen(ctx, token, cardID, "loading", streamCardLoadingElement(loadingText, loadingImageKey)); err != nil {
			return err
		}
		g.setStreamCardLoadingText(cardID, loadingText)
	}
	return nil
}

func (g *LiveGateway) closeStreamCard(ctx context.Context, cardID, text string) error {
	if err := g.updateStreamCard(ctx, cardID, text, ""); err != nil {
		return err
	}
	token, err := g.tenantToken(ctx)
	if err != nil {
		return err
	}
	sequence := g.nextStreamCardSequence(cardID)
	if err := g.patchStreamCardSettings(ctx, token, cardID, streamCardSettings(false, truncateStreamSummary(text, 50)), sequence, fmt.Sprintf("close_%s_%d", strings.TrimSpace(cardID), sequence)); err != nil {
		return err
	}
	g.mu.Lock()
	delete(g.streamSeq, strings.TrimSpace(cardID))
	delete(g.streamText, strings.TrimSpace(cardID))
	delete(g.streamLoadingText, strings.TrimSpace(cardID))
	g.mu.Unlock()
	return nil
}

func (g *LiveGateway) nextStreamCardSequence(cardID string) int {
	cardID = strings.TrimSpace(cardID)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.streamSeq == nil {
		g.streamSeq = map[string]int{}
	}
	next := g.streamSeq[cardID] + 1
	if next <= 1 {
		next = 2
	}
	g.streamSeq[cardID] = next
	return next
}

func (g *LiveGateway) doStreamCardJSON(ctx context.Context, api, method, url, token string, body []byte, out any) error {
	_, err := DoHTTP(ctx, g.broker, CallSpec{
		GatewayID: g.config.GatewayID,
		API:       api,
		Class:     CallClassIMPatch,
		Priority:  CallPriorityInteractive,
		Retry:     RetryRateLimitOnly,
	}, func(callCtx context.Context, client *http.Client) (struct{}, error) {
		req, err := http.NewRequestWithContext(callCtx, method, url, bytes.NewReader(body))
		if err != nil {
			return struct{}{}, err
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return struct{}{}, err
		}
		defer resp.Body.Close()
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return struct{}{}, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return struct{}{}, fmt.Errorf("%s failed: http=%d body=%s", api, resp.StatusCode, strings.TrimSpace(string(responseBody)))
		}
		if out != nil && len(responseBody) != 0 {
			if err := json.Unmarshal(responseBody, out); err != nil {
				return struct{}{}, err
			}
		}
		return struct{}{}, nil
	})
	return err
}

func (g *LiveGateway) putStreamCardContent(ctx context.Context, token, cardID, elementID, text string, sequence int) (feishuGenericResponse, error) {
	payload, err := json.Marshal(map[string]any{
		"content":  streamCardContent(text),
		"sequence": sequence,
		"uuid":     fmt.Sprintf("%s_%s_%d", strings.TrimSpace(elementID), strings.TrimSpace(cardID), sequence),
	})
	if err != nil {
		return feishuGenericResponse{}, err
	}
	var parsed feishuGenericResponse
	url := fmt.Sprintf("%s/cardkit/v1/cards/%s/elements/%s/content", g.feishuOpenAPIBase(), strings.TrimSpace(cardID), strings.TrimSpace(elementID))
	if err := g.doStreamCardJSON(ctx, "cardkit.v1.card.elements.content.update", http.MethodPut, url, token, payload, &parsed); err != nil {
		return feishuGenericResponse{}, err
	}
	return parsed, nil
}

func (g *LiveGateway) putStreamCardElementContentWithReopen(ctx context.Context, token, cardID, elementID, text string) error {
	sequence := g.nextStreamCardSequence(cardID)
	parsed, err := g.putStreamCardContent(ctx, token, cardID, elementID, text, sequence)
	if err != nil {
		return err
	}
	if parsed.Code == 0 {
		return nil
	}
	if shouldReopenStreamCard(parsed.Code) {
		reopenSequence := g.nextStreamCardSequence(cardID)
		if err := g.patchStreamCardSettings(ctx, token, cardID, streamCardSettings(true, "[Generating...]"), reopenSequence, fmt.Sprintf("reopen_%s_%d", strings.TrimSpace(cardID), reopenSequence)); err != nil {
			return err
		}
		retrySequence := g.nextStreamCardSequence(cardID)
		parsed, err = g.putStreamCardContent(ctx, token, cardID, elementID, text, retrySequence)
		if err != nil {
			return err
		}
		if parsed.Code == 0 {
			return nil
		}
	}
	return fmt.Errorf("feishu stream card update failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
}

func (g *LiveGateway) putStreamCardElement(ctx context.Context, token, cardID, elementID string, element map[string]any, sequence int) (feishuGenericResponse, error) {
	payload, err := json.Marshal(map[string]any{
		"element":  JSONString(element),
		"sequence": sequence,
		"uuid":     fmt.Sprintf("element_%s_%s_%d", strings.TrimSpace(elementID), strings.TrimSpace(cardID), sequence),
	})
	if err != nil {
		return feishuGenericResponse{}, err
	}
	var parsed feishuGenericResponse
	url := fmt.Sprintf("%s/cardkit/v1/cards/%s/elements/%s", g.feishuOpenAPIBase(), strings.TrimSpace(cardID), strings.TrimSpace(elementID))
	if err := g.doStreamCardJSON(ctx, "cardkit.v1.card.element.update", http.MethodPut, url, token, payload, &parsed); err != nil {
		return feishuGenericResponse{}, err
	}
	return parsed, nil
}

func (g *LiveGateway) putStreamCardElementWithReopen(ctx context.Context, token, cardID, elementID string, element map[string]any) error {
	sequence := g.nextStreamCardSequence(cardID)
	parsed, err := g.putStreamCardElement(ctx, token, cardID, elementID, element, sequence)
	if err != nil {
		return err
	}
	if parsed.Code == 0 {
		return nil
	}
	if shouldReopenStreamCard(parsed.Code) {
		reopenSequence := g.nextStreamCardSequence(cardID)
		if err := g.patchStreamCardSettings(ctx, token, cardID, streamCardSettings(true, "[Generating...]"), reopenSequence, fmt.Sprintf("reopen_%s_%d", strings.TrimSpace(cardID), reopenSequence)); err != nil {
			return err
		}
		retrySequence := g.nextStreamCardSequence(cardID)
		parsed, err = g.putStreamCardElement(ctx, token, cardID, elementID, element, retrySequence)
		if err != nil {
			return err
		}
		if parsed.Code == 0 {
			return nil
		}
	}
	return fmt.Errorf("feishu stream card element update failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
}

func (g *LiveGateway) patchStreamCardSettings(ctx context.Context, token, cardID string, config map[string]any, sequence int, uuid string) error {
	payload, err := json.Marshal(map[string]any{
		"settings": JSONString(map[string]any{"config": config}),
		"sequence": sequence,
		"uuid":     uuid,
	})
	if err != nil {
		return err
	}
	var parsed feishuGenericResponse
	url := fmt.Sprintf("%s/cardkit/v1/cards/%s/settings", g.feishuOpenAPIBase(), strings.TrimSpace(cardID))
	if err := g.doStreamCardJSON(ctx, "cardkit.v1.card.settings.patch", http.MethodPatch, url, token, payload, &parsed); err != nil {
		return err
	}
	if parsed.Code != 0 {
		return fmt.Errorf("feishu stream card settings patch failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
	}
	return nil
}

func streamingCardDocument(title, body, loadingText, loadingImageKey, theme string) map[string]any {
	title = strings.TrimSpace(title)
	doc := map[string]any{
		"schema": "2.0",
		"config": streamCardSettings(true, "[Generating...]"),
		"body": map[string]any{
			"elements": []map[string]any{
				{
					"tag":        "markdown",
					"content":    strings.TrimSpace(body),
					"element_id": "content",
				},
				streamCardLoadingElement(loadingText, loadingImageKey),
			},
		},
	}
	if title != "" {
		doc["header"] = map[string]any{
			"title": map[string]string{
				"tag":     "plain_text",
				"content": title,
			},
			"template": feishuCardTemplate(theme),
		}
	}
	return doc
}

func (g *LiveGateway) streamCardState(cardID string) (string, string) {
	cardID = strings.TrimSpace(cardID)
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.streamText[cardID], g.streamLoadingText[cardID]
}

func (g *LiveGateway) setStreamCardText(cardID, text string) {
	cardID = strings.TrimSpace(cardID)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.streamText[cardID] = strings.TrimSpace(text)
}

func (g *LiveGateway) setStreamCardLoadingText(cardID, text string) {
	cardID = strings.TrimSpace(cardID)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.streamLoadingText[cardID] = strings.TrimSpace(text)
}

func streamCardSettings(streaming bool, summary string) map[string]any {
	summary = strings.TrimSpace(summary)
	config := map[string]any{
		"streaming_mode": streaming,
		"summary": map[string]string{
			"content": summary,
		},
	}
	if streaming {
		config["streaming_config"] = map[string]any{
			"print_frequency_ms": map[string]int{"default": 70},
			"print_step":         map[string]int{"default": 1},
			"print_strategy":     "fast",
		}
	}
	return config
}

func shouldReopenStreamCard(code int) bool {
	switch code {
	case 200850, 300309:
		return true
	default:
		return false
	}
}

func streamCardContent(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return " "
	}
	return text
}

func streamCardLoadingContent(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return " "
	}
	return "<text_tag color='neutral'>" + text + "</text_tag>"
}

func streamCardLoadingElement(text, imageKey string) map[string]any {
	text = strings.TrimSpace(text)
	imageKey = strings.TrimSpace(imageKey)
	element := map[string]any{
		"tag":        "div",
		"element_id": "loading",
		"width":      "auto",
		"margin":     "4px 0 0 0",
		"text": map[string]any{
			"tag":        "plain_text",
			"content":    " ",
			"text_size":  "notation",
			"text_color": "default",
		},
	}
	if imageKey != "" && text != "" {
		element["icon"] = map[string]any{
			"tag":     "custom_icon",
			"img_key": imageKey,
		}
		return element
	}
	element["text"] = map[string]any{
		"tag":        "lark_md",
		"content":    streamCardLoadingContent(text),
		"text_size":  "notation",
		"text_color": "default",
	}
	return element
}

func (g *LiveGateway) streamLoadingImageKeyOrEmpty(ctx context.Context) string {
	if g == nil {
		return ""
	}
	g.mu.Lock()
	if strings.TrimSpace(g.streamLoadingImageKey) != "" {
		key := strings.TrimSpace(g.streamLoadingImageKey)
		g.mu.Unlock()
		return key
	}
	if g.streamLoadingImageKeyFailed {
		g.mu.Unlock()
		return ""
	}
	g.mu.Unlock()

	data, err := decodeBase64Image(streamLoadingDotsWhiteGIFBase64)
	if err != nil {
		g.mu.Lock()
		g.streamLoadingImageKeyFailed = true
		g.mu.Unlock()
		return ""
	}
	key, err := g.uploadImageBytesFn(ctx, data)
	if err != nil {
		g.mu.Lock()
		g.streamLoadingImageKeyFailed = true
		g.mu.Unlock()
		return ""
	}
	g.mu.Lock()
	g.streamLoadingImageKey = strings.TrimSpace(key)
	g.mu.Unlock()
	return strings.TrimSpace(key)
}

func feishuCardTemplate(theme string) string {
	switch strings.TrimSpace(theme) {
	case cardThemeSuccess:
		return "green"
	case cardThemeError:
		return "red"
	case cardThemePlan:
		return "purple"
	case cardThemeFinal:
		return "green"
	default:
		return "blue"
	}
}

func truncateStreamSummary(text string, limit int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func JSONString(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}
