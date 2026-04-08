package issuedocsync

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

func BuildPlanReport(repo string, summaries []IssueSummary, state StateFile) PlanReport {
	report := PlanReport{
		Repo:             repo,
		ScannedClosed:    len(summaries),
		CachedIssueCount: len(state.Issues),
		Candidates:       make([]PlanCandidate, 0),
	}
	for _, summary := range summaries {
		key := fmt.Sprintf("%d", summary.Number)
		record, ok := state.Issues[key]
		currentUpdatedAt := summary.UpdatedAt.UTC().Format(time.RFC3339)
		if !ok {
			report.Candidates = append(report.Candidates, PlanCandidate{
				Number:    summary.Number,
				Title:     summary.Title,
				UpdatedAt: currentUpdatedAt,
				URL:       summary.URL,
				Reason:    "not yet recorded in tracked issue-doc sync state",
			})
			continue
		}
		if record.UpdatedAt != currentUpdatedAt {
			report.Candidates = append(report.Candidates, PlanCandidate{
				Number:            summary.Number,
				Title:             summary.Title,
				UpdatedAt:         currentUpdatedAt,
				URL:               summary.URL,
				Reason:            "issue updated since the recorded sync decision",
				PreviousUpdatedAt: record.UpdatedAt,
			})
		}
	}
	sort.Slice(report.Candidates, func(i, j int) bool {
		return report.Candidates[i].Number > report.Candidates[j].Number
	})
	report.CandidateCount = len(report.Candidates)
	return report
}

func WritePlanReport(w io.Writer, report PlanReport, format string) error {
	switch format {
	case "json":
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(payload))
		return err
	case "text":
		if _, err := fmt.Fprintf(w, "repo: %s\nclosed issues scanned: %d\ntracked cache entries: %d\ncandidates: %d\n", report.Repo, report.ScannedClosed, report.CachedIssueCount, report.CandidateCount); err != nil {
			return err
		}
		if len(report.Candidates) == 0 {
			_, err := fmt.Fprintln(w, "no changed closed issues need review")
			return err
		}
		for _, candidate := range report.Candidates {
			if _, err := fmt.Fprintf(w, "- #%d %s\n  updatedAt: %s\n  reason: %s\n", candidate.Number, candidate.Title, candidate.UpdatedAt, candidate.Reason); err != nil {
				return err
			}
			if candidate.PreviousUpdatedAt != "" {
				if _, err := fmt.Fprintf(w, "  previousUpdatedAt: %s\n", candidate.PreviousUpdatedAt); err != nil {
					return err
				}
			}
			if candidate.URL != "" {
				if _, err := fmt.Fprintf(w, "  url: %s\n", candidate.URL); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}
