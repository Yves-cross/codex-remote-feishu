package preview

import "strings"

type webPreviewRendererKey string

const (
	webPreviewRendererNumberedSource            webPreviewRendererKey = "numbered_source"
	webPreviewRendererNumberedHighlightedSource webPreviewRendererKey = "numbered_highlighted_source"
	webPreviewRendererMarkdownProse             webPreviewRendererKey = "markdown_prose"
	webPreviewRendererDiff                      webPreviewRendererKey = "diff"
	webPreviewRendererSummary                   webPreviewRendererKey = "summary"
	webPreviewRendererImage                     webPreviewRendererKey = "image"
	webPreviewRendererPDF                       webPreviewRendererKey = "pdf"
	webPreviewRendererUnsupportedMessage        webPreviewRendererKey = "unsupported_message"
)

type webPreviewRenderInput struct {
	Current      *webPreviewArtifact
	Previous     *webPreviewArtifact
	DownloadHref string
	PageTitle    string
	Location     PreviewLocation
}

type webPreviewRendererChoice struct {
	RendererKey webPreviewRendererKey
	NoticeParts []string
}

type webPreviewRenderPlan struct {
	Layout      webPreviewLayout
	NoticeParts []string
	Choices     []webPreviewRendererChoice
}

type webPreviewRenderer interface {
	Key() webPreviewRendererKey
	Render(webPreviewRenderInput) (bodyHTML string, ok bool, err error)
}

type webPreviewRendererFunc struct {
	key    webPreviewRendererKey
	render func(webPreviewRenderInput) (string, bool, error)
}

func (f webPreviewRendererFunc) Key() webPreviewRendererKey { return f.key }

func (f webPreviewRendererFunc) Render(input webPreviewRenderInput) (string, bool, error) {
	if f.render == nil {
		return "", false, nil
	}
	return f.render(input)
}

type webPreviewRendererRegistry struct {
	renderers map[webPreviewRendererKey]webPreviewRenderer
}

func newWebPreviewRendererRegistry(renderers ...webPreviewRenderer) webPreviewRendererRegistry {
	registry := webPreviewRendererRegistry{
		renderers: map[webPreviewRendererKey]webPreviewRenderer{},
	}
	for _, renderer := range renderers {
		if renderer == nil {
			continue
		}
		registry.renderers[renderer.Key()] = renderer
	}
	return registry
}

func (r webPreviewRendererRegistry) render(choice webPreviewRendererChoice, input webPreviewRenderInput) (string, bool, error) {
	renderer := r.renderers[choice.RendererKey]
	if renderer == nil {
		return "", false, nil
	}
	return renderer.Render(input)
}

var defaultWebPreviewRendererRegistry = newWebPreviewRendererRegistry(
	webPreviewRendererFunc{
		key: webPreviewRendererNumberedSource,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			if input.Current == nil {
				return "", false, nil
			}
			return renderNumberedSourcePreviewHTML(input.Current.Content, input.Location), true, nil
		},
	},
	webPreviewRendererFunc{
		key: webPreviewRendererNumberedHighlightedSource,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			if input.Current == nil {
				return "", false, nil
			}
			bodyHTML, err := renderNumberedHighlightedSourcePreviewHTML(input.Current.Record.SourcePath, input.Current.Content, input.Location)
			return bodyHTML, strings.TrimSpace(bodyHTML) != "", err
		},
	},
	webPreviewRendererFunc{
		key: webPreviewRendererMarkdownProse,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			if input.Current == nil {
				return "", false, nil
			}
			html, err := renderMarkdownHTML(input.Current.Content, shouldHighlightMarkdownPreview(input.Current.Record))
			if err != nil {
				return "", false, err
			}
			return `<article class="preview-prose">` + html + `</article>`, true, nil
		},
	},
	webPreviewRendererFunc{
		key: webPreviewRendererDiff,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			return renderDiffPreviewHTML(input.Previous, input.Current), true, nil
		},
	},
	webPreviewRendererFunc{
		key: webPreviewRendererSummary,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			if input.Current == nil {
				return "", false, nil
			}
			return renderTextSummaryHTML(input.Current.Content), true, nil
		},
	},
	webPreviewRendererFunc{
		key: webPreviewRendererImage,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			return `<img class="preview-image" src="` + escapePreviewText(appendPreviewInlineQuery(input.DownloadHref)) + `" alt="` + escapePreviewText(input.PageTitle) + `" />`, true, nil
		},
	},
	webPreviewRendererFunc{
		key: webPreviewRendererPDF,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			return `<iframe class="preview-pdf" src="` + escapePreviewText(appendPreviewInlineQuery(input.DownloadHref)) + `" title="` + escapePreviewText(input.PageTitle) + `"></iframe>`, true, nil
		},
	},
	webPreviewRendererFunc{
		key: webPreviewRendererUnsupportedMessage,
		render: func(input webPreviewRenderInput) (string, bool, error) {
			return `<p>这份文件保留了快照下载能力，但当前不会在页面内直接展开。</p>`, true, nil
		},
	},
)

func executeWebPreviewRenderPlan(plan webPreviewRenderPlan, input webPreviewRenderInput) (string, []string) {
	noticeParts := append([]string(nil), plan.NoticeParts...)
	for _, choice := range plan.Choices {
		bodyHTML, ok, err := defaultWebPreviewRendererRegistry.render(choice, input)
		if err != nil || !ok || strings.TrimSpace(bodyHTML) == "" {
			continue
		}
		return bodyHTML, append(noticeParts, choice.NoticeParts...)
	}
	fallbackBody, _, _ := defaultWebPreviewRendererRegistry.render(webPreviewRendererChoice{
		RendererKey: webPreviewRendererUnsupportedMessage,
	}, input)
	if strings.TrimSpace(fallbackBody) == "" {
		fallbackBody = `<p>当前没有可展示的正文内容。</p>`
	}
	return fallbackBody, noticeParts
}
