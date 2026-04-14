# File Length Split Audit 2026-04

> Type: `draft`
> Updated: `2026-04-14`
> Summary: 首轮审计了疑似由单文件行数门限触发的拆分提交，给出合理性分级与分阶段整改清单。

## 背景

仓库已经启用 `scripts/check/go-file-length.sh`（业务文件 `<=1000` 行，测试文件 `<=2000` 行）。
近期多次观察到“为了过门限而拆分”但边界粗糙的情况，本审计用于回看存量拆分质量。

## 扫描范围与方法

- 范围：
  - 重点扫描 `2026-04-06` 到 `2026-04-14` 期间，commit message 含 `split`/`oversized`/`refactor` 且涉及大文件拆分的改动。
  - 同步检查当前代码中接近门限的 Go 文件（业务文件 `>=900` 行）和拆分后回涨情况。
- 方法：
  - 对候选 commit 做 `git show --name-status --stat`。
  - 对拆分前后文件行数做对比（`git show <commit>^:<file> | wc -l` vs 当前 `wc -l`）。
  - 依据“职责边界是否清晰、是否只做最小挪动、是否快速回涨到门限附近”进行分级。

## 阶段 1 结果（提交级）

### A. 拆分质量整体较好

1. `fc09e88 refactor: split orchestrator service by concern`
   - `service.go` 从 `4749` 行降到 `458` 行，并拆出 queue/request/routing/snapshot/surface/helpers。
   - 按职责面切分，方向正确。
2. `1893784 refactor: split oversized relay service files`
   - 继续把 orchestrator 的 routing/snapshot/surface/helpers 细化为子主题文件。
   - 例如 `service_routing.go` 从 `1213` 行拆成 claims/blockers/lifecycle/request_state 多文件。
3. `2be6be1 Split markdown preview pipeline modules`
   - `markdown_preview.go` `1322 -> 175`，拆成 admin/helpers/lark_api/rewrite/state，语义清晰。
4. `2cf4b6b refactor: split oversized relay files`
   - `projector_selection.go` `1022 -> 523`，并新增 `projector_selection_use_thread.go` `507`。
   - 按 use-thread 语义拆分，边界明确。
5. `03d629e refactor: split admin management flow and refresh docs`
   - `admin_feishu.go` `1036 -> 73`，拆出 handlers/helpers/runtime。
   - 后端拆分方向合理。
6. `1be6c76 refactor(web): split setup route modules`
   - `SetupRoute.tsx` `1065 -> 502`，拆出 step content/helpers/types，路径清晰。
7. `f68cf7f test: split oversized local request menu tests`
   - 从 `service_local_request_test.go` 中抽出 menu 场景为独立测试文件，主题一致。

### B. 存在明显“为过线而最小挪动”的拆分

1. `82af9a3 refactor: split oversized control and orchestrator test files`（其中一部分）
   - `feishu_commands.go` `1011 -> 991`。
   - 新增 `feishu_commands_helpers.go` 仅 `23` 行，只迁移了两个函数（`commandOption`、`buildMenuVerboseText`）。
   - 判定：这是典型 line-count-first，未按命令模型/catalog/parser 进行结构化拆分。

### C. 拆分当时合理，但子文件已经明显回涨

1. `dc683f3 Refactor codex translator into smaller units`
   - `translator_helpers.go` `463 -> 901`（当前）。
   - `translator_requests_test.go` `295 -> 590`（当前）。
   - `translator_observe_server.go` `530 -> 722`（当前）。
2. `1893784` 相关子文件回涨：
   - `service_snapshot_runtime.go` `867 -> 920`。
   - `service_surface_actions.go` `672 -> 749`。
3. `fc09e88` 后回涨：
   - `service_queue.go` `520 -> 910`。
4. `796fd34 test: split oversized relay test files` 后回涨：
   - `projector_snapshot_final_test.go` `743 -> 1153`。
   - `service_local_request_test.go` `1566 -> 1897`。
   - `service_config_prompt_test.go` `1949 -> 1995`（非常接近测试门限）。

## 当前高风险近门限文件（Go 业务文件 >=900 行）

1. `internal/core/control/feishu_commands.go` `993`
2. `internal/core/orchestrator/service_thread_global.go` `979`
3. `internal/app/daemon/app_upgrade.go` `966`
4. `internal/adapter/feishu/gateway_inbound.go` `932`
5. `internal/core/orchestrator/service_snapshot_runtime.go` `920`
6. `internal/adapter/feishu/projector.go` `914`
7. `internal/core/orchestrator/service_queue.go` `910`
8. `internal/adapter/codex/translator_helpers.go` `901`

## 分阶段整改建议

### Phase 2（先修“拆分质量明显不达标”的点）

1. `feishu_commands.go` 做结构化拆分（优先级最高）
   - 目标边界：`command_specs`、`catalog_build`、`command_parse`、`runtime_overrides`。
   - 禁止再用“只挪几个 helper”方式过线。

### Phase 3（处理近门限核心业务文件）

1. `service_queue.go`
   - 拆为：`queue_enqueue_dispatch`、`queue_turn_lifecycle`、`queue_dynamic_tool_projection`。
2. `service_snapshot_runtime.go`
   - 拆为：`surface_materialize`、`command_dispatch_ack`、`instance_lifecycle`、`snapshot_query`。
3. `translator_helpers.go`
   - 拆为：`json_lookup`、`item_extract`、`request_extract`、`structured_content_extract`。
4. `app_upgrade.go`
   - 拆为：`upgrade_command_parse`、`upgrade_state_store`、`upgrade_check_scheduler`、`upgrade_catalog_render`。
5. `gateway_inbound.go`
   - 拆为：`message_parse_text_post`、`quoted_inputs`、`merge_forward_summary`、`media_fetch_download`。

### Phase 4（测试文件防回涨）

1. 把 `>=1800` 行测试文件继续按“单一场景簇”切分，避免再次贴近 `2000` 门限。
2. 目标优先级：
   - `service_config_prompt_test.go`
   - `service_test.go`
   - `service_local_request_test.go`
   - `service_thread_selection_test.go`
   - `service_headless_thread_test.go`

## 判定结论（阶段 1）

- 明确不合理拆分：`1` 处（`feishu_commands.go` 在 `82af9a3` 中的拆法）。
- 结构合理但需要二次治理（回涨或接近门限）：`多处`（见 Phase 3/4）。
- 建议下一步：直接按 Phase 2 开始整改，不必等待全量讨论后再动手。
