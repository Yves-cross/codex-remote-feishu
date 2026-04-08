package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/kxn/codex-remote-feishu/internal/issuedocsync"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "issue-doc-sync: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError("missing command")
	}
	switch args[0] {
	case "plan":
		return runPlan(ctx, args[1:])
	default:
		return usageError("unknown command %q", args[0])
	}
}

func runPlan(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	repoValue := fs.String("repo", "", "GitHub repo in owner/name form")
	statePath := fs.String("state-file", ".codex/state/issue-doc-sync/state.json", "tracked sync state file")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repoValue == "" {
		return usageError("plan requires --repo")
	}

	repo, err := issuedocsync.ParseRepo(*repoValue)
	if err != nil {
		return err
	}
	client := issuedocsync.NewGitHubCLI()
	summaries, err := client.ListClosedIssueSummaries(ctx, repo)
	if err != nil {
		return err
	}
	state, err := issuedocsync.LoadState(*statePath, repo.String())
	if err != nil {
		return err
	}
	report := issuedocsync.BuildPlanReport(repo.String(), summaries, state)
	return issuedocsync.WritePlanReport(os.Stdout, report, *format)
}

func usageError(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return errors.New(msg + "\nusage:\n  go run ./cmd/issue-doc-sync plan --repo owner/name [--state-file path] [--format text|json]")
}
