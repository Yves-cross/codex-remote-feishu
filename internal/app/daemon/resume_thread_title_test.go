package daemon

import (
	"testing"

	"github.com/kxn/codex-remote-feishu/internal/app/daemon/surfaceresume"
	"github.com/kxn/codex-remote-feishu/internal/core/control"
	"github.com/kxn/codex-remote-feishu/internal/core/threadtitle"
)

func TestNormalizeResumeThreadTitle(t *testing.T) {
	t.Parallel()

	threadID := "019d56f0-de5e-7943-bc9a-18c42ef11acb"
	shortID := control.ShortenThreadID(threadID)

	cases := []struct {
		name         string
		title        string
		threadID     string
		threadCWD    string
		workspaceKey string
		want         string
	}{
		{
			name:      "keeps raw title",
			title:     "修复登录流程",
			threadID:  threadID,
			threadCWD: "/data/dl/droid",
			want:      "修复登录流程",
		},
		{
			name:      "strips display prefix",
			title:     "droid · 修复登录流程",
			threadID:  threadID,
			threadCWD: "/data/dl/droid",
			want:      "修复登录流程",
		},
		{
			name:      "strips repeated display prefix and suffix",
			title:     "droid · droid · 修复登录流程 · " + shortID,
			threadID:  threadID,
			threadCWD: "/data/dl/droid",
			want:      "修复登录流程",
		},
		{
			name:      "clears current unnamed display title",
			title:     "droid · 未命名会话",
			threadID:  threadID,
			threadCWD: "/data/dl/droid",
			want:      "",
		},
		{
			name:      "clears legacy short id only display title",
			title:     "droid · " + shortID,
			threadID:  threadID,
			threadCWD: "/data/dl/droid",
			want:      "",
		},
		{
			name:         "uses workspace fallback when cwd missing",
			title:        "droid · 修复登录流程",
			threadID:     threadID,
			workspaceKey: "/data/dl/droid",
			want:         "修复登录流程",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := threadtitle.NormalizeStoredInput(tc.title, threadtitle.Context{
				ThreadID:     tc.threadID,
				ThreadCWD:    tc.threadCWD,
				WorkspaceKey: tc.workspaceKey,
			}); got != tc.want {
				t.Fatalf("threadtitle.NormalizeStoredInput() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSurfaceResumeStoreNormalizesLegacyDisplayThreadTitle(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	store, err := surfaceresume.LoadStore(surfaceresume.StatePath(stateDir))
	if err != nil {
		t.Fatalf("load surface resume store: %v", err)
	}

	threadID := "019d56f0-de5e-7943-bc9a-18c42ef11acb"
	shortID := control.ShortenThreadID(threadID)
	if err := store.Put(surfaceresume.Entry{
		SurfaceSessionID:  "surface-1",
		ProductMode:       "normal",
		ResumeThreadID:    threadID,
		ResumeThreadTitle: "droid · droid · 修复登录流程 · " + shortID,
		ResumeThreadCWD:   "/data/dl/droid",
		ResumeHeadless:    true,
	}); err != nil {
		t.Fatalf("put surface resume entry: %v", err)
	}

	entry, ok := store.Get("surface-1")
	if !ok {
		t.Fatal("expected normalized surface resume entry")
	}
	if entry.ResumeThreadTitle != "修复登录流程" {
		t.Fatalf("expected legacy display title to normalize to raw thread name, got %#v", entry)
	}
}

func TestStoredThreadTitleDoesNotFallBackToThreadID(t *testing.T) {
	t.Parallel()

	if got := threadtitle.StoredTitle("", threadtitle.Context{
		ThreadID:     "thread-1",
		ThreadCWD:    "/data/dl/droid",
		WorkspaceKey: "/data/dl/droid",
	}, nil); got != "" {
		t.Fatalf("expected empty stored title when no reusable thread title exists, got %q", got)
	}
}
