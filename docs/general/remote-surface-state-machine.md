# Remote Surface 核心状态机

> Type: `general`
> Updated: `2026-04-06`
> Summary: 记录当前已实现的 remote surface 状态机、全局仲裁规则、命令矩阵、watchdog 与死状态审计结论，作为后续改动的提交前复审基线。

## 1. 文档定位

这份文档描述的是**当前代码已经实现**的 remote surface 状态机，不是历史问题列表，也不是未来方案草稿。

它承担两个职责：

1. 作为当前 remote surface 行为的长期 source of truth。
2. 作为后续状态机相关改动在提交前必须回看的 guardrail。

审计基线覆盖：

1. [internal/core/orchestrator/service.go](../../internal/core/orchestrator/service.go)
2. [internal/core/orchestrator/service_test.go](../../internal/core/orchestrator/service_test.go)
3. [internal/core/state/types.go](../../internal/core/state/types.go)
4. [internal/core/control/types.go](../../internal/core/control/types.go)
5. [internal/adapter/feishu/gateway.go](../../internal/adapter/feishu/gateway.go)
6. [internal/adapter/feishu/projector.go](../../internal/adapter/feishu/projector.go)
7. [internal/app/daemon/app_test.go](../../internal/app/daemon/app_test.go)

## 2. 审计前提

### 2.1 `threadID` 当前就是 relay 全局仲裁键

当前 thread claim 是 `map[string]*threadClaimRecord`，key 只有 `threadID`。

这依赖下面这个前提，而且现在就是产品前提：

1. 同一台机器上，`threadID` 在单个 `relayd` 仲裁域内全局唯一。
2. 同一台机器上只运行一个 `relayd`。

这个假设必须保留在文档里，避免以后误改成“按 instance 局部唯一”。

### 2.2 surface 是分 gateway/chat 的，但 claim 是 relay 全局的

surface 本身仍按 `gatewayID + chat/user` 区分，不同飞书 app 会形成不同 surface。

但 `instanceClaims` 和 `threadClaims` 都在同一个 orchestrator 里仲裁，所以：

1. 不同飞书 app 之间会竞争同一套 instance/thread 资源。
2. instance attach 互斥、thread attach 互斥都是**跨 app 的全局规则**。

## 3. 当前状态机的四层结构

surface 不是单一枚举，而是四层正交状态叠加。

### 3.1 路由主状态

| 代号 | 条件 | 用户语义 |
| --- | --- | --- |
| `R0 Detached` | `AttachedInstanceID == ""` | 当前没有接管任何实例 |
| `R1 AttachedUnbound` | `AttachedInstanceID != ""`，`RouteMode=unbound`，`SelectedThreadID == ""` | 已接管实例，但当前没有可发送 thread |
| `R2 AttachedPinned` | `AttachedInstanceID != ""`，`RouteMode=pinned`，`SelectedThreadID != ""`，且持有 thread claim | 当前输入固定发到该 thread |
| `R3 FollowWaiting` | `AttachedInstanceID != ""`，`RouteMode=follow_local`，`SelectedThreadID == ""` | 已进入 follow，但当前没有可接管 thread |
| `R4 FollowBound` | `AttachedInstanceID != ""`，`RouteMode=follow_local`，`SelectedThreadID != ""`，且持有 thread claim | 已跟随到一个 thread |

### 3.2 执行状态

| 代号 | 条件 | 含义 |
| --- | --- | --- |
| `E0 Idle` | `DispatchMode=normal`，无 active，无 queued | 空闲 |
| `E1 Queued` | `QueuedQueueItemIDs` 非空，`ActiveQueueItemID == ""` | 有待派发远端输入 |
| `E2 Dispatching` | `ActiveQueueItemID` 指向 `dispatching` | prompt 已发给 wrapper，turn 尚未建立 |
| `E3 Running` | `ActiveQueueItemID` 指向 `running` | turn 已进入执行 |
| `E4 PausedForLocal` | `DispatchMode=paused_for_local` | 观察到本地 VS Code 活动，远端暂停出队 |
| `E5 HandoffWait` | `DispatchMode=handoff_wait` | 本地刚结束，等待短窗口后恢复远端队列 |
| `E6 Abandoning` | `Abandoning=true` | surface 已放弃接管，等待已有 turn 收尾后最终 detach |

### 3.3 输入门禁状态

| 代号 | 条件 | 作用 |
| --- | --- | --- |
| `G0 None` | 无附加门禁 | 普通输入按主路由走 |
| `G1 PendingHeadlessStarting` | `PendingHeadless.Status=starting` | headless 仍在启动 |
| `G2 PendingHeadlessSelecting` | `PendingHeadless.Status=selecting` | headless 已 attach，但等待用户选恢复 thread |
| `G3 PendingRequest` | `PendingRequests` 非空 | 普通文本/图片会被确认卡片门禁挡住 |
| `G4 RequestCapture` | `ActiveRequestCapture != nil` | 下一条普通文本会被当成拒绝反馈 |
| `G5 AbandoningGate` | `Abandoning=true` | 只有 `/status` 继续正常，其余动作被挡 |

### 3.4 草稿状态

| 代号 | 条件 | 含义 |
| --- | --- | --- |
| `D0 NoDraft` | 无 staged image，无 queued draft | 没有待绑定输入 |
| `D1 StagedImages` | `StagedImages` 中存在 `ImageStaged` | 图片已上传，但尚未冻结到 queue item |
| `D2 QueuedDrafts` | `QueuedQueueItemIDs` 非空 | 已冻结 thread/cwd/override，等待派发 |

关键区别：

1. `D2` 已冻结路由。
2. `D1` 还没有冻结路由，所以 route change 时必须显式处理。

## 4. 当前已实现的不变量

### 4.1 instance attach 全局互斥

当前 `attachInstance()` 与 `attachHeadlessInstance()` 都走 `instanceClaims`。

结果：

1. 一个 instance 同时只能被一个飞书 surface attach。
2. 第二个 surface attach 同一 instance 会直接收到 `instance_busy`。
3. 不会进入“instance attach 成功但 thread attach 失败且用户不知道下一步”的半 attach 状态。

例外只剩一种显式可恢复状态：

1. attach instance 成功。
2. 默认 thread 当前拿不到 claim 或没有默认 thread。
3. surface 进入 `R1 AttachedUnbound`。
4. 服务端会主动发 thread 选择卡片。

这不是死状态，因为用户仍然只有一条明确下一步：`/use` 或点 thread 卡片。

### 4.2 thread attach 全局互斥

当前 `threadClaims` 仍按 `threadID` 做全局仲裁。

结果：

1. 一个 thread 同时只能被一个飞书 surface 占有。
2. `/use` 命中已被他人占用的 thread 时：
   1. 对方 idle 才会弹强踢确认。
   2. 对方 queued/running 会直接拒绝。

### 4.3 `PendingHeadless` 现在是 dominant gate

只要 `PendingHeadless != nil`：

1. 允许：`/status`、`/killinstance`、`resume_headless_thread`、消息撤回、reaction。
2. 其余 surface action 全部在 `ApplySurfaceAction()` 顶层被拦截。

这意味着：

1. `starting` 时不能旁路 attach/use/follow。
2. `selecting` 时也不能通过 `/use`、`/follow`、普通文本去改路由。
3. headless 选择的唯一正常逃生口是“恢复某个 thread”或“/killinstance”。

### 4.4 选择卡片不再是服务端持久状态

当前服务端已经不再保存 `SelectionPrompt` 状态，也不再把“纯数字文本”解释成选择。

当前行为：

1. attach/use/headless resume/kick confirm 都改成**直达动作**。
2. Feishu 卡片按钮直接携带：
   1. `attach_instance`
   2. `use_thread`
   3. `resume_headless_thread`
   4. `kick_thread_confirm`
   5. `kick_thread_cancel`
3. 旧 `prompt_select` 只保留兼容解析，服务端统一返回 `selection_expired`。
4. `"1"`、`"2"` 这类纯数字文本现在就是普通文本。

注意：

1. `control.UIEventSelectionPrompt` 仍然存在。
2. 它现在只是“卡片渲染 helper”，不是 surface 状态机的一部分。

### 4.5 route change 时会丢弃未冻结图片草稿

当前这些动作都会调用 `discardStagedImagesForRouteChange()`：

1. `/use`
2. `/follow`
3. follow 自动跟随到新 thread
4. thread claim 被别人强踢后退回 waiting/unbound
5. 其他所有通过 `bindSurfaceToThreadMode()` 发生的 route change

行为固定为：

1. 所有 `ImageStaged` 被标记为 `discarded`。
2. 发 `thumbs down` / `queue off` 反应。
3. 发 `staged_images_discarded_on_route_change` notice。

当前实现不允许未冻结图片静默串到新 thread。

### 4.6 `PausedForLocal` 和 `Abandoning` 都有 watchdog

当前 `Tick()` 已经提供两类恢复：

1. `paused_for_local` 超时后：
   1. 自动回到 `normal`
   2. 发 `local_activity_watchdog_resumed`
   3. 继续 `dispatchNext`
2. `abandoning` 超时后：
   1. 强制 `finalizeDetachedSurface`
   2. 发 `detach_timeout_forced`

所以这两个状态不再依赖单一异步事件才能退出。

## 5. 主要状态迁移

### 5.1 attach / list / use / follow

```text
R0 Detached
  -- /attach(instance，可拿默认 thread) --> R2 AttachedPinned
  -- /attach(instance，默认 thread 不可拿或不存在) --> R1 AttachedUnbound
  -- /newinstance --> R0 + G1 PendingHeadlessStarting

R1 AttachedUnbound
  -- /use(thread) --> R2 AttachedPinned
  -- /follow --> R4 FollowBound 或 R3 FollowWaiting
  -- /detach --> R0 Detached
  -- instance offline --> R0 Detached

R2 AttachedPinned
  -- /use(other thread) --> R2 AttachedPinned
  -- /follow --> R4 FollowBound 或 R3 FollowWaiting
  -- selected thread claim 丢失 --> R1 AttachedUnbound 或 R3 FollowWaiting
  -- /detach(no live work) --> R0 Detached
  -- /detach(live work) --> E6 Abandoning -> R0 Detached
  -- instance offline --> R0 Detached

R3 FollowWaiting
  -- VS Code focus 到可接管 thread --> R4 FollowBound
  -- /use(thread) --> R2 AttachedPinned
  -- /detach --> R0 Detached
  -- instance offline --> R0 Detached

R4 FollowBound
  -- VS Code focus 切到其他可接管 thread --> R4 FollowBound
  -- VS Code focus 消失或被别人占用 --> R3 FollowWaiting
  -- /use(thread) --> R2 AttachedPinned
  -- /detach(no live work) --> R0 Detached
  -- /detach(live work) --> E6 Abandoning -> R0 Detached
  -- instance offline --> R0 Detached
```

补充说明：

1. `/follow` 从 `pinned` 切到 `follow_local` 时，即使 thread 没变，也会发 route-mode 变更投影。
2. `/list` 的 instance 卡片会把已被他人 attach 的 instance 标成 disabled。

### 5.2 远端队列生命周期

```text
E0 Idle
  -- enqueue --> E1 Queued
  -- dispatchNext --> E2 Dispatching

E2 Dispatching
  -- turn.started(remote_surface) --> E3 Running
  -- command rejected / dispatch failure --> E0 Idle

E3 Running
  -- turn.completed(remote_surface) --> E0 Idle
```

补充说明：

1. `pendingRemote` 先按 instance 保留“哪个 queue item 正在等 turn”。
2. turn 建立后再提升到 `activeRemote`。
3. 这样 turn 归属不靠“同 thread 的下一个事件”猜，而靠显式 binding。

### 5.3 本地 VS Code 仲裁

```text
E0/E1
  -- local.interaction.observed 或 local turn.started --> E4 PausedForLocal

E4 PausedForLocal
  -- local turn.completed 且 queue 空 --> E0 Idle
  -- local turn.completed 且 queue 非空 --> E5 HandoffWait
  -- Tick 超时 --> E0 Idle 并自动恢复 dispatch

E5 HandoffWait
  -- Tick 到期 --> E0 Idle 并继续 dispatchNext
```

### 5.4 headless 生命周期

```text
G0 None
  -- /newinstance --> G1 PendingHeadlessStarting

G1 PendingHeadlessStarting
  -- instance connected --> attach headless instance + G2 PendingHeadlessSelecting
  -- /killinstance --> G0 None
  -- Tick timeout --> kill headless + clear pending + detach if needed

G2 PendingHeadlessSelecting
  -- resume_headless_thread(thread) --> R2 AttachedPinned + G0 None
  -- /killinstance --> G0 None
  -- 无 recoverable threads --> kill headless + R0 Detached + G0 None
```

### 5.5 detach / abandoning 生命周期

```text
/detach
  -- 无 live work --> finalizeDetachedSurface --> R0 Detached
  -- 有 live work --> discard drafts + E6 Abandoning

E6 Abandoning
  -- 当前 turn 收尾 / disconnect / queue fail --> R0 Detached
  -- Tick 超时 --> force finalize --> R0 Detached
```

detach 时额外保证：

1. 未发送 queue item 会被丢弃。
2. staged image 会被丢弃。
3. request prompt / request capture 会被清空。

## 6. 命令矩阵

### 6.1 基础路由态

| 命令 | `R0 Detached` | `R1 AttachedUnbound` | `R2 AttachedPinned` | `R3 FollowWaiting` | `R4 FollowBound` |
| --- | --- | --- | --- | --- | --- |
| `/list` | 允许 | 允许 | 允许 | 允许 | 允许 |
| `/newinstance` | 允许 | 拒绝 | 拒绝 | 拒绝 | 拒绝 |
| `/killinstance` | 仅 pending headless 时有效 | 仅 headless attach/launch 时有效 | 同左 | 同左 | 同左 |
| `/use` `/useall` | 拒绝 | 允许 | 允许 | 允许 | 允许 |
| `/follow` | 拒绝 | 允许 | 允许 | 允许 | 允许 |
| 文本 | 拒绝 | 拒绝 | 允许 | 拒绝 | 允许 |
| 图片 | 拒绝 | 拒绝 | 允许 | 拒绝 | 允许 |
| 请求按钮 | 拒绝 | 拒绝 | 允许 | 拒绝 | 允许 |
| `/stop` | 通常无效果 | 通常无效果 | 允许 | 允许 | 允许 |
| `/status` | 允许 | 允许 | 允许 | 允许 | 允许 |
| `/detach` | 允许但通常只提示已 detached | 允许 | 允许 | 允许 | 允许 |
| `/model` `/reasoning` `/access` | 拒绝 | 允许 | 允许 | 允许 | 允许 |

### 6.2 覆盖门禁

| 覆盖状态 | 当前行为 |
| --- | --- |
| `G1/G2 PendingHeadless` | 只允许 `/status`、`/killinstance`、`resume_headless_thread`、revoke/reaction；其余动作统一被 headless notice 挡住 |
| `G3 PendingRequest` | 普通文本、图片被挡；用户必须先处理请求卡片 |
| `G4 RequestCapture` | 下一条文本优先被当成反馈；数字文本不再被 selection 抢走 |
| `E6 Abandoning` | 只允许 `/status`；再次 `/detach` 只回 `detach_pending`；其余动作统一拒绝 |

## 7. UI 动作协议

当前 Feishu 卡片动作与服务端 action 对应关系如下：

| 卡片动作 | 服务端 action | 说明 |
| --- | --- | --- |
| `attach_instance` | `ActionAttachInstance` | 直达 attach |
| `use_thread` | `ActionUseThread` | 直达 thread 切换 |
| `resume_headless_thread` | `ActionResumeHeadless` | 直达 headless 恢复 |
| `kick_thread_confirm` | `ActionConfirmKickThread` | 强踢前再次校验实时状态 |
| `kick_thread_cancel` | `ActionCancelKickThread` | 仅回 notice |
| `prompt_select` | `ActionSelectPrompt` | 旧兼容入口，统一回 `selection_expired` |

这层协议意味着：

1. 卡片可以过期。
2. 但过期卡片不会再篡改 surface 状态，只会给明确反馈。

## 8. 当前死状态审计结论

这轮按当前实现重新审计后，以下几类 bug-grade 半死状态已经收口：

1. **instance 半 attach**：已修复。第二个 surface attach 同一 instance 会直接失败。
2. **数字文本误切换 thread**：已修复。数字文本现在是普通消息。
3. **headless 选择期还能旁路 `/use` `/follow`**：已修复。`PendingHeadless` 现为顶层 gate。
4. **staged image 跟着 route change 串 thread**：已修复。route change 会显式丢图并告知用户。
5. **`PausedForLocal` 永久卡住**：已修复。现在有 watchdog。
6. **`Abandoning` 永久锁死**：已修复。现在有 watchdog。
7. **`/follow` 切模式但 thread 不变时 UI 不知道 route mode 已变**：已修复。现在会补发 route-mode selection 投影。

当前审计范围内，未再发现“attach/use 成功后用户没有任何可恢复下一步”的 bug-grade 状态。

## 9. 与 `/new` 的关系

本轮文档只覆盖**现有 remote surface 状态机**。

`/new` 仍然是后续 feature，不在本文实现范围里。当前仍需记住一个前提：

1. 现有状态机已经能稳定处理“已有 thread 的 attach/use/follow/headless 恢复”。
2. 但“空 thread 建立后如何稳定归属回 surface”仍是 `/new` 方案必须单独处理的问题。

所以后续做 `/new` 时，应该在这份文档之上新增状态，而不是回退当前这些 guardrail。

## 10. 提交前复审基线

凡是修改以下任一行为，都应该在提交前回看本文并同步更新：

1. instance/thread attach/detach
2. `/use`、`/follow`
3. `PendingHeadless`
4. queue/dispatch/turn ownership
5. staged image / draft 命运
6. request capture / request prompt
7. Feishu 卡片动作协议
8. watchdog 与恢复路径

最低复审问题：

1. 有没有新增“用户表面上看已 attach 或已选 thread，但文本/图片仍无路可走”的状态。
2. 有没有新增只靠异步事件才能退出、但没有 watchdog 或手动逃生口的 blocked state。
3. 有没有让未冻结草稿在 route change 时静默改投目标。
4. 有没有把 UI helper 状态重新变回服务端持久 modal state。

## 11. 待讨论取舍

当前无。
