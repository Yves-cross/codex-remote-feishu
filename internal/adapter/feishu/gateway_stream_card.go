package feishu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const streamCardTokenRefreshSkew = time.Minute
const streamLoadingDotsGIFBase64 = "R0lGODlhGAAIAPcfMQAAACQAAEgAAGwAAJAAALQAANgAAPwAAAAkACQkAEgkAGwkAJAkALQkANgkAPwkAABIACRIAEhIAGxIAJBIALRIANhIAPxIAABsACRsAEhsAGxsAJBsALRsANhsAPxsAACQACSQAEiQAGyQAJCQALSQANiQAPyQAAC0ACS0AEi0AGy0AJC0ALS0ANi0APy0AADYACTYAEjYAGzYAJDYALTYANjYAPzYAAD8ACT8AEj8AGz8AJD8ALT8ANj8APz8AAAAVSQAVUgAVWwAVZAAVbQAVdgAVfwAVQAkVSQkVUgkVWwkVZAkVbQkVdgkVfwkVQBIVSRIVUhIVWxIVZBIVbRIVdhIVfxIVQBsVSRsVUhsVWxsVZBsVbRsVdhsVfxsVQCQVSSQVUiQVWyQVZCQVbSQVdiQVfyQVQC0VSS0VUi0VWy0VZC0VbS0Vdi0Vfy0VQDYVSTYVUjYVWzYVZDYVbTYVdjYVfzYVQD8VST8VUj8VWz8VZD8VbT8Vdj8Vfz8VQAAqiQAqkgAqmwAqpAAqrQAqtgAqvwAqgAkqiQkqkgkqmwkqpAkqrQkqtgkqvwkqgBIqiRIqkhIqmxIqpBIqrRIqthIqvxIqgBsqiRsqkhsqmxsqpBsqrRsqthsqvxsqgCQqiSQqkiQqmyQqpCQqrSQqtiQqvyQqgC0qiS0qki0qmy0qpC0qrS0qti0qvy0qgDYqiTYqkjYqmzYqpDYqrTYqtjYqvzYqgD8qiT8qkj8qmz8qpD8qrT8qtj8qvz8qgAA/yQA/0gA/2wA/5AA/7QA/9gA//wA/wAk/yQk/0gk/2wk/5Ak/7Qk/9gk//wk/wBI/yRI/0hI/2xI/5BI/7RI/9hI//xI/wBs/yRs/0hs/2xs/5Bs/7Rs/9hs//xs/wCQ/ySQ/0iQ/2yQ/5CQ/7SQ/9iQ//yQ/wC0/yS0/0i0/2y0/5C0/7S0/9i0//y0/wDY/yTY/0jY/2zY/5DY/7TY/9jY//zY/wD8/yT8/0j8/2z8/5D8/7T8/9j8//z8/yH/C05FVFNDQVBFMi4wAwEAAAAh+QQEEQAfACwAAAAAGAAIAAAIRQB//RtI8J/AggMPIlzIsCHCTNm0ZSvYql1FihbbFYwYsSCrWuxaeQQpkqBEiBhbsUu5kiBHbQVDsio5UCZNhzhz6kQYEAAh+QQFEAAAACwCAAIADAAEAAAIJQBbtRMIoCCATNm0ZWNVi10rgwASJhTYih1EhQgdsnpoUKK2gAAAIfkEBREAAAAsCgACAAwABAAACCUAW7UTCKAggEzZtGVjVYtdK4MAEiYU2IodRIUIHbJ6aFCitoAAADs="

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
	cardJSON, err := json.Marshal(streamingCardDocument(operation.CardTitle, operation.CardBody, operation.CardThemeKey, loadingImageKey, operation.StreamLoading))
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
	g.streamLoadingShown[cardID] = operation.StreamLoading
	g.mu.Unlock()
	return cardID, nil
}

func (g *LiveGateway) updateStreamCard(ctx context.Context, cardID, text string, loading bool) error {
	parsed, err := g.updateStreamCardResponse(ctx, cardID, text)
	if err != nil {
		return err
	}
	if parsed.Code != 0 {
		if isFeishuStreamAlreadyClosed(parsed) {
			g.forgetStreamCard(cardID)
			return nil
		}
		return fmt.Errorf("feishu stream card update failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
	}
	if err := g.syncStreamCardLoadingElement(ctx, cardID, loading); err != nil {
		return err
	}
	return nil
}

func (g *LiveGateway) updateStreamCardResponse(ctx context.Context, cardID, text string) (feishuGenericResponse, error) {
	token, err := g.tenantToken(ctx)
	if err != nil {
		return feishuGenericResponse{}, err
	}
	sequence := g.nextStreamCardSequence(cardID)
	payload, err := json.Marshal(map[string]any{
		"content":  streamCardContent(text),
		"sequence": sequence,
		"uuid":     fmt.Sprintf("content_%s_%d", strings.TrimSpace(cardID), sequence),
	})
	if err != nil {
		return feishuGenericResponse{}, err
	}
	var parsed feishuGenericResponse
	url := fmt.Sprintf("%s/cardkit/v1/cards/%s/elements/content/content", g.feishuOpenAPIBase(), strings.TrimSpace(cardID))
	if err := g.doStreamCardJSON(ctx, "cardkit.v1.card.elements.content.update", http.MethodPut, url, token, payload, &parsed); err != nil {
		return feishuGenericResponse{}, err
	}
	return parsed, nil
}

func (g *LiveGateway) closeStreamCard(ctx context.Context, cardID, text string) error {
	parsed, err := g.updateStreamCardResponse(ctx, cardID, text)
	if err != nil {
		return err
	}
	if parsed.Code != 0 {
		if isFeishuStreamAlreadyClosed(parsed) {
			g.forgetStreamCard(cardID)
			return nil
		}
		return fmt.Errorf("feishu stream card update failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
	}
	if err := g.syncStreamCardLoadingElement(ctx, cardID, false); err != nil {
		return err
	}
	token, err := g.tenantToken(ctx)
	if err != nil {
		return err
	}
	sequence := g.nextStreamCardSequence(cardID)
	payload, err := json.Marshal(map[string]any{
		"settings": JSONString(map[string]any{"config": map[string]any{
			"streaming_mode": false,
			"summary": map[string]string{
				"content": truncateStreamSummary(text, 50),
			},
		}}),
		"sequence": sequence,
		"uuid":     fmt.Sprintf("close_%s_%d", strings.TrimSpace(cardID), sequence),
	})
	if err != nil {
		return err
	}
	var closeResp feishuGenericResponse
	url := fmt.Sprintf("%s/cardkit/v1/cards/%s/settings", g.feishuOpenAPIBase(), strings.TrimSpace(cardID))
	if err := g.doStreamCardJSON(ctx, "cardkit.v1.card.settings.patch", http.MethodPatch, url, token, payload, &closeResp); err != nil {
		return err
	}
	if closeResp.Code != 0 {
		if isFeishuStreamAlreadyClosed(closeResp) {
			g.forgetStreamCard(cardID)
			return nil
		}
		return fmt.Errorf("feishu stream card close failed: code=%d msg=%s", closeResp.Code, strings.TrimSpace(closeResp.Msg))
	}
	g.forgetStreamCard(cardID)
	return nil
}

func isFeishuStreamAlreadyClosed(resp feishuGenericResponse) bool {
	return resp.Code == 300309 && strings.Contains(strings.ToLower(resp.Msg), "streaming mode is closed")
}

func (g *LiveGateway) forgetStreamCard(cardID string) {
	g.mu.Lock()
	delete(g.streamSeq, strings.TrimSpace(cardID))
	delete(g.streamLoadingShown, strings.TrimSpace(cardID))
	g.mu.Unlock()
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

func streamingCardDocument(title, body, theme, loadingImageKey string, showLoading bool) map[string]any {
	title = strings.TrimSpace(title)
	elements := []map[string]any{{
		"tag":        "markdown",
		"content":    strings.TrimSpace(body),
		"element_id": "content",
	}}
	elements = append(elements, streamCardLoadingElement(loadingImageKey, showLoading))
	doc := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"streaming_mode": true,
			"summary": map[string]string{
				"content": "[Generating...]",
			},
			"streaming_config": map[string]any{
				"print_frequency_ms": map[string]int{"default": 50},
				"print_step":         map[string]int{"default": 1},
				"print_strategy":     "fast",
			},
		},
		"body": map[string]any{
			"elements": elements,
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

func streamCardLoadingElement(imageKey string, show bool) map[string]any {
	if !show {
		return map[string]any{
			"tag":        "markdown",
			"content":    "",
			"element_id": "loading",
		}
	}
	if strings.TrimSpace(imageKey) == "" {
		return map[string]any{
			"tag":        "markdown",
			"content":    "<text_tag color='neutral'>...</text_tag>",
			"element_id": "loading",
		}
	}
	return map[string]any{
		"tag":                "column_set",
		"element_id":         "loading",
		"horizontal_spacing": "small",
		"columns": []map[string]any{{
			"tag":            "column",
			"width":          "auto",
			"vertical_align": "top",
			"elements": []map[string]any{{
				"tag":           "img",
				"img_key":       strings.TrimSpace(imageKey),
				"custom_width":  10,
				"compact_width": true,
				"preview":       false,
				"alt": map[string]any{
					"tag":     "plain_text",
					"content": "loading",
				},
			}},
		}},
	}
}

func (g *LiveGateway) syncStreamCardLoadingElement(ctx context.Context, cardID string, loading bool) error {
	cardID = strings.TrimSpace(cardID)
	g.mu.Lock()
	current, ok := g.streamLoadingShown[cardID]
	g.mu.Unlock()
	if ok && current == loading {
		return nil
	}
	imageKey := g.streamLoadingImageKeyOrEmpty(ctx)
	if err := g.updateStreamCardElement(ctx, cardID, "loading", streamCardLoadingElement(imageKey, loading), "loading"); err != nil {
		return err
	}
	g.mu.Lock()
	g.streamLoadingShown[cardID] = loading
	g.mu.Unlock()
	return nil
}

func (g *LiveGateway) updateStreamCardElement(ctx context.Context, cardID, elementID string, element map[string]any, prefix string) error {
	token, err := g.tenantToken(ctx)
	if err != nil {
		return err
	}
	sequence := g.nextStreamCardSequence(cardID)
	elementBody, err := json.Marshal(element)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"element":  string(elementBody),
		"sequence": sequence,
		"uuid":     fmt.Sprintf("%s_%s_%d", prefix, strings.TrimSpace(cardID), sequence),
	})
	if err != nil {
		return err
	}
	var parsed feishuGenericResponse
	url := fmt.Sprintf("%s/cardkit/v1/cards/%s/elements/%s", g.feishuOpenAPIBase(), strings.TrimSpace(cardID), strings.TrimSpace(elementID))
	if err := g.doStreamCardJSON(ctx, "cardkit.v1.card.elements.update", http.MethodPut, url, token, payload, &parsed); err != nil {
		return err
	}
	if parsed.Code != 0 {
		if isFeishuStreamAlreadyClosed(parsed) {
			g.forgetStreamCard(cardID)
			return nil
		}
		return fmt.Errorf("feishu stream card element update failed: code=%d msg=%s", parsed.Code, strings.TrimSpace(parsed.Msg))
	}
	return nil
}

func (g *LiveGateway) streamLoadingImageKeyOrEmpty(ctx context.Context) string {
	g.tokenMu.Lock()
	if strings.TrimSpace(g.streamLoadingImageKey) != "" {
		key := g.streamLoadingImageKey
		g.tokenMu.Unlock()
		return key
	}
	if g.streamLoadingUploadFailed {
		g.tokenMu.Unlock()
		return ""
	}
	g.tokenMu.Unlock()

	data, err := base64.StdEncoding.DecodeString(streamLoadingDotsGIFBase64)
	if err != nil {
		log.Printf("feishu stream loading gif decode failed: %v", err)
		g.tokenMu.Lock()
		g.streamLoadingUploadFailed = true
		g.tokenMu.Unlock()
		return ""
	}
	imageKey, err := g.uploadImageBytesFn(ctx, data)
	if err != nil {
		log.Printf("feishu stream loading gif upload failed: %v", err)
		g.tokenMu.Lock()
		g.streamLoadingUploadFailed = true
		g.tokenMu.Unlock()
		return ""
	}
	imageKey = strings.TrimSpace(imageKey)
	g.tokenMu.Lock()
	g.streamLoadingImageKey = imageKey
	g.streamLoadingUploadFailed = false
	g.tokenMu.Unlock()
	return imageKey
}

func streamCardContent(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return " "
	}
	return text
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
