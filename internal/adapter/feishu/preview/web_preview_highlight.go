package preview

import (
	"bytes"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
)

const (
	previewSyntaxStyleName               = "github"
	previewSyntaxClassPrefix             = "pv-"
	previewSyntaxHighlightThresholdBytes = 512 * 1024
)

var (
	previewSourceHighlighterFormatter = chromahtml.New(
		chromahtml.WithClasses(true),
		chromahtml.ClassPrefix(previewSyntaxClassPrefix),
	)
	previewSourceHighlighterStyle = previewSyntaxStyle()
	plainMarkdownPreviewRenderer  = goldmark.New(
		goldmark.WithRendererOptions(goldmarkhtml.WithHardWraps()),
	)
	highlightedMarkdownRenderer = goldmark.New(
		goldmark.WithRendererOptions(goldmarkhtml.WithHardWraps()),
		goldmark.WithExtensions(highlighting.NewHighlighting(
			highlighting.WithStyle(previewSyntaxStyleName),
			highlighting.WithGuessLanguage(false),
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.ClassPrefix(previewSyntaxClassPrefix),
			),
		)),
	)
	previewSyntaxCSSOnce sync.Once
	previewSyntaxCSS     string
)

var previewSourceHighlightExtensions = map[string]struct{}{
	".diff":  {},
	".go":    {},
	".htm":   {},
	".html":  {},
	".ini":   {},
	".js":    {},
	".json":  {},
	".jsx":   {},
	".patch": {},
	".py":    {},
	".sh":    {},
	".sql":   {},
	".svg":   {},
	".toml":  {},
	".ts":    {},
	".tsx":   {},
	".xml":   {},
	".yaml":  {},
	".yml":   {},
}

func previewSyntaxStyle() *chroma.Style {
	if style := styles.Get(previewSyntaxStyleName); style != nil {
		return style
	}
	return styles.Fallback
}

func previewSyntaxStylesheet() string {
	previewSyntaxCSSOnce.Do(func() {
		var buf bytes.Buffer
		if err := previewSourceHighlighterFormatter.WriteCSS(&buf, previewSourceHighlighterStyle); err == nil {
			previewSyntaxCSS = buf.String()
		}
	})
	return previewSyntaxCSS
}

func shouldHighlightMarkdownPreview(record webPreviewRecord) bool {
	return record.SizeBytes > 0 && record.SizeBytes <= previewSyntaxHighlightThresholdBytes
}

func shouldHighlightSourcePreview(record webPreviewRecord) bool {
	switch strings.TrimSpace(record.RendererKind) {
	case "text", "html_source", "svg_source":
	default:
		return false
	}
	return record.SizeBytes > 0 &&
		record.SizeBytes <= previewSyntaxHighlightThresholdBytes &&
		previewSourceLexer(record.SourcePath) != nil
}

func previewSourceLexer(sourcePath string) chroma.Lexer {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(sourcePath)))
	if _, ok := previewSourceHighlightExtensions[ext]; !ok {
		return nil
	}
	lexer := lexers.Match(filepath.Base(strings.TrimSpace(sourcePath)))
	if lexer == nil {
		return nil
	}
	return chroma.Coalesce(lexer)
}

func renderHighlightedSourcePreviewHTML(sourcePath string, content []byte) (string, error) {
	lexer := previewSourceLexer(sourcePath)
	if lexer == nil {
		return "", nil
	}
	iterator, err := lexer.Tokenise(nil, string(content))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := previewSourceHighlighterFormatter.Format(&buf, previewSourceHighlighterStyle, iterator); err != nil {
		return "", err
	}
	return `<div class="preview-syntax preview-syntax--source">` + buf.String() + `</div>`, nil
}

func renderLineAddressedHighlightedSourcePreviewHTML(sourcePath string, content []byte, location PreviewLocation) (string, error) {
	lexer := previewSourceLexer(sourcePath)
	if lexer == nil {
		return "", nil
	}
	iterator, err := lexer.Tokenise(nil, string(content))
	if err != nil {
		return "", err
	}
	lines := chroma.SplitTokensIntoLines(iterator.Tokens())
	if len(lines) == 0 {
		lines = [][]chroma.Token{{{Type: chroma.Text, Value: ""}}}
	}

	var builder strings.Builder
	builder.WriteString(`<div class="source-block source-block--numbered preview-syntax preview-syntax--source">`)
	for index, line := range lines {
		lineNumber := index + 1
		lineID := "L" + previewItoa(lineNumber)
		lineClass := "source-line"
		if location.Line == lineNumber {
			lineClass += " source-line--target"
		}
		builder.WriteString(`<div id="`)
		builder.WriteString(lineID)
		builder.WriteString(`" class="`)
		builder.WriteString(lineClass)
		builder.WriteString(`"><a class="source-line-number" href="#`)
		builder.WriteString(lineID)
		builder.WriteString(`">`)
		builder.WriteString(previewItoa(lineNumber))
		builder.WriteString(`</a><span class="source-line-text">`)
		builder.WriteString(renderPreviewHighlightedLineTokens(line, lineNumber, location))
		builder.WriteString(`</span></div>`)
	}
	builder.WriteString(`</div>`)
	return builder.String(), nil
}

func renderPreviewHighlightedLineTokens(tokens []chroma.Token, lineNumber int, location PreviewLocation) string {
	if location.Line != lineNumber || location.Column <= 0 {
		return renderPreviewHighlightedTokenHTML(tokens)
	}
	columnIndex := location.Column - 1
	if columnIndex < 0 {
		return renderPreviewHighlightedTokenHTML(tokens)
	}

	var prefix strings.Builder
	var suffix strings.Builder
	remaining := columnIndex
	highlightStarted := false
	for _, token := range tokens {
		if highlightStarted {
			suffix.WriteString(renderPreviewHighlightedToken(token))
			continue
		}
		runes := []rune(token.Value)
		if remaining >= len(runes) {
			prefix.WriteString(renderPreviewHighlightedToken(token))
			remaining -= len(runes)
			continue
		}

		if remaining > 0 {
			prefix.WriteString(renderPreviewHighlightedToken(chroma.Token{Type: token.Type, Value: string(runes[:remaining])}))
		}
		suffix.WriteString(renderPreviewHighlightedToken(chroma.Token{Type: token.Type, Value: string(runes[remaining:])}))
		highlightStarted = true
	}
	if !highlightStarted {
		return prefix.String()
	}
	return prefix.String() + `<span class="source-column-target">` + suffix.String() + `</span>`
}

func renderPreviewHighlightedTokenHTML(tokens []chroma.Token) string {
	var builder strings.Builder
	for _, token := range tokens {
		builder.WriteString(renderPreviewHighlightedToken(token))
	}
	return builder.String()
}

func renderPreviewHighlightedToken(token chroma.Token) string {
	if token.Value == "" {
		return ""
	}
	class := previewSyntaxTokenClass(token.Type)
	text := escapePreviewText(token.Value)
	if class == "" {
		return text
	}
	return `<span class="` + class + `">` + text + `</span>`
}

func previewSyntaxTokenClass(tokenType chroma.TokenType) string {
	for tokenType != 0 {
		if class, ok := chroma.StandardTypes[tokenType]; ok {
			if class != "" {
				return previewSyntaxClassPrefix + class
			}
			return ""
		}
		tokenType = tokenType.Parent()
	}
	if class := chroma.StandardTypes[tokenType]; class != "" {
		return previewSyntaxClassPrefix + class
	}
	return ""
}

func renderMarkdownHTML(content []byte, highlight bool) (string, error) {
	var buf bytes.Buffer
	renderer := plainMarkdownPreviewRenderer
	if highlight {
		renderer = highlightedMarkdownRenderer
	}
	if err := renderer.Convert(content, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
