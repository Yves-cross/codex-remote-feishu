package feishu

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const streamCardTokenRefreshSkew = time.Minute
const streamLoadingDotsGIFBase64 = "R0lGODlhMAAwAIEAAAAAALC4xB1p/wAAACH/C05FVFNDQVBFMi4wAwEAAAAh+QQJEQAAACwAAAAAMAAwAAACcYSPqcvtD6OctNqLs968+w+G4kiW5omm6sq27iVEgUwJ9h0rwc7Per+D4IYJYK9oDDqGOGTSaXwwb1Bg9diYNg1JLKDLW2qvYQRYOjZ3ycps+nBWP9HEn9XujeQePv7rDxgoOEhYaHiImKi4yNjoqFAAACH5BAkQAAAALAAAAAAwADAAAAJ0hI+py+0Po5y02ouz3rz7D4biSJbmiabqyrbuFURCFE/BjdeJwPezksNBgrmdz5cgCh3Km/HISzYfTZ0B2pMqmVUdFmnoWoFTxPd3qFLLh7OWuN6av+/gEK7ALuSSMQPN9CI4SFhoeIiYqLjI2Oj4CBmpUAAAIfkECREAAAAsAAAAADAAMAAAAnOEj6nL7Q+jnLTai7PevPsPhuJIluaJpurKtu4VRDEkUMGNz0mOL8IPrD14uR3xlggqh0edoekEKIPMo9GKmAIdUOSze9BSG9ArMSmunhHlbJrbNPPQWghW7qWPJVFG3/cSKDhIWGh4iJiouMjY6PgIqVAAADs="

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

type streamLoadingImageCache struct {
	Hash      string `json:"hash"`
	ImageKey  string `json:"image_key"`
	UpdatedAt string `json:"updated_at"`
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
		if isFeishuStreamTerminal(parsed) {
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
		if isFeishuStreamTerminal(parsed) {
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
		if isFeishuStreamTerminal(closeResp) {
			g.forgetStreamCard(cardID)
			return nil
		}
		return fmt.Errorf("feishu stream card close failed: code=%d msg=%s", closeResp.Code, strings.TrimSpace(closeResp.Msg))
	}
	g.forgetStreamCard(cardID)
	return nil
}

func isFeishuStreamTerminal(resp feishuGenericResponse) bool {
	msg := strings.ToLower(strings.TrimSpace(resp.Msg))
	switch resp.Code {
	case 300309:
		return strings.Contains(msg, "streaming mode is closed")
	case 200850:
		return strings.Contains(msg, "card streaming timeout")
	default:
		return false
	}
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
	if strings.TrimSpace(imageKey) != "" {
		return map[string]any{
			"tag":         "img",
			"img_key":     strings.TrimSpace(imageKey),
			"element_id":  "loading",
			"mode":        "tiny",
			"transparent": true,
			"preview":     false,
			"alt": map[string]any{
				"tag":     "plain_text",
				"content": "loading",
			},
		}
	}
	return map[string]any{
		"tag":        "markdown",
		"content":    streamCardLoadingText(imageKey),
		"element_id": "loading",
	}
}

func streamCardLoadingText(marker string) string {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		marker = "..."
	}
	return "<text_tag color='neutral'>" + escapeFeishuTextTagContent(marker) + "</text_tag>"
}

func escapeFeishuTextTagContent(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
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
		if isFeishuStreamTerminal(parsed) {
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
	hash := streamLoadingImageHash(data)
	if imageKey := g.readCachedStreamLoadingImageKey(hash); imageKey != "" {
		g.tokenMu.Lock()
		g.streamLoadingImageKey = imageKey
		g.streamLoadingUploadFailed = false
		g.tokenMu.Unlock()
		return imageKey
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
	g.writeCachedStreamLoadingImageKey(hash, imageKey)
	return imageKey
}

func streamLoadingImageHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (g *LiveGateway) streamLoadingImageCachePath() string {
	dir := strings.TrimSpace(g.config.TempDir)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "stream-loading-image-key.json")
}

func (g *LiveGateway) readCachedStreamLoadingImageKey(hash string) string {
	path := g.streamLoadingImageCachePath()
	if path == "" || strings.TrimSpace(hash) == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cached streamLoadingImageCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return ""
	}
	if cached.Hash != hash {
		return ""
	}
	return strings.TrimSpace(cached.ImageKey)
}

func (g *LiveGateway) writeCachedStreamLoadingImageKey(hash, imageKey string) {
	path := g.streamLoadingImageCachePath()
	if path == "" || strings.TrimSpace(hash) == "" || strings.TrimSpace(imageKey) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		log.Printf("feishu stream loading gif cache mkdir failed: %v", err)
		return
	}
	payload, err := json.Marshal(streamLoadingImageCache{
		Hash:      hash,
		ImageKey:  strings.TrimSpace(imageKey),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		log.Printf("feishu stream loading gif cache marshal failed: %v", err)
		return
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		log.Printf("feishu stream loading gif cache write failed: %v", err)
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		log.Printf("feishu stream loading gif cache replace failed: %v", err)
		_ = os.Remove(tmpPath)
	}
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
