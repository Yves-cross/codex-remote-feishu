package preview

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/kxn/codex-remote-feishu/internal/branding"
)

//go:embed turn_diff_preview_page.tmpl
var turnDiffPreviewPageTemplateText string

var turnDiffPreviewPageTemplate = template.Must(template.New("turn_diff_preview_page").Parse(turnDiffPreviewPageTemplateText))

type turnDiffPreviewPageTemplateData struct {
	LogoJSON  template.JS
	FilesJSON template.JS
}

func serveTurnDiffPreviewPageHTTP(w http.ResponseWriter, artifact *webPreviewArtifact) error {
	decoded, err := decodeTurnDiffPreviewArtifact(artifact)
	if err != nil {
		return err
	}
	if decoded == nil || len(decoded.Files) == 0 {
		return fmt.Errorf("turn diff preview artifact has no files")
	}
	html, err := renderTurnDiffPreviewPageHTML(decoded)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; form-action 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
	return nil
}

func renderTurnDiffPreviewPageHTML(artifact *turnDiffPreviewArtifact) (string, error) {
	if artifact == nil {
		return "", fmt.Errorf("turn diff preview artifact is nil")
	}
	filesJSON, err := json.Marshal(artifact.Files)
	if err != nil {
		return "", fmt.Errorf("marshal turn diff files: %w", err)
	}
	logoJSON, err := json.Marshal(branding.LogoSVGDataURI())
	if err != nil {
		return "", fmt.Errorf("marshal turn diff preview logo: %w", err)
	}
	data := turnDiffPreviewPageTemplateData{
		LogoJSON:  template.JS(logoJSON),
		FilesJSON: template.JS(filesJSON),
	}
	var out bytes.Buffer
	if err := turnDiffPreviewPageTemplate.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute turn diff preview template: %w", err)
	}
	return out.String(), nil
}

func serveTurnDiffPreviewDownloadHTTP(w http.ResponseWriter, artifact *webPreviewArtifact) error {
	decoded, err := decodeTurnDiffPreviewArtifact(artifact)
	if err != nil {
		return err
	}
	body := ""
	if decoded != nil {
		body = decoded.RawUnifiedDiff
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("turn diff preview artifact missing raw unified diff")
	}
	name := strings.TrimSpace(artifact.Record.DisplayName)
	if name == "" {
		name = "turn-diff.patch"
	} else if !strings.Contains(name, ".") {
		name += ".patch"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizePreviewDownloadName(name)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
	return nil
}

func decodeTurnDiffPreviewArtifact(artifact *webPreviewArtifact) (*turnDiffPreviewArtifact, error) {
	if artifact == nil {
		return nil, fmt.Errorf("turn diff preview artifact is nil")
	}
	var decoded turnDiffPreviewArtifact
	if err := json.Unmarshal(artifact.Content, &decoded); err != nil {
		return nil, fmt.Errorf("decode turn diff preview artifact: %w", err)
	}
	if decoded.SchemaVersion == 0 {
		decoded.SchemaVersion = turnDiffPreviewSchemaV1
	}
	return &decoded, nil
}
