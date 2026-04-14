# Claude Provider 协议映射与实现矩阵

> Type: `inprogress`
> Updated: `2026-04-14`
> Summary: 基于 Claude Code 源码确认 `stream-json` 为首选接入面，并补齐 canonical 协议到 Claude provider 的实现映射矩阵。

## 1. 文档定位

这份文档是 **实现文档**，不是产品 PRD。

它回答的是下面这些实现问题：

1. 当前仓库的 canonical command / event / state 概念，分别该如何映射到 Claude Code 当前真实支持的程序化交互面。
2. 哪些能力可以直接对齐，哪些必须加 adapter，哪些在 v1 必须显式降级。
3. 如果按本文实现，一个工程师应该如何拆 provider 边界、命令编码、事件归一化和 session catalog。

它与 [claude-normal-mode-poc-design.md](./claude-normal-mode-poc-design.md) 的关系是：

- `claude-normal-mode-poc-design.md` 负责产品边界、状态机和 rollout 范围。
- 本文负责协议映射、adapter 设计和实现落点。

## 2. 调研基线与证据来源

本次判断基于两部分 source of truth：

1. 本仓库当前 canonical 协议与产品约束
   - [docs/general/relay-protocol-spec.md](../general/relay-protocol-spec.md)
   - [docs/general/codex-mcp-app-server-protocol.md](../general/codex-mcp-app-server-protocol.md)
   - [docs/inprogress/claude-normal-mode-poc-design.md](./claude-normal-mode-poc-design.md)
2. 本机 Claude Code 源码快照
   - 根路径：`/tmp/tmp.blIwUuAvOL/claude-code-sourcemap/`

本次最关键的源码证据如下：

- `package/package.json`
  - 包形态是 CLI-first，`bin.claude = cli.js`，不是一个常规稳定 JS SDK 包导出面。
- `restored-src/src/main.tsx`
  - 暴露 `--print`
  - 暴露 `--input-format stream-json`
  - 暴露 `--output-format stream-json`
  - 暴露 `--resume`
  - 暴露 `--session-id`
  - 暴露 `--permission-mode`
  - 暴露 `--model`
  - 暴露 `--effort`
  - 暴露 `--replay-user-messages`
- `restored-src/src/bridge/sessionRunner.ts`
  - bridge 真正也是通过 `claude --print --input-format stream-json --output-format stream-json` 起子进程。
- `restored-src/src/entrypoints/sdk/controlSchemas.ts`
  - 定义了 `control_request` / `control_response` / `control_cancel_request`
  - 定义了 `interrupt` / `can_use_tool` / `set_permission_mode` / `set_model` / `set_max_thinking_tokens` / `get_context_usage` / `elicitation` 等 request subtype
  - 定义了 stdin / stdout 的 aggregate schema
- `restored-src/src/entrypoints/sdk/coreSchemas.ts`
  - 定义了 `assistant` / `result` / `system.init` / `stream_event` / `session_state_changed` / `tool_progress` / `task_*` / `tool_use_summary` / `api_retry` / `prompt_suggestion` / `post_turn_summary`
  - 定义了 `SDKSessionInfo`
- `restored-src/src/utils/streamJsonStdoutGuard.ts`
  - 明确把 `stream-json` 当成 NDJSON line stream 保护。
- `restored-src/src/entrypoints/mcp.ts`
  - Claude 自身可以作为 MCP server。
- `restored-src/src/entrypoints/cli.tsx`
  - `remote-control` 是单独入口，属于 bridge / remote-control 语义，不是本地 provider 首选接入面。
- 对源码树执行 `rg "acp|ACP|Agent Communication Protocol"` 没有找到可作为 Claude 原生会话协议的 ACP 入口。

## 3. 先给结论

### 3.1 v1 首选接入面

Claude provider 的首选接入面应当是：

- `claude --print`
- `--input-format stream-json`
- `--output-format stream-json`
- 长生命周期子进程
- stdin/stdout NDJSON

也就是：

- **主会话面用 `stream-json`**
- **不是 ACP**
- **不是 remote-control**
- **MCP 只作为工具层能力，不是主 turn/session 协议**

### 3.2 对本仓库最重要的协议判断

1. Claude 当前确实有稳定的 `session_id` 概念，可以承接我们当前的 `thread/session` 语义。
2. `turn.interrupt` 有原生映射。
3. `request.respond` 可以覆盖 permission request 和 elicitation，但必须按 subtype 分开做。
4. `threads.refresh` 和 `thread.history.read` 不能假装来自 live session wire；它们更像本地 session catalog / transcript 读取能力。
5. `turn.steer` 没有与当前 canonical 语义等价的原生入口，v1 应显式不支持。
6. Claude 的消息流比 Codex 更“混合”：
   - assistant 内容
   - partial stream
   - tool progress
   - task progress
   - retry/status
   - post-turn summary
   都在同一条 SDK stream 上，需要 adapter 做归一化。

### 3.3 实施建议

不要继续沿着 “ACP adapter” 方向做 v1。

应该实现的是一个 `claude provider adapter`，内部拆成两条面：

1. **Live turn transport**
   - `claude --print --input-format stream-json --output-format stream-json`
   - 负责 `prompt.send` / `turn.interrupt` / `request.respond`
2. **Session catalog / history plane**
   - 基于 Claude 本地 session transcript / metadata
   - 负责 `threads.refresh` / `thread.history.read`

## 4. 推荐 Provider 边界

```text
surface / orchestrator / daemon
          |
          | canonical command / canonical event
          v
   claude provider adapter
      |            |
      |            +-- local session catalog / transcript reader
      |
      +-- long-lived `claude -p` child
              stdin:  stream-json NDJSON
              stdout: stream-json NDJSON
```

职责切分：

- daemon / orchestrator
  - 继续持有 canonical command / event / queue / surface 状态机
- claude provider adapter
  - 命令编码
  - stdout 解析
  - control request / response 关联
  - session catalog / history 读取
  - provider-specific config translation
- Claude child process
  - 只负责 Claude 自己的会话执行、tool runtime、permission / elicitation 请求

## 5. 核心映射原则

### 5.1 `session_id` 是 Claude 的 native 会话主键

- Claude native conversation id 使用 `session_id`
- 本仓库持久层不能只把它当全局唯一键
- 推荐持久键：
  - `backend + instance_id + session_id`

实现建议：

- canonical event 里的 `threadId` 可以继续承载 Claude `session_id`
- 但任何跨 backend 持久状态都必须带 backend / instance 维度

### 5.2 新建与恢复是两条不同路径

新会话：

- 使用 `--session-id <new uuid>` 是最稳妥方案
- 这样 wrapper 自己先拥有 canonical `threadId`

恢复会话：

- 使用 `--resume <session-id>`
- 不要和 `--session-id` 混用
- 源码里只有 `--fork-session` 场景才允许 `--resume` 同时配 `--session-id`

因此：

- `new` 和 `resume` 必须在 adapter 内显式区分
- 不能把 “恢复老会话” 偷偷实现成 “开新会话再塞历史”

### 5.3 `threads.refresh` / `thread.history.read` 不属于 live turn wire

Claude source 里虽然有 `listSessionsImpl` / `getSessionInfo` / `loadConversationForResume` / session storage，但这不是 `stream-json` stdout 上自然冒出来的 live RPC。

所以在本仓库里应该建模成：

- `session_catalog` capability
- `thread_history_read` capability

而不是假装它们和 `prompt.send` 一样是同一条 wire 命令。

### 5.4 `turn.completed` 不能只靠 `session_state_changed`

推荐规则：

- `session_state_changed=running`
  - turn 进入运行态
- `result`
  - turn 终态 payload 的 authoritative 来源
- `session_state_changed=idle`
  - turn 已彻底不再运行的 authoritative barrier

也就是：

- `result` 负责“结果是什么”
- `idle` 负责“真的结束了没有”

### 5.5 `request.respond` 必须 subtype-aware

Claude 控制面里至少存在两类需要我们回写的 request：

1. `can_use_tool`
2. `elicitation`

两者不能共用一个拍平的处理器：

- `can_use_tool` 回写的是 permission decision
- `elicitation` 回写的是 `{ action, content }`

### 5.6 `turn.steer` 不要假装支持

Claude current source 没有和我们现有 `turn.steer` 等价的原生语义：

- 我们的 `turn.steer` 是对正在运行 turn 的 queue item 升级
- 不是单纯的“打断后再发一句话”

因此 v1 要显式：

- `capability.turn_steer = false`
- command 直接拒绝

不要在 v1 偷偷做：

- `interrupt + enqueue new prompt`

因为这会制造“看起来支持，语义其实变了”的假兼容。

## 6. Canonical Command / Session 映射矩阵

| Canonical 概念 | Claude 对应面 | 支持级别 | Adapter 实现规则 |
| --- | --- | --- | --- |
| provider hello / capability 上报 | 无单独 Claude wire 消息；由我们 wrapper 自己上报 | 直接支持 | hello 时明确上报 `backend=claude`，并建议至少带上 `prompt_send=true`、`turn_interrupt=true`、`request_respond=true`、`session_catalog=true`、`thread_history_read=true`、`resume_by_thread_id=true`、`turn_steer=false`、`supports_vscode_mode=false`。 |
| `prompt.send` 新会话 | `claude -p --session-id <uuid> --input-format stream-json --output-format stream-json` | 直接支持 | wrapper 生成新的 `session_id`，在目标 `cwd` 启动 child；首条用户输入通过 stdin `SDKUserMessage` 发送；线程 identity 从第一刻就固定。 |
| `prompt.send` 续写当前 live 会话 | 已存在的长生命周期 child stdin | 直接支持 | 若该 `session_id` 已有存活 child，直接向 stdin 发送新的 `SDKUserMessage`，不要重起子进程。 |
| `prompt.send` 恢复会话 | `claude -p --resume <session-id> --input-format stream-json --output-format stream-json` | 直接支持 | 仅在 child 不存在但目标是历史会话时使用；先 `--resume` 恢复 transcript，再发用户消息。不要把恢复行为实现成“新会话 + 手工 replay”。 |
| `prompt.inputs[].text` | `SDKUserMessage` | 直接支持 | 文本输入是 v1 主路径。按 queue item 顺序合并进用户消息内容。 |
| `prompt.inputs[].local_image` / `remote_image` | `SDKUserMessage.message` 理论上可承载多模态内容，但当前 schema 快照把 message 作为 opaque placeholder | 部分支持 | 源码快照没有给出稳定 typed image block schema。v1 应先按 text-only 落地；图片作为后续 capability，在运行时确认 accepted content block 格式后再开。 |
| `turn.interrupt` | `control_request{subtype=interrupt}` | 直接支持 | command 下发时向 child stdin 发送 control request；wrapper 立即 `command_ack.accepted=true`，真正 turn 收敛仍等 `result` / `session_state_changed`。 |
| `request.respond` for approval | 对上一条 `can_use_tool` 的 `control_response` | 部分支持 | `accept -> {behavior: allow}`；`decline -> {behavior: deny, message}`；`acceptForSession` 需要把 Claude 提供的 `permission_suggestions` 转成 `updatedPermissions` 或 provider 内的“持久放行”响应，不能只发裸 allow。 |
| `request.respond` for elicitation | 对上一条 `elicitation` 的 `control_response` | 直接支持 | 映射到 `{action: accept|decline|cancel, content}`；表单/URL elicitation 走这条线；不要复用 permission response 结构。 |
| `threads.refresh` | 本地 session catalog，不来自 live `stream-json` | 适配支持 | 基于 Claude 本地 session storage / transcript metadata 读取，生成 `threads.snapshot`。这条能力应与 live child 解耦。 |
| `thread.history.read` | 本地 transcript read，不来自 live `stream-json` | 适配支持 | 基于本地 transcript 读取完整 session，归一化成 canonical `thread.history.read` event。 |
| `turn.steer` | 无原生等价面 | v1 不支持 | 显式 `command_rejected/problem`。不要 fake 成 `interrupt + prompt.send`。 |
| `process.exit` | 无 Claude wire 命令，对应 child process lifecycle | 直接支持 | 关闭 stdin、取消待处理 request、发送终止信号并等待 child 退出。 |
| `thread/session fork` | `--fork-session` | 预留 | 不是 v1 必需能力，但 Claude 源码已有入口，可为后续“从历史会话分叉”预留。 |

## 7. Canonical Event 映射矩阵

| Canonical event / 概念 | Claude 来源 | 支持级别 | Adapter 归一化规则 |
| --- | --- | --- | --- |
| `config.observed` | `system.init`，以及后续 `status` / control ack | 直接支持 | 首次 `system.init` 时记录当前 `model`、`permissionMode`、`tools`、`mcp_servers`、`cwd`、`skills/plugins`；必要时增量更新。 |
| `thread.discovered` / `threads.snapshot` | `SDKSessionInfo` 或 session catalog reader | 适配支持 | 来自本地 session catalog，不来自 live turn stream。`summary`、`lastModified`、`cwd`、`gitBranch` 可以直出到 ThreadRecord。 |
| `turn.started` | `system.session_state_changed{state=running}` | 直接支持 | 对每次从非 running 进入 running 的边界发一次 `turn.started`。 |
| `item.started` / `item.delta` / `item.completed` for assistant text | `stream_event` + `assistant` | 直接支持 | `stream_event` 作为同一 assistant item 的 delta；`assistant` 到来时封 item 完成。不要把 partial chunk 各自当独立 item。 |
| `item.started` / `item.delta` / `item.completed` for tool/task activity | `tool_progress`、`task_started`、`task_progress`、`task_notification`、`tool_use_summary` | 适配支持 | 建立 provider-specific synthetic item type。`task_started` 开始，`task_progress` / `tool_progress` 追加，`task_notification` 或最终 summary 完成。 |
| `request.started` | `control_request{subtype=can_use_tool}`、`control_request{subtype=elicitation}` | 直接支持 | `request_id` 直接作为 native request key 保留。metadata 中要区分 `requestKind=permission` 和 `requestKind=elicitation`。 |
| `request.resolved` | 我们自己成功写回 `control_response`，或收到 `control_cancel_request` | 适配支持 | Claude stdout 不一定单独给出“request 完成”事件，因此 wrapper 可以在成功回写后本地合成 `request.resolved`。若收到 `control_cancel_request`，也要本地结束对应 request。 |
| `turn.completed` | `result{subtype=success|error}`，辅以 `session_state_changed=idle` | 直接支持 | `result` 承担终态 payload、usage、cost、error；`idle` 作为 turn 真正结束的 barrier。两者都要消费。 |
| `thread.token_usage.updated` | `result.usage` + `result.modelUsage` | 直接支持 | 每个 turn 结束时更新 thread 累计 usage；`last` 来自当前 result；`total` 由 thread 级聚合维护。 |
| 上下文使用量 | `control_request{subtype=get_context_usage}` response | 直接支持 | 这不是自然事件流，应作为按需查询或状态卡片补充数据，不要塞进 tick。 |
| `local.interaction.observed` | `system.local_command_output` | 直接支持 | 只作为本地 slash / command 的文本观测，不要冒充 assistant 正文。 |
| 非致命系统提示 | `system.api_retry`、`status`、`auth_status`、`hook_*` | 适配支持 | 这些更适合归类为 local/system notice。`api_retry` 明确是“会重试”的过程信号，不能上升成 fatal runtime error。 |
| `turn.plan.updated` | 无精确等价消息 | v1 不支持 | Claude 当前 source 没有与 Codex `turn/plan/updated` 等价的稳定会话事件。不要从 `task_progress.summary` 硬推导计划快照。 |
| `prompt suggestion` | `prompt_suggestion` | 预留 | 当前 canonical 协议没有对应一等事件，v1 可先忽略或做 local notice。 |
| `post turn summary` | `post_turn_summary` | 预留 | 与 current canonical event 不一一对应。v1 可先忽略或仅内部记录。 |

## 8. 配置与能力映射矩阵

### 8.1 会话配置

| 本仓库概念 | Claude 对应能力 | 支持级别 | 实现建议 |
| --- | --- | --- | --- |
| `model` override | CLI `--model`，control `set_model` | 直接支持 | 新 session 用 CLI 参数，live session 可用 control request 动态切换。 |
| `reasoningEffort` | CLI `--effort`，control `set_max_thinking_tokens`，CLI `--thinking` | 部分支持 | 语义不是 1:1。建议 v1 把 `reasoningEffort` 当 provider-specific config：新 session 用 `--effort`，live 变更先不承诺完全等价；不要把 Codex 的 effort 枚举生搬成 Claude wire。 |
| `accessMode` | `permissionMode` / `set_permission_mode` | 部分支持 | 这两套概念相近但不等价。必须单独维护一张 provider translation table，不能共享同一枚举。 |
| 上下文窗口统计 | `get_context_usage` | 直接支持 | 按需查询，不做 tick 周期轮询。 |

### 8.2 工具与扩展能力

| 能力语义 | Claude 对应能力 | 支持级别 | 说明 |
| --- | --- | --- | --- |
| 本地 bash / shell | 内建工具 + `can_use_tool` + `tool_progress` | 直接支持 | 在我们的协议里表现为 request + tool progress + assistant/result，不是单独 command。 |
| 读文件 / 写文件 / 编辑文件 | 内建工具 + `can_use_tool` + `tool_progress` | 直接支持 | 同上。 |
| Web search / Web fetch | 内建工具，是否可用取决于当前配置 / policy | 直接支持 | 对我们来说仍表现为 tool request / progress，不需要新协议面。 |
| MCP tools | MCP client config + tool runtime | 直接支持 | MCP 是 Claude 的工具层能力，不是主 turn/session 协议。 |
| MCP server 状态 | `mcp_status` | 直接支持 | 适合做 status / debug 面板，不影响主 turn 流。 |
| MCP server 动态管理 | `mcp_set_servers` / `mcp_toggle` / `mcp_reconnect` / `reload_plugins` / `mcp_message` | 预留 | Claude source 已有控制面，但 v1 不是首批必须实现。 |
| 子任务 / sub-agent 进度 | `task_started` / `task_progress` / `task_notification` | 直接支持 | 比 Codex 现有 item 流更丰富，adapter 应保留，不要丢。 |
| prompt 建议 | `prompt_suggestion` | 预留 | v1 可以不投影。 |
| post-turn summary | `post_turn_summary` | 预留 | v1 可以只记录日志。 |
| VS Code follow / focus | 无 normal-mode 等价面 | v1 不支持 | 明确不做。 |

## 9. Session Catalog / History 的实现约束

这是 Claude provider 最容易被误判的一块。

### 9.1 不要把 catalog 能力绑到 live child

`threads.refresh` / `thread.history.read` 应满足：

- 在没有 live child 时也可工作
- 不依赖当前 attached turn 正在运行
- 不阻塞 `prompt.send`

### 9.2 推荐读取来源

从源码快照看，Claude 已有：

- `listSessionsImpl`
- `getSessionInfo`
- `loadConversationForResume`
- session JSONL transcript storage

对本仓库来说，最现实的 v1 方案是：

1. 把 session catalog 视为 adapter-private 本地读取能力。
2. 读取 Claude session storage，生成本仓库的 `threads.snapshot` / `thread.history.read`。
3. 明确在 capability 上标注这是 `session_catalog`，不是 live wire command。

### 9.3 `requires_cwd_for_resume`

从 upstream 实现看：

- Claude 可以用 `session_id` 直接恢复历史 session
- 但本产品仍应保留 `requires_cwd_for_resume=true` 这个约束

原因不是 wire 限制，而是产品约束：

1. workspace root 决定工具可见目录和后续文件操作语义
2. 同一个 session 在错误 cwd 下恢复，容易让“会话恢复成功”但工具行为偏移
3. 我们当前产品本来就是 workspace-aware，而不是全局 session browser

因此推荐规则：

- 允许 catalog 中跨项目看见 session
- 但真正 attach / resume 前，仍要校验并切回正确 cwd / workspace

## 10. `request.respond` 的细化规则

### 10.1 Permission request

Claude 的 `can_use_tool` 请求里会带：

- `tool_name`
- `input`
- `permission_suggestions`
- `tool_use_id`
- 可选 `blocked_path`
- 可选 `decision_reason`

因此在 canonical `request.started` 上建议保留：

- 原始 `tool_use_id`
- `tool_name`
- `permission_suggestions`

这样 `acceptForSession` 才不会在 adapter 内丢语义。

### 10.2 Elicitation

Claude `elicitation` 已经是独立 request subtype，至少有：

- `message`
- `mode=form|url`
- `url`
- `elicitation_id`
- `requested_schema`

因此 canonical request metadata 里建议明确暴露：

- `requestKind=elicitation`
- `mode`
- `requestedSchema`
- `url`
- `elicitationId`

### 10.3 不要把产品层 follow-up 混进 provider

本仓库 Feishu 侧已有：

- `captureFeedback`
- “拒绝后再补一句话”

这些仍然应该停留在 server / projector / surface 产品层：

- provider 只负责把 `decline` / `accept` / `cancel` 回写给 Claude
- 不应在 provider 内直接实现额外 follow-up prompt 逻辑

## 11. v1 明确不做 / 明确降级

1. `turn.steer`
   - 显式不支持
2. Claude normal mode 的 VS Code follow / focus
   - 显式不支持
3. `turn.plan.updated`
   - 不强行伪造
4. 图片输入
   - 在确认稳定 wire shape 之前先不承诺
5. MCP server 动态管理面板
   - 不是首批必需
6. 任何 “看起来像 Codex，所以沿用 Codex 字段名直接透传” 的做法
   - 尤其是 `reasoningEffort` 和 `accessMode`

## 12. 最小实现路径

按工程落地，建议按下面顺序推进：

1. provider / capability 基座
   - instance 记录 backend=claude
   - command dispatch 前做 capability 检查
2. live session runner
   - 长生命周期 `claude -p`
   - stdin/stdout NDJSON
   - request correlation
3. `prompt.send`
   - 新会话
   - 续写 live session
   - 恢复历史会话
4. `turn.interrupt`
   - control request 打通
5. `request.respond`
   - 先做 `can_use_tool`
   - 再做 `elicitation`
6. stdout event normalizer
   - `system.init`
   - `session_state_changed`
   - `stream_event`
   - `assistant`
   - `result`
   - `tool_progress` / `task_*`
7. session catalog / history
   - `threads.refresh`
   - `thread.history.read`
8. provider-specific config translation
   - `model`
   - `reasoning`
   - `permission mode`
9. 明确 reject path
   - `turn.steer`
   - `supports_vscode_mode=false`

## 13. 对旧 ACP 草案的关系

[acp-claude-integration-design.md](../draft/acp-claude-integration-design.md) 仍保留为历史草案，但它对 “Claude native integration surface” 的判断不应继续作为实现依据。

当前 source-backed 结论应以本文为准：

- v1 不是 ACP adapter
- v1 是 `claude --print --input-format stream-json --output-format stream-json` provider adapter
