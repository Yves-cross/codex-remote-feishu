package issuedocsync

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	payloads map[string]string
}

func (f fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	cursor := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "after=") {
			cursor = strings.TrimPrefix(arg, "after=")
		}
	}
	return []byte(f.payloads[cursor]), nil
}

func TestListClosedIssueSummariesPaginates(t *testing.T) {
	client := &ghCLI{
		runner: fakeRunner{
			payloads: map[string]string{
				"": `{
  "data": {
    "repository": {
      "issues": {
        "pageInfo": {"hasNextPage": true, "endCursor": "cursor-1"},
        "nodes": [
          {"number": 40, "title": "first", "updatedAt": "2026-04-08T04:00:00Z", "closedAt": "2026-04-08T03:00:00Z", "url": "https://example.com/40"}
        ]
      }
    }
  }
}`,
				"cursor-1": `{
  "data": {
    "repository": {
      "issues": {
        "pageInfo": {"hasNextPage": false, "endCursor": ""},
        "nodes": [
          {"number": 39, "title": "second", "updatedAt": "2026-04-08T02:00:00Z", "closedAt": "2026-04-08T01:00:00Z", "url": "https://example.com/39"}
        ]
      }
    }
  }
}`,
			},
		},
	}

	summaries, err := client.ListClosedIssueSummaries(context.Background(), Repo{Owner: "kxn", Name: "codex-remote-feishu"})
	if err != nil {
		t.Fatalf("ListClosedIssueSummaries error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("summary count = %d, want 2", len(summaries))
	}
	if summaries[0].Number != 40 || summaries[1].Number != 39 {
		t.Fatalf("unexpected summaries: %#v", summaries)
	}
}
