#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

failed=0

legacy_kind_switch_matches="$(rg -n 'case[[:space:]]+control\.UIEvent' internal --glob '!internal/core/eventcontractcompat/**' --glob '!**/*_test.go' || true)"
if [[ -n "${legacy_kind_switch_matches}" ]]; then
  echo "Forbidden legacy UIEventKind switch cases outside eventcontractcompat:" >&2
  printf '%s\n' "${legacy_kind_switch_matches}" >&2
  failed=1
fi

legacy_projector_entry_matches="$(rg -n '\bprojector\.Project\(' internal --glob '!**/*_test.go' || true)"
if [[ -n "${legacy_projector_entry_matches}" ]]; then
  echo "Forbidden legacy projector entrypoint usage in production code (use ProjectEvent instead):" >&2
  printf '%s\n' "${legacy_projector_entry_matches}" >&2
  failed=1
fi

followup_heuristic_matches="$(rg -n 'Notice[[:space:]]*!=[[:space:]]*nil|ThreadSelection[[:space:]]*!=[[:space:]]*nil' \
  internal/app/daemon/app_ingress.go \
  internal/core/orchestrator/service_followup_filter.go \
  internal/core/orchestrator/service_path_picker_contract.go \
  internal/core/orchestrator/service_target_picker_owner_card.go \
  --glob '!**/*_test.go' || true)"
if [[ -n "${followup_heuristic_matches}" ]]; then
  echo "Forbidden followup payload heuristics in followup filters (use eventcontract semantics):" >&2
  printf '%s\n' "${followup_heuristic_matches}" >&2
  failed=1
fi

resolver_matches="$(rg -n 'resolveGatewayTarget\(' internal/adapter/feishu --glob '!**/*_test.go' || true)"
resolver_disallowed="$(printf '%s\n' "${resolver_matches}" | \
  grep -vE '^internal/adapter/feishu/(controller_gateway|controller_preview|controller_target_resolver)\.go:' || true)"
if [[ -n "${resolver_disallowed}" ]]; then
  echo "Forbidden resolveGatewayTarget call sites outside controller resolver boundary:" >&2
  printf '%s\n' "${resolver_disallowed}" >&2
  failed=1
fi

if [[ "${failed}" -ne 0 ]]; then
  exit 1
fi
