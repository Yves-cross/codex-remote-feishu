---
name: issue-verifier
description: "Independent read-only verification for nearly finished GitHub issue work in this repository. Use when the user asks for 验收, 独立验证, 对齐验证, 完成前复核, or when a medium/large issue needs a separate acceptance pass before closing."
---

# Issue Verifier

Use this skill for an independent acceptance pass after implementation has largely stabilized.

Default to read-only review. Do not silently continue implementation unless the user explicitly asks for fixes in the same step.

If the issue is still being shaped, still missing execution closure, or still mid-implementation, hand control back to `$issue-workflow-guardrail` instead of forcing verification early.

## Read Order

Review in this order:

1. issue body
2. latest maintainer or user comments
3. linked design docs and acceptance references
4. changed files / diff / validation evidence
5. current worktree state and publish state if close-out is in scope

Treat later maintainer or user direction as authoritative when it conflicts with older issue body text.

## Verification Checklist

Verify these dimensions explicitly:

- goal alignment
  - does the implementation solve the issue's stated goal
- non-goal discipline
  - did implementation drift into areas that were intentionally excluded
- acceptance coverage
  - is each acceptance item satisfied, or is anything still missing
- regression surface
  - are there unverified risky paths or likely regressions
- durable knowledge sync
  - should issue body, docs, state-machine docs, AGENTS, or skills have been updated
- close-out readiness
  - is the issue actually ready to close, or is it only locally complete

If the issue is medium/large, also check whether parent/child issue boundaries stayed coherent and whether deferred follow-ups were recorded in a durable place.

## Output Format

Use a findings-first review style.

Expected shape:

- findings first
  - order by severity
  - include file references when the finding is code-specific
- then open questions or assumptions
- then pass/fail recommendation
  - `pass`
  - `pass with gaps`
  - `fail`
- then a short close-out note
  - what remains before closure
  - whether durable knowledge sync is missing

If there are no findings, say that explicitly and still note any residual validation gaps.

## Guardrails

- Do not silently change code just because a fix looks obvious.
- Do not downgrade acceptance gaps into stylistic suggestions.
- Do not close the issue yourself unless the user explicitly asked for that action as part of the verifier run.
- If the verifier uncovers only trivial, clearly bounded gaps, report them first; implementation can happen in a follow-up step.
- If the issue lacks enough durable context to verify cleanly, say that the issue needs reshaping or closure-index repair and hand it back to `$issue-workflow-guardrail`.

## Typical Triggers

- `验收 #123`
- `独立验证 #123`
- `对齐验证 #123`
- `完成前复核 #123`
- "这个 issue 准备关了，帮我独立审一遍"

For broader orchestration or ongoing implementation, switch back to `$issue-workflow-guardrail`.
