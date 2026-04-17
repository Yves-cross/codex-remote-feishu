package issueworkflow

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	parentRequiredSections        = []string{"拆分结构", "推荐顺序", "可并行组", "当前风险", "总调度表", "当前执行点", "恢复步骤"}
	parentScheduleRequiredColumns = []string{"结果回卷", "verifier 状态", "当前结论"}
	parentIssueRefPattern         = regexp.MustCompile(`(?:父|母)\s*issue\s*#(\d+)`)
	issueRefPattern               = regexp.MustCompile(`#(\d+)`)
	verifierResultPattern         = regexp.MustCompile(`独立\s*verifier\s*结果[:：]\s*(pass with gaps|pass|fail)`)
	verifierWaiverMarkers         = []string{"用户显式豁免verifier", "verifier豁免"}
)

type issueStructure struct {
	IsParent                 bool
	ParentIssueNumber        int
	ChildIssueNumbers        []int
	MissingParentSections    []string
	MissingParentSummaryCols []string
	LegacyChildParentRef     bool
	CloseVerifierRequired    bool
}

func analyzeIssueStructure(issueNumber int, body string, sections documentSections) issueStructure {
	structure := issueStructure{
		IsParent: sections.Present["拆分结构"] || sections.Present["总调度表"],
	}
	if parentBody, ok := sections.Bodies["父 issue"]; ok {
		refs := collectIssueRefs(parentBody)
		if len(refs) > 0 {
			structure.ParentIssueNumber = refs[0]
		}
	}
	if structure.ParentIssueNumber == 0 {
		matches := parentIssueRefPattern.FindStringSubmatch(body)
		if len(matches) == 2 {
			if parsed, err := strconv.Atoi(matches[1]); err == nil && parsed > 0 {
				structure.ParentIssueNumber = parsed
				structure.LegacyChildParentRef = !sections.Present["父 issue"]
			}
		}
	}
	if structure.IsParent {
		for _, section := range parentRequiredSections {
			if !sections.Present[section] {
				structure.MissingParentSections = append(structure.MissingParentSections, section)
			}
		}
		scheduleBody := sections.Bodies["总调度表"]
		normalizedSchedule := normalizeForContains(scheduleBody)
		for _, column := range parentScheduleRequiredColumns {
			if !strings.Contains(normalizedSchedule, normalizeForContains(column)) {
				structure.MissingParentSummaryCols = append(structure.MissingParentSummaryCols, column)
			}
		}
		structure.ChildIssueNumbers = uniqueIssueRefs(
			collectIssueRefs(sections.Bodies["拆分结构"]),
			collectIssueRefs(sections.Bodies["总调度表"]),
		)
		filtered := structure.ChildIssueNumbers[:0]
		for _, ref := range structure.ChildIssueNumbers {
			if ref > 0 && ref != issueNumber {
				filtered = append(filtered, ref)
			}
		}
		structure.ChildIssueNumbers = append([]int(nil), filtered...)
	}
	normalizedBody := normalizeForContains(body)
	structure.CloseVerifierRequired = structure.IsParent ||
		structure.ParentIssueNumber > 0 ||
		sections.Present["建议范围"] ||
		sections.Present["执行快照"] ||
		strings.Contains(normalizedBody, normalizeForContains("当前执行点")) ||
		strings.Contains(normalizedBody, normalizeForContains("恢复步骤"))
	return structure
}

func collectIssueRefs(text string) []int {
	matches := issueRefPattern.FindAllStringSubmatch(text, -1)
	refs := make([]int, 0, len(matches))
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		parsed, err := strconv.Atoi(match[1])
		if err != nil || parsed <= 0 {
			continue
		}
		refs = append(refs, parsed)
	}
	return refs
}

func uniqueIssueRefs(groups ...[]int) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0)
	for _, group := range groups {
		for _, ref := range group {
			if _, ok := seen[ref]; ok {
				continue
			}
			seen[ref] = struct{}{}
			out = append(out, ref)
		}
	}
	sort.Ints(out)
	return out
}

func (s *Service) closeGateChecks(ctx context.Context, repo Repo, issue Issue, mode WorkflowMode) ([]CheckResult, error) {
	structure := analyzeIssueStructure(issue.Number, issue.Body, scanDocumentSections(issue.Body))
	checks := make([]CheckResult, 0, 3)
	if verifierCheck := closeVerifierGate(issue, structure, mode); verifierCheck != nil {
		checks = append(checks, *verifierCheck)
	}
	if structure.ParentIssueNumber > 0 {
		if structure.LegacyChildParentRef {
			checks = append(checks, CheckResult{
				Name:    "issue_close_child_contract_gate",
				Status:  CheckStatusFail,
				Message: "child issue close is blocked until the parent link is promoted into a dedicated `父 issue` section",
			})
		}
		parent, err := s.GitHub.FetchIssue(ctx, repo, structure.ParentIssueNumber, closeGateCommentsLimit)
		if err != nil {
			return checks, fmt.Errorf("fetch parent issue #%d: %w", structure.ParentIssueNumber, err)
		}
		if hasChildRollup(parent, issue.Number) {
			checks = append(checks, CheckResult{Name: "issue_close_rollup_gate", Status: CheckStatusPass, Message: "ok"})
		} else {
			checks = append(checks, CheckResult{
				Name:    "issue_close_rollup_gate",
				Status:  CheckStatusFail,
				Message: fmt.Sprintf("child issue close is blocked until parent issue #%d records a durable roll-up for #%d", structure.ParentIssueNumber, issue.Number),
			})
		}
	}
	if structure.IsParent {
		problems := make([]string, 0)
		if len(structure.MissingParentSections) > 0 {
			problems = append(problems, "missing parent sections: "+strings.Join(structure.MissingParentSections, ", "))
		}
		if len(structure.MissingParentSummaryCols) > 0 {
			problems = append(problems, "missing parent summary columns: "+strings.Join(structure.MissingParentSummaryCols, ", "))
		}
		missingRollups := missingChildRollups(issue, structure.ChildIssueNumbers)
		if len(missingRollups) > 0 {
			problems = append(problems, "missing child roll-ups for: "+strings.Join(missingRollups, ", "))
		}
		if len(problems) == 0 {
			checks = append(checks, CheckResult{Name: "issue_close_parent_summary_gate", Status: CheckStatusPass, Message: "ok"})
		} else {
			checks = append(checks, CheckResult{
				Name:    "issue_close_parent_summary_gate",
				Status:  CheckStatusFail,
				Message: strings.Join(problems, "; "),
			})
		}
	}
	return checks, nil
}

func closeVerifierGate(issue Issue, structure issueStructure, mode WorkflowMode) *CheckResult {
	if !structure.CloseVerifierRequired {
		return nil
	}
	if mode == WorkflowModeFast {
		return &CheckResult{Name: "issue_close_verifier_gate", Status: CheckStatusPass, Message: "ok (fast mode)"}
	}
	if hasVerifierWaiver(issue) {
		return &CheckResult{Name: "issue_close_verifier_gate", Status: CheckStatusPass, Message: "ok (user waiver recorded)"}
	}
	result, ok := lastVerifierResult(issue)
	if !ok {
		return &CheckResult{
			Name:    "issue_close_verifier_gate",
			Status:  CheckStatusFail,
			Message: "medium/large issue close is blocked until a durable `独立 verifier 结果：pass` record exists",
		}
	}
	if result != "pass" {
		return &CheckResult{
			Name:    "issue_close_verifier_gate",
			Status:  CheckStatusFail,
			Message: "close is blocked because the latest verifier result is `" + result + "`",
		}
	}
	return &CheckResult{Name: "issue_close_verifier_gate", Status: CheckStatusPass, Message: "ok"}
}

func lastVerifierResult(issue Issue) (string, bool) {
	latest := ""
	for _, text := range verifierSearchTexts(issue) {
		matches := verifierResultPattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) == 2 {
				latest = strings.TrimSpace(strings.ToLower(match[1]))
			}
		}
	}
	if latest == "" {
		return "", false
	}
	return latest, true
}

func hasVerifierWaiver(issue Issue) bool {
	normalized := normalizeForContains(strings.Join(verifierSearchTexts(issue), "\n"))
	for _, marker := range verifierWaiverMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func verifierSearchTexts(issue Issue) []string {
	texts := make([]string, 0, len(issue.Comments)+1)
	texts = append(texts, issue.Body)
	for _, comment := range issue.Comments {
		texts = append(texts, comment.Body)
	}
	return texts
}

func hasChildRollup(issue Issue, childNumber int) bool {
	pattern := regexp.MustCompile(fmt.Sprintf(`子 issue\s+`+"`?"+`#%d`+"`?"+`\s+已完成并关闭`, childNumber))
	for _, text := range verifierSearchTexts(issue) {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func missingChildRollups(issue Issue, childNumbers []int) []string {
	missing := make([]string, 0)
	for _, child := range childNumbers {
		if hasChildRollup(issue, child) {
			continue
		}
		missing = append(missing, fmt.Sprintf("#%d", child))
	}
	return missing
}
