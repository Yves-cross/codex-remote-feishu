package feishu

import "strings"

func inspectGitBranchLabel(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	if branch, ok := runGitInspector(cwd, "branch", "--show-current"); ok {
		branch = strings.TrimSpace(branch)
		if branch != "" {
			return branch
		}
	}
	if _, ok := runGitInspector(cwd, "rev-parse", "--is-inside-work-tree"); !ok {
		return ""
	}
	if shortCommit, ok := runGitInspector(cwd, "rev-parse", "--short", "HEAD"); ok {
		shortCommit = strings.TrimSpace(shortCommit)
		if shortCommit != "" {
			return "detached@" + shortCommit
		}
	}
	return "detached"
}
