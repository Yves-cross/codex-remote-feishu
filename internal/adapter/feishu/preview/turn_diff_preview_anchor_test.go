package preview

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/core/control"
)

func TestBuildTurnDiffPreviewArtifactResolvesNestedWorkspacePath(t *testing.T) {
	root := t.TempDir()
	parentRoot := filepath.Join(root, "oclaw")
	workspaceRoot := filepath.Join(parentRoot, "openclaw-v2026.4.2")
	afterText := strings.Join([]string{
		"// header",
		"",
		"import { a } from \"./a\";",
		"const x = 1;",
		"const y = 2;",
		"export default x + y;",
		"",
	}, "\n")
	writePreviewFile(t, filepath.Join(workspaceRoot, "extensions", "deepseek", "index.ts"), afterText)
	beforeText := strings.Join([]string{
		"// header",
		"",
		"import { a } from \"./a\";",
		"const x = 1;",
		"export default x;",
		"",
	}, "\n")
	diff := strings.Join([]string{
		"diff --git a/extensions/deepseek/index.ts b/extensions/deepseek/index.ts",
		turnDiffTestIndexLine(beforeText, afterText),
		"--- a/extensions/deepseek/index.ts",
		"+++ b/extensions/deepseek/index.ts",
		"@@ -3,3 +3,4 @@",
		" import { a } from \"./a\";",
		" const x = 1;",
		"-export default x;",
		"+const y = 2;",
		"+export default x + y;",
		"",
	}, "\n")

	previewer := NewDriveMarkdownPreviewer(nil, MarkdownPreviewConfig{
		ProcessCWD: filepath.Join(root, "unrelated"),
		CacheDir:   filepath.Join(root, "preview-cache"),
	})
	artifact, err := previewer.buildTurnDiffPreviewArtifact(FinalBlockPreviewRequest{
		WorkspaceRoot: parentRoot,
		ThreadCWD:     parentRoot,
		TurnDiffSnapshot: &control.TurnDiffSnapshot{
			Diff: diff,
		},
	})
	if err != nil {
		t.Fatalf("build turn diff preview artifact: %v", err)
	}
	if artifact == nil || len(artifact.Files) != 1 {
		t.Fatalf("expected one file artifact, got %#v", artifact)
	}
	file := artifact.Files[0]
	if file.ParseStatus != "ok" {
		t.Fatalf("expected merged full-file view, got parseStatus=%q file=%#v", file.ParseStatus, file)
	}
	if len(file.Lines) == 0 || file.Lines[0].Now != "1" {
		t.Fatalf("expected full-file lines from top, got %#v", file.Lines)
	}
	if file.AfterText != afterText {
		t.Fatalf("expected afterText to match file content, got %q", file.AfterText)
	}
}

func TestBuildTurnDiffPreviewArtifactFallsBackWhenNestedPathAmbiguous(t *testing.T) {
	root := t.TempDir()
	parentRoot := filepath.Join(root, "work")
	contentA := "line-1\nline-2\nline-3-new\n"
	contentB := "line-1\nline-2\nline-3-other\n"
	writePreviewFile(t, filepath.Join(parentRoot, "proj-a", "extensions", "deepseek", "index.ts"), contentA)
	writePreviewFile(t, filepath.Join(parentRoot, "proj-b", "extensions", "deepseek", "index.ts"), contentB)
	diff := strings.Join([]string{
		"diff --git a/extensions/deepseek/index.ts b/extensions/deepseek/index.ts",
		"--- a/extensions/deepseek/index.ts",
		"+++ b/extensions/deepseek/index.ts",
		"@@ -1,3 +1,3 @@",
		" line-1",
		" line-2",
		"-line-3-old",
		"+line-3-new",
		"",
	}, "\n")

	previewer := NewDriveMarkdownPreviewer(nil, MarkdownPreviewConfig{
		ProcessCWD: filepath.Join(root, "unrelated"),
		CacheDir:   filepath.Join(root, "preview-cache"),
	})
	artifact, err := previewer.buildTurnDiffPreviewArtifact(FinalBlockPreviewRequest{
		WorkspaceRoot: parentRoot,
		ThreadCWD:     parentRoot,
		TurnDiffSnapshot: &control.TurnDiffSnapshot{
			Diff: diff,
		},
	})
	if err != nil {
		t.Fatalf("build turn diff preview artifact: %v", err)
	}
	if artifact == nil || len(artifact.Files) != 1 {
		t.Fatalf("expected one file artifact, got %#v", artifact)
	}
	file := artifact.Files[0]
	if file.ParseStatus != "patch_only" {
		t.Fatalf("expected patch_only fallback for ambiguous nested paths, got %q", file.ParseStatus)
	}
}

func TestBuildTurnDiffPreviewArtifactUsesBlobHashToDisambiguateNestedPath(t *testing.T) {
	root := t.TempDir()
	parentRoot := filepath.Join(root, "work")
	beforeText := "line-1\nline-2-old\nline-3\n"
	afterText := "line-1\nline-2-new\nline-3\n"
	writePreviewFile(t, filepath.Join(parentRoot, "proj-a", "extensions", "deepseek", "index.ts"), afterText)
	writePreviewFile(t, filepath.Join(parentRoot, "proj-b", "extensions", "deepseek", "index.ts"), "line-1\nline-2-other\nline-3\n")
	diff := strings.Join([]string{
		"diff --git a/extensions/deepseek/index.ts b/extensions/deepseek/index.ts",
		turnDiffTestIndexLine(beforeText, afterText),
		"--- a/extensions/deepseek/index.ts",
		"+++ b/extensions/deepseek/index.ts",
		"@@ -1,3 +1,3 @@",
		" line-1",
		"-line-2-old",
		"+line-2-new",
		" line-3",
		"",
	}, "\n")

	previewer := NewDriveMarkdownPreviewer(nil, MarkdownPreviewConfig{
		ProcessCWD: filepath.Join(root, "unrelated"),
		CacheDir:   filepath.Join(root, "preview-cache"),
	})
	artifact, err := previewer.buildTurnDiffPreviewArtifact(FinalBlockPreviewRequest{
		WorkspaceRoot: parentRoot,
		ThreadCWD:     parentRoot,
		TurnDiffSnapshot: &control.TurnDiffSnapshot{
			Diff: diff,
		},
	})
	if err != nil {
		t.Fatalf("build turn diff preview artifact: %v", err)
	}
	if artifact == nil || len(artifact.Files) != 1 {
		t.Fatalf("expected one file artifact, got %#v", artifact)
	}
	file := artifact.Files[0]
	if file.ParseStatus != "ok" {
		t.Fatalf("expected blob hash to disambiguate nested path, got parseStatus=%q", file.ParseStatus)
	}
	if file.AfterText != afterText {
		t.Fatalf("expected afterText from hash-matched file, got %q", file.AfterText)
	}
}

func turnDiffTestIndexLine(beforeText, afterText string) string {
	oldID := turnDiffGitBlobHash([]byte(beforeText))
	newID := turnDiffGitBlobHash([]byte(afterText))
	return "index " + oldID[:12] + ".." + newID[:12] + " 100644"
}
