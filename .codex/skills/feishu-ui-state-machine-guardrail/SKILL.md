---
name: feishu-ui-state-machine-guardrail
description: Audit and update this repository's canonical Feishu card UI state machine after changing card navigation, callback payloads, inline replace behavior, lifecycle stamping, or old-card handling. Use after implementation stabilizes and before committing.
---

# Feishu UI State Machine Guardrail

Treat [docs/general/feishu-card-ui-state-machine.md](../../../docs/general/feishu-card-ui-state-machine.md) as the canonical reference for current Feishu card UI / callback-layer behavior.

Use this skill once per implementation pass, after the code and tests are mostly stable and before committing. Do not trigger it after every tiny edit.

## Workflow

1. Read the canonical document and the touched Feishu UI code paths.
2. Update the document so it matches the current implementation:
   - owner classification
   - callback `kind` and payload fields
   - form submit conventions
   - inline replace vs append-only boundary
   - `daemon_lifecycle_id` stamping and old-card semantics
   - current test baseline
3. Audit the changed behavior for Feishu UI dead-ends or stale-action leaks:
   - a same-context navigation action unexpectedly appends a new card instead of replacing
   - a state-changing action now replaces the current card and hides the real result
   - a stale card can still mutate product state
   - projector and gateway drift on payload keys or field names
   - lifecycle stamping is missing on cards that depend on current-daemon freshness
4. If the audit reveals a bug-grade issue, fix it before commit, add or update tests, then run this audit flow one more time.
5. If the remaining issue is a product tradeoff instead of a bug, append it under `## 待讨论取舍` in the canonical document.

## Update Rules

1. Keep “current implemented behavior” separate from “future controller / architecture ideas”.
2. When a change also affects attach/use/follow/new/request-gate product semantics, run the remote surface guardrail in the same pass.
3. Update `docs/README.md` when the canonical document path or classification changes.
4. Update `AGENTS.md` if the set of default trigger scenarios for this guardrail changes.

## Validation Floor

1. Run focused tests for the touched Feishu UI path:
   - `go test ./internal/adapter/feishu ./internal/app/daemon ./internal/core/control ./internal/core/orchestrator`
2. Do not commit if projector and gateway disagree on a callback payload that current cards can emit.
3. Do not commit if a stale card can still perform a product state mutation without an intentional, documented compatibility reason.
4. Do not commit if the new UI flow leaves the user with a clickable old card but no clear next action.

## Scope Reminder

This skill is the Feishu-card counterpart to the core remote surface guardrail. It covers Feishu UI session, payload, and freshness behavior, not the full attach/follow/queue product state graph by itself.
