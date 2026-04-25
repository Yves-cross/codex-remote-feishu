# Web Setup / Web Admin 改造方案与技术调研（V2 合并版）

> Type: `draft`
> Updated: `2026-04-25`
> Summary: 合并 Web Setup 与 Web Admin 的产品改造要求，记录当前确认的独立页面 Mock，给出后端实现路径、接口改造方案、无效流程清理清单；本版仅文档调研，不包含正式产品代码改动。

## 1. 目标与范围

本文件用于一次性收口两件事：

1. Web Setup 流程重构（自动检查、强阻断与非阻断边界、机器人侧主交互）
2. Web Admin 信息架构重排（机器人管理为主、系统集成、存储管理）

本文件同时补充技术调研：

1. 后端如何实现各功能
2. 现有后端接口哪些可复用、哪些要新增/改造/下线
3. 代码里哪些旧流程和旧路径应彻底清理

约束：本轮仅文档，不进行任何代码修改。

## 1.1 当前 Mock 产物

当前已产出两份独立单页 Mock，供后续正式实现时直接对照：

1. `docs/draft/web-setup-user-mock.html`
2. `docs/draft/web-admin-user-mock.html`

说明：

1. 两个 Mock 均为单 HTML，自带内嵌样式与交互脚本。
2. Mock 仅用于固化最终用户可见结构、文案边界与页面交互，不代表最终实现必须沿用其前端技术组织方式。
3. 旧的合并型单页 Mock 已废弃，不再作为实现参考。

## 2. 产品方案（确认版）

## 2.1 Web Setup（新流程）

### A. 环境检查（强阻断）

1. 进入 Setup 即自动检查。
2. 检查通过自动跳到“飞书连接”，不需要手点下一步。
3. 检查失败停留并展示失败项。

### B. 飞书连接（强阻断）

1. 保留两条路径：扫码创建、手动 App ID + App Secret。
2. 两条路径都必须做真实连接验证。
3. 验证成功后机器人主动发两条提示：连接成功 + 基础设置通过（回 Web Setup 继续）。
4. 验证失败阻断前进。
5. 扫码路径在选中后立即显示二维码，不保留“开始扫码”之类的二次触发按钮。
6. 扫码状态由页面自动轮询，当前 Mock 约定轮询间隔为 2 秒。
7. 扫码成功后页面自动跳转到后续目标步骤，不要求用户再点确认。

### C. 权限检查（强阻断）

1. 到页面即自动检查权限，不再让用户手动点“检查”。
2. 权限完整则自动跳转“事件订阅”页。
3. 权限缺失则停留页面，展示：
   - 缺失权限列表
   - 飞书后台入口链接
   - 按缺失项生成的一次性 JSON（可一键复制）
4. 此页不允许跳过。

### D. 事件订阅（非阻断，机器人主交互）

1. 机器人主动发测试提示，指导用户回复特定验证词。
2. 收到并验证成功则机器人回执“订阅通过，请回 Web Setup 继续”。
3. 失败时给出后台订阅入口和排障提示。
4. Web 页允许用户“我已完成，继续”，不记录硬门禁状态。

### E. 回调配置（非阻断，机器人主交互）

1. 机器人主动发送带回调按钮卡片。
2. 回调成功后机器人回执“回调完成，请回 Web Setup 下一步”。
3. 失败时给出后台入口和排障提示。
4. Web 页同样允许直接继续，不设置硬门禁。

### F. 菜单确认（非阻断）

只保留菜单配置入口链接 + 简短说明，然后继续。

### G. 自动启动 / VS Code / 完成

后半段沿用当前能力。

补充约定：

1. Setup 页标题使用“产品名 + 安装程序”。
2. Setup 顶部不保留“首次设置完成后即可开始使用”这类解释性引导文案。
3. 自动启动页先做能力检测：
   - 不支持则自动跳过
   - 支持时默认视为“当前未启用”，由用户决定是否开启
4. 自动启动后增加 VS Code 集成确认页：
   - 只保留“确认集成 / 先不使用”两条路径
   - 不再暴露多种集成方式选择
5. Setup 末尾保留完成欢迎页，并提供进入管理页面的入口。

## 2.2 Web Admin（新结构）

顶层只保留 3 个模块，顺序固定：

1. 机器人管理（主模块）
2. 系统集成（自动运行设置、VS Code 集成）
3. 存储管理（预览文件、图片暂存、日志清理）

机器人管理交互规则：

1. 左侧列表显示“全部机器人 + 新增机器人”。
2. 列表仅在失败状态打标；成功状态保持安静。
3. 点“新增机器人”时，右侧显示接入流程。
4. 点现有机器人时，右侧显示“机器人状态页”。
5. 状态页必须包含机器人基础信息。
6. 状态页只强调失败，不展示“成功流水”。
7. 状态页仅保留 2 个按钮：
   - 测试事件订阅
   - 测试回调
8. 不保留通用“测试”按钮。
9. 负责人信息如果后端无法稳定提供，则前端不展示该字段。
10. 状态页保留删除机器人入口，删除动作需二次确认模态框。

明确移除：

1. 管理实例模块（前端入口移除）
2. 运行概览模块
3. 技术详情模块对最终用户可见入口
4. “设置到某阶段”的记忆式引导（`wizard.*At` 驱动 UI 流）

## 3. 现状调研（代码映射）

已确认当前实现与目标存在以下偏差：

1. Setup 与 Admin 都深度依赖 `wizard.*At`（`scopes/events/callbacks/menus/published`）驱动流程推进，属于“手工确认式状态记忆”。
2. Setup 的 `capability` 阶段仍是“勾选 + PATCH /wizard + publish-check”模型，非真实自动校验。
3. Admin 当前信息架构仍包含 `运行概览/工作实例/技术详情`，与新结构不一致。
4. Admin 虽然实例创建/删除已返回 `410 Gone`，但实例列表与相关页面仍在。
5. 存储侧已有“预览目录”和“图片暂存”管理接口，但没有“日志目录统计 + 清理”接口。
6. 权限检查目前缺少“缺失权限结果 + 一次性 JSON 直接返回”的独立 API；`scopes-json` 当前只回默认清单，不是实时差异结果。
7. “测试事件订阅/测试回调”在 Admin 尚无专用 API；仅有连接验证（`/verify`）与重连（`/reconnect`）。

## 4. 后端实现方案（按功能）

## 4.1 Setup 自动检查与自动跳转

1. 复用现有 `/api/setup/runtime-requirements/detect`。
2. 前端策略改为“进入即调用 + ready 自动前进 + fail 停留”。
3. 后端无需新增核心能力，主要是前端门禁逻辑改造。

## 4.2 飞书连接验证 + 成功通知

1. 继续复用现有：
   - 新建/更新应用
   - `/verify` 连接验证
2. 增加“验证成功后机器人主动通知”的后端动作封装：
   - 统一发文本提示（成功建立连接、基础设置已通过）
3. 通知动作应由服务端发起，避免前端拼接文案直出。

## 4.3 权限检查（真实校验）

1. 以 `manifest` 作为“应有权限”基线。
2. 使用 `feishu.ListAppScopes(...)` 获取当前应用权限实际状态。
3. 计算 `missingScopes` 后返回：
   - 缺失权限列表
   - 可直接导入的最小 JSON（建议仅包含缺失项）
   - 飞书后台跳转链接
4. Setup 页与 Admin 状态页都消费同一份权限检查结果。

## 4.4 事件订阅测试

1. 测试入口已明确绑定机器人（Setup 当前步骤机器人，Admin 当前详情机器人），不做任何“目标选择”流程。
2. 服务端直接通过飞书 API 发测试提示，目标绑定到当前机器人测试流程，不依赖“活跃会话选择”。
3. 服务端保存一条短时效测试上下文（建议 5-10 分钟）：
   - 绑定 `gateway_id`
   - 绑定测试类型 `event_subscription`
   - 绑定固定验证词
4. 用户输入固定验证词后，只对该 `gateway_id` 的测试上下文进行命中与通过判定。
5. 命中成功后机器人回执“订阅通过，请回 Web Setup/Web Admin 继续”；Web 侧保持非阻断。

## 4.5 回调测试

1. 测试入口同样已明确绑定机器人，不做任何“目标选择”流程。
2. 服务端直接通过飞书 API 发送测试卡片（带回调按钮）。
3. 服务端保存短时效测试上下文（建议 5-10 分钟）：
   - 绑定 `gateway_id`
   - 绑定测试类型 `callback`
4. 回调命中后只对对应 `gateway_id` 标记成功，并由机器人回执成功消息。
5. Web 侧保持非阻断。

## 4.6 Web Admin 机器人状态页

1. 机器人列表接口继续基于 `/api/admin/feishu/apps`。
2. 增加每个机器人“权限检查摘要字段”（仅失败时用于列表标记）。
3. 详情页读取：
   - 基础信息
   - 连接状态
   - 权限失败项（若有）
4. 仅暴露两个测试动作入口（事件订阅、回调）。

## 4.7 存储管理（预览 + 图片暂存 + 日志）

1. 预览文件管理继续复用现有 preview drive status/cleanup。
2. 图片暂存管理继续保留并复用现有 image staging status/cleanup，用于展示总体占用与清理旧文件。
3. 新增日志管理：
   - 统计“全部日志”总大小与文件数（单一模块展示，不拆多块）
   - 一键清理一天前日志（默认阈值 24h）
4. 日志路径来源使用现有 runtime paths（`LogsDir`），清理范围覆盖 relayd/wrapper/raw 等该目录下日志文件。

## 4.8 系统集成（自动运行设置、VS Code 集成）

1. 自动运行设置复用 `/api/admin/autostart/*`。
2. VS Code 集成复用 `/api/admin/vscode/*`。
3. 仅做页面结构调整与文案裁剪，不新增后端复杂逻辑。

## 4.9 飞书 API 调用约束（强制）

本改造所有“测试消息/测试卡片/权限读取”相关飞书调用统一遵循以下约束：

1. 不允许在 daemon 或 admin handler 里直调 Lark SDK client。
2. 必须统一走 `internal/adapter/feishu` 的 broker 调用链：
   - SDK API：`DoSDK(...)`
   - HTTP API：`DoHTTP(...)`
3. 必须带 `CallSpec`，显式声明：
   - `API`
   - `Class`
   - `Priority`
   - `Retry`
   - `Permission`
4. 测试消息/测试卡片默认使用：
   - `Class=CallClassIMSend`
   - `Priority=CallPriorityInteractive`
   - `Retry=RetryRateLimitOnly`
   - `Permission=PermissionCooldownOnly`
5. 权限检查（scope list）默认使用：
   - `Class=CallClassMetaHTTP`
   - `Priority=CallPriorityBackground`
   - `Retry=RetrySafe`
   - `Permission=PermissionFailFast`

说明：当前网关发送链路本身已通过 `LiveGateway -> gateway_im_calls.go -> DoSDK(...)` 接入统一限流/权限冷却机制，本方案新增测试能力必须复用该链路。

## 4.10 测试上下文模型（事件订阅/回调）

为保证“机器人已明确、无需用户选择”且可稳定回执，新增服务端测试上下文运行时：

1. 建议新增运行时结构（daemon 内存）：
   - `gateway_id`
   - `test_kind` (`event_subscription` / `callback`)
   - `fixed_phrase`（仅事件订阅）
   - `created_at`
   - `expires_at`
   - `status`（`pending` / `passed` / `expired`）
2. 上下文有效期建议 10 分钟（可配置，默认 10 分钟）。
3. 同一机器人同一种测试在有效期内重复触发时，采用“覆盖旧上下文并重发测试提示”策略（幂等化）。
4. 网关入站事件命中规则：
   - 仅匹配同 `gateway_id`
   - 事件订阅仅匹配固定验证词
   - 回调仅匹配对应测试按钮 action
5. 通过后立即回执成功消息并标记 `passed`；过期由后台惰性清理。
6. 测试上下文只作为临时状态，不写入持久化配置。

## 4.11 测试目标路由（无用户选择）

“不做目标选择”定义为：前端不暴露选择 UI，后端按机器人上下文自动确定投递目标。

1. Setup 场景：当前步骤已绑定唯一机器人（`app/gateway`）。
2. Admin 场景：机器人详情页已绑定唯一机器人（`app/gateway`）。
3. 投递目标解析由服务端内部完成，不出现任何用户可见的目标选择步骤。
4. 该目标解析结果仅用于服务端调用飞书 API 发测试提示，不改变用户交互流程。

## 5. 接口改造清单（建议）

## 5.1 复用保留

1. `/api/setup/runtime-requirements/detect`
2. `/api/setup/feishu/apps`、`/api/setup/feishu/apps/{id}/verify`
3. `/api/admin/feishu/apps`、`/api/admin/feishu/apps/{id}/verify`、`/reconnect`
4. `/api/admin/autostart/detect`、`/apply`
5. `/api/admin/vscode/detect`、`/apply`、`/reinstall-shim`
6. `/api/admin/storage/preview-drive/{id}`、`/cleanup`
7. `/api/admin/storage/image-staging`、`/cleanup`

## 5.2 新增接口

1. `GET /api/setup/feishu/apps/{id}/permission-check`
2. `GET /api/admin/feishu/apps/{id}/permission-check`
3. `POST /api/setup/feishu/apps/{id}/test-events`
4. `POST /api/setup/feishu/apps/{id}/test-callback`
5. `POST /api/admin/feishu/apps/{id}/test-events`
6. `POST /api/admin/feishu/apps/{id}/test-callback`
7. `GET /api/admin/storage/logs`
8. `POST /api/admin/storage/logs/cleanup`

建议响应字段（权限检查）：

1. `ready`
2. `missingScopes[]`
3. `grantJSON`
4. `consoleURL`
5. `lastCheckedAt`

## 5.3 新增接口契约（可开工粒度）

### `POST /api/setup/feishu/apps/{id}/test-events`

1. 请求体（可为空）：
   - `phrase`（可选；为空则使用系统固定词）
2. 响应体：
   - `gatewayId`
   - `startedAt`
   - `expiresAt`
   - `phrase`
   - `message`（已发出测试提示）

### `POST /api/setup/feishu/apps/{id}/test-callback`

1. 请求体：空
2. 响应体：
   - `gatewayId`
   - `startedAt`
   - `expiresAt`
   - `message`（已发出测试卡片）

### `POST /api/admin/feishu/apps/{id}/test-events`

契约同 setup 版本。

### `POST /api/admin/feishu/apps/{id}/test-callback`

契约同 setup 版本。

### `GET /api/admin/storage/logs`

1. 响应体：
   - `rootDir`
   - `fileCount`
   - `totalBytes`
   - `latestFileAt`

### `POST /api/admin/storage/logs/cleanup`

1. 请求体：
   - `olderThanHours`（默认 24）
2. 响应体：
   - `rootDir`
   - `olderThanHours`
   - `deletedFiles`
   - `deletedBytes`
   - `remainingFileCount`
   - `remainingBytes`

## 5.4 下线或降级为内部接口

1. `PATCH /api/setup/feishu/apps/{id}/wizard`
2. `PATCH /api/admin/feishu/apps/{id}/wizard`
3. `POST /api/setup/feishu/apps/{id}/publish-check`
4. `POST /api/admin/feishu/apps/{id}/publish-check`
5. `GET /api/setup/feishu/apps/{id}/scopes-json`
6. `GET /api/admin/feishu/apps/{id}/scopes-json`
7. `GET /api/admin/instances`
8. `POST /api/admin/instances`
9. `DELETE /api/admin/instances/{id}`

备注：`7-9` 可先前端下线，再做后端正式移除，避免一次性大改造成回归风险。

## 5.5 后端落点文件（建议）

1. 路由注册：
   - `internal/app/daemon/admin.go`
2. 机器人接口与 handler：
   - `internal/app/daemon/admin_feishu.go`
   - `internal/app/daemon/admin_feishu_handlers.go`
3. 测试上下文运行时：
   - `internal/app/daemon/runtime_state.go`
   - 新增 `internal/app/daemon/admin_feishu_test_runtime.go`（建议）
4. 飞书投递实现：
   - 复用 `internal/adapter/feishu/controller_gateway.go`
   - 复用 `internal/adapter/feishu/gateway_im_calls.go` 的统一 broker 调用链
5. 日志管理实现：
   - 新增 `internal/app/daemon/admin_storage_logs.go`（建议）
   - 复用 `internal/runtime/paths.go` 的 `LogsDir`

## 6. 代码与流程清理清单

## 6.1 前端清理（web）

1. `AdminRoute.tsx` 中移除：
   - `AdminOverviewPanel`
   - `AdminInstancesPanel`
   - `AdminTechnicalPanel`
   - 与 `wizard` 相关的“确认能力”动作链
2. `AdminPanels.tsx` 中删除上述面板与其 helper 依赖。
3. `SetupRoute.tsx` 与 `SetupStepContent.tsx` 中删除：
   - 勾选确认状态（permissions/events/callbacks/menus）
   - publish-check 步骤
   - 基于 `wizard.publishedAt` 的 capability 门禁
4. 重写导航与模块结构，按“机器人管理/系统集成/存储管理”组织。

## 6.2 后端清理（daemon）

1. 清理 `wizard` 更新链路：
   - `handleFeishuAppWizardUpdate`
   - `updateFeishuAppWizard`
   - `applyWizardToggle` 及相关手工确认流程
2. 清理 publish-check 驱动的流程门禁逻辑。
3. 下线实例管理 API 与对应测试。
4. 保留 image-staging 管理 API，统一归入 `storage` 大类，不散落在其他页面。
5. 新增日志清理服务后，统一落到 `storage` 大类，不再散落在技术页。
6. 旧配置中的 `wizard.*At` 历史状态字段与相关依赖逻辑直接清理，不保留兼容写回路径。
7. 新增测试接口时禁止旁路直调 SDK，统一接入 `FeishuCallBroker`。

## 6.3 测试与文档清理

1. 更新前端测试：
   - `AdminRoute.test.tsx`
   - Setup 流程相关测试
2. 更新后端测试：
   - `admin_feishu_test.go`（移除 wizard/publish-check 断言）
   - `admin_instances_test.go`（按下线策略处理）
   - 新增权限检查、事件测试、回调测试、日志清理测试
3. 更新产品文档，避免“同一功能两条路径”长期并存。

## 7. 本版已定规则（替代待拍板）

1. 事件订阅测试与回调测试均不做“目标选择”；测试机器人由当前页面上下文唯一确定。
2. 测试触发由服务端直接调用飞书 API 发出，并保存短时效测试上下文。
3. 事件订阅测试使用固定验证词，不使用 token 输入方案。
4. 日志清理按“全部日志一块管理”执行，范围覆盖 `LogsDir` 下全部日志文件类型。
5. 旧 `wizard.*At` 配置与流程依赖直接移除，不做反向降级兼容。

## 8. 实施建议（顺序）

1. 先改 Setup 流程门禁与权限检查接口（影响 onboarding 成功率最大）。
2. 再改 Admin 信息架构和机器人状态页（收口最终用户面）。
3. 落地事件订阅/回调测试接口（统一 broker 调用链 + 测试上下文）。
4. 最后做存储管理整合与日志清理接口落地。
5. 收尾阶段统一删旧接口与旧测试，避免双路径长期共存。
