package preview

import (
	"strings"
	"testing"
)

func TestPlanWebPreviewRenderKeepsMarkdownAsProseByDefault(t *testing.T) {
	content := []byte("# Title\n\nBody\n")
	plan := planWebPreviewRender(webPreviewRenderInput{
		Current: &webPreviewArtifact{
			Record: webPreviewRecord{
				RendererKind: "markdown",
				SourcePath:   "docs/design.md",
				SizeBytes:    int64(len(content)),
			},
			Content: content,
		},
	})

	if plan.Layout != webPreviewLayoutDocument {
		t.Fatalf("layout = %q, want %q", plan.Layout, webPreviewLayoutDocument)
	}
	if len(plan.Choices) != 2 {
		t.Fatalf("choices len = %d, want 2", len(plan.Choices))
	}
	if plan.Choices[0].RendererKey != webPreviewRendererMarkdownProse {
		t.Fatalf("first choice = %q, want %q", plan.Choices[0].RendererKey, webPreviewRendererMarkdownProse)
	}
	if plan.Choices[1].RendererKey != webPreviewRendererNumberedSource {
		t.Fatalf("fallback choice = %q, want %q", plan.Choices[1].RendererKey, webPreviewRendererNumberedSource)
	}
	if got := joinWebPreviewNoticeParts(plan.NoticeParts); got != "" {
		t.Fatalf("default markdown notice = %q, want empty", got)
	}
}

func TestPlanWebPreviewRenderUsesSourceForMarkdownLocation(t *testing.T) {
	content := []byte("# Title\n\nBody\n")
	plan := planWebPreviewRender(webPreviewRenderInput{
		Current: &webPreviewArtifact{
			Record: webPreviewRecord{
				RendererKind: "markdown",
				SourcePath:   "docs/design.md",
				SizeBytes:    int64(len(content)),
			},
			Content: content,
		},
		Location: PreviewLocation{Line: 3},
	})

	if len(plan.Choices) != 1 {
		t.Fatalf("choices len = %d, want 1", len(plan.Choices))
	}
	if plan.Choices[0].RendererKey != webPreviewRendererNumberedSource {
		t.Fatalf("choice = %q, want %q", plan.Choices[0].RendererKey, webPreviewRendererNumberedSource)
	}
	if notice := joinWebPreviewNoticeParts(plan.NoticeParts); !strings.Contains(notice, "当前按源码视图展示") {
		t.Fatalf("location notice = %q, want source-view explanation", notice)
	}
}

func TestPlanWebPreviewRenderUsesNumberedSourceForSourceLikeDefaults(t *testing.T) {
	content := []byte("package main\n")
	plan := planWebPreviewRender(webPreviewRenderInput{
		Current: &webPreviewArtifact{
			Record: webPreviewRecord{
				RendererKind: "text",
				SourcePath:   "internal/main.go",
				SizeBytes:    int64(len(content)),
			},
			Content: content,
		},
	})

	if len(plan.Choices) != 2 {
		t.Fatalf("choices len = %d, want 2", len(plan.Choices))
	}
	if plan.Choices[0].RendererKey != webPreviewRendererNumberedHighlightedSource {
		t.Fatalf("first choice = %q, want %q", plan.Choices[0].RendererKey, webPreviewRendererNumberedHighlightedSource)
	}
	if plan.Choices[1].RendererKey != webPreviewRendererNumberedSource {
		t.Fatalf("fallback choice = %q, want %q", plan.Choices[1].RendererKey, webPreviewRendererNumberedSource)
	}
	if got := joinWebPreviewNoticeParts(plan.NoticeParts); got != "" {
		t.Fatalf("source-like notice = %q, want empty", got)
	}
}

func TestPlanWebPreviewRenderKeepsHTMLSourceSafetyNotice(t *testing.T) {
	content := []byte("<script>alert(1)</script>\n")
	plan := planWebPreviewRender(webPreviewRenderInput{
		Current: &webPreviewArtifact{
			Record: webPreviewRecord{
				RendererKind: "html_source",
				SourcePath:   "docs/unsafe.html",
				SizeBytes:    int64(len(content)),
			},
			Content: content,
		},
	})

	if len(plan.Choices) != 2 {
		t.Fatalf("choices len = %d, want 2", len(plan.Choices))
	}
	if plan.Choices[0].RendererKey != webPreviewRendererNumberedHighlightedSource {
		t.Fatalf("first choice = %q, want %q", plan.Choices[0].RendererKey, webPreviewRendererNumberedHighlightedSource)
	}
	if plan.Choices[1].RendererKey != webPreviewRendererNumberedSource {
		t.Fatalf("fallback choice = %q, want %q", plan.Choices[1].RendererKey, webPreviewRendererNumberedSource)
	}
	if notice := joinWebPreviewNoticeParts(plan.NoticeParts); !strings.Contains(notice, "HTML 以源码方式展示") {
		t.Fatalf("html notice = %q, want safety notice", notice)
	}
}

func TestPlanWebPreviewRenderPreservesLargeFileFallbacks(t *testing.T) {
	originalThreshold := previewDiffFirstThresholdBytes
	previewDiffFirstThresholdBytes = 1
	defer func() { previewDiffFirstThresholdBytes = originalThreshold }()

	current := &webPreviewArtifact{
		Record: webPreviewRecord{
			RendererKind: "text",
			SourcePath:   "docs/note.txt",
			SizeBytes:    16,
		},
		Content: []byte("beta\n"),
	}
	previous := &webPreviewArtifact{
		Record: webPreviewRecord{
			RendererKind: "text",
			SourcePath:   "docs/note.txt",
			SizeBytes:    16,
		},
		Content: []byte("alpha\n"),
	}

	diffPlan := planWebPreviewRender(webPreviewRenderInput{
		Current:  current,
		Previous: previous,
	})
	if len(diffPlan.Choices) != 1 || diffPlan.Choices[0].RendererKey != webPreviewRendererDiff {
		t.Fatalf("diff plan choices = %+v, want diff", diffPlan.Choices)
	}

	summaryPlan := planWebPreviewRender(webPreviewRenderInput{
		Current: current,
	})
	if len(summaryPlan.Choices) != 1 || summaryPlan.Choices[0].RendererKey != webPreviewRendererSummary {
		t.Fatalf("summary plan choices = %+v, want summary", summaryPlan.Choices)
	}

	htmlPlan := planWebPreviewRender(webPreviewRenderInput{
		Current: &webPreviewArtifact{
			Record: webPreviewRecord{
				RendererKind: "html_source",
				SourcePath:   "docs/unsafe.html",
				SizeBytes:    16,
			},
			Content: []byte("<div>body</div>"),
		},
	})
	if len(htmlPlan.Choices) != 2 || htmlPlan.Choices[0].RendererKey != webPreviewRendererNumberedHighlightedSource || htmlPlan.Choices[1].RendererKey != webPreviewRendererNumberedSource {
		t.Fatalf("html plan choices = %+v, want numbered source fallback chain", htmlPlan.Choices)
	}
	if notice := joinWebPreviewNoticeParts(htmlPlan.NoticeParts); !strings.Contains(notice, "HTML 以源码方式展示") {
		t.Fatalf("html notice = %q, want safety notice", notice)
	}
	if strings.Contains(joinWebPreviewNoticeParts(htmlPlan.NoticeParts), "页面只展示摘要") {
		t.Fatalf("html plan should not degrade to summary-only by default, got %q", joinWebPreviewNoticeParts(htmlPlan.NoticeParts))
	}
}
