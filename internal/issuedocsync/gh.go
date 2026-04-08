package issuedocsync

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const listClosedIssuesQuery = `
query($owner: String!, $name: String!, $after: String) {
  repository(owner: $owner, name: $name) {
    issues(first: 100, states: CLOSED, orderBy: {field: UPDATED_AT, direction: DESC}, after: $after) {
      pageInfo {
        hasNextPage
        endCursor
      }
      nodes {
        number
        title
        updatedAt
        closedAt
        url
      }
    }
  }
}
`

type Runner interface {
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type ghCLI struct {
	runner Runner
}

func NewGitHubCLI() *ghCLI {
	return &ghCLI{runner: execRunner{}}
}

func (c *ghCLI) ListClosedIssueSummaries(ctx context.Context, repo Repo) ([]IssueSummary, error) {
	type graphQLResponse struct {
		Data struct {
			Repository struct {
				Issues struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []struct {
						Number    int    `json:"number"`
						Title     string `json:"title"`
						UpdatedAt string `json:"updatedAt"`
						ClosedAt  string `json:"closedAt"`
						URL       string `json:"url"`
					} `json:"nodes"`
				} `json:"issues"`
			} `json:"repository"`
		} `json:"data"`
	}

	after := ""
	all := make([]IssueSummary, 0, 128)
	for {
		args := []string{
			"api", "graphql",
			"-f", "query=" + listClosedIssuesQuery,
			"-f", "owner=" + repo.Owner,
			"-f", "name=" + repo.Name,
		}
		if after != "" {
			args = append(args, "-f", "after="+after)
		}
		payload, err := c.runner.Run(ctx, args...)
		if err != nil {
			return nil, err
		}
		var resp graphQLResponse
		if err := json.Unmarshal(payload, &resp); err != nil {
			return nil, fmt.Errorf("decode github graphql response: %w", err)
		}
		for _, node := range resp.Data.Repository.Issues.Nodes {
			updatedAt, err := time.Parse(time.RFC3339, node.UpdatedAt)
			if err != nil {
				return nil, fmt.Errorf("parse issue #%d updatedAt: %w", node.Number, err)
			}
			var closedAt time.Time
			if node.ClosedAt != "" {
				closedAt, err = time.Parse(time.RFC3339, node.ClosedAt)
				if err != nil {
					return nil, fmt.Errorf("parse issue #%d closedAt: %w", node.Number, err)
				}
			}
			all = append(all, IssueSummary{
				Number:    node.Number,
				Title:     node.Title,
				UpdatedAt: updatedAt,
				ClosedAt:  closedAt,
				URL:       node.URL,
			})
		}
		if !resp.Data.Repository.Issues.PageInfo.HasNextPage {
			return all, nil
		}
		after = resp.Data.Repository.Issues.PageInfo.EndCursor
	}
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	payload, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gh %v: %w\n%s", args, err, string(payload))
	}
	return payload, nil
}
