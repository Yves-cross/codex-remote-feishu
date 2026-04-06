# Remote Surface 核心状态机

> Type: `general`
> Updated: `2026-04-06`
> Summary: 将 remote surface 状态机整理为长期核心参考文档，记录当前实现、命令矩阵、死/半死状态与提交前复审基线。

## 1. 文档定位

这份文档是 remote surface 的**核心状态机参考文档**。

它承担两个职责：

1. 作为当前实现行为的长期 source of truth。
2. 作为后续状态机相关改动在提交前必须回看的复审基线。

当前文档内容建立在这次重新审计之上，目标是先把下面四件事讲清楚：

1. 当前代码里到底有哪些稳定状态、临时状态、输入门禁状态。
2. 各类命令在这些状态下到底能不能执行，谁负责把状态带走。
3. 哪些问题其实是实现 bug，哪些才是真正需要产品取舍。
4. 如果先不做 `/new`，当前状态机应该先怎么加固。

审计范围：

1. [internal/core/orchestrator/service.go](../../internal/core/orchestrator/service.go)
2. [internal/core/state/types.go](../../internal/core/state/types.go)
3. [internal/core/control/types.go](../../internal/core/control/types.go)
4. [internal/adapter/feishu/gateway.go](../../internal/adapter/feishu/gateway.go)
5. [internal/adapter/feishu/projector.go](../../internal/adapter/feishu/projector.go)
6. [internal/adapter/codex/translator.go](../../internal/adapter/codex/translator.go)
7. [internal/app/wrapper/app.go](../../internal/app/wrapper/app.go)
8. [internal/core/orchestrator/service_test.go](../../internal/core/orchestrator/service_test.go)

## 2. 审计前提

### 2.1 `threadID` 当前就是 relay 全局仲裁键

当前代码里的 thread claim 是 `map[string]*threadClaimRecord`，key 只有 `threadID`，没有 `(instanceID, threadID)` 复合键。

这意味着当前实现已经内建了下面这个前提：

1. 同一台机器上，`threadID` 在 `relayd` 仲裁域内全局唯一。
2. 同一台机器上只运行一个 `relayd`。

这和当前产品前提一致，所以这个点本轮不需要改，但必须记在状态机文档里，避免以后误把它改成“按实例局部唯一”。

### 2.2 surface 是 per-gateway、per-chat 的；claim 是 relay 全局的

surface 自己已经按 `gatewayID + chat/user` 分离。

但当前的 instance/thread 仲裁都发生在同一个 orchestrator 里，所以：

1. 一个飞书 App 的 surface 和另一个飞书 App 的 surface，本质上在竞争同一套全局资源。
2. 这次状态机加固必须按“全局 claim”来想，不能按“单 app 局部状态”来想。

## 3. 当前状态机的四层结构

当前 surface 不是单一枚举状态，而是四层正交状态叠加。

### 3.1 路由主状态

这是用户最直观看到的“当前消息会往哪去”。

| 代号 | 条件 | 用户语义 |
| --- | --- | --- |
| `R0 Detached` | `AttachedInstanceID == ""` | 还没有接管任何实例 |
| `R1 AttachedUnbound` | `AttachedInstanceID != ""`，`RouteMode=unbound`，`SelectedThreadID == ""` | 已接管实例，但还没有可发消息的 thread |
| `R2 AttachedPinned` | `AttachedInstanceID != ""`，`RouteMode=pinned`，`SelectedThreadID != ""`，且持有 thread claim | 当前输入固定发到该 thread |
| `R3 FollowWaiting` | `AttachedInstanceID != ""`，`RouteMode=follow_local`，`SelectedThreadID == ""` | 已进入 follow，但当前没有可接管 thread |
| `R4 FollowBound` | `AttachedInstanceID != ""`，`RouteMode=follow_local`，`SelectedThreadID != ""`，且持有 thread claim | 已跟随到一个 thread |

当前代码没有 `instance claim`，所以这张表只描述 thread 级路由，不代表 instance 真的是互斥 attach。

### 3.2 执行状态

这是“有没有活、当前活执行到哪”的状态。

| 代号 | 条件 | 含义 |
| --- | --- | --- |
| `E0 Idle` | `DispatchMode=normal`，无 active，无 queued | 空闲 |
| `E1 Queued` | `QueuedQueueItemIDs` 非空，`ActiveQueueItemID == ""` | 有待派发的远端输入 |
| `E2 Dispatching` | `ActiveQueueItemID` 指向 `dispatching` | 已发给 wrapper，turn 还没建立 |
| `E3 Running` | `ActiveQueueItemID` 指向 `running` | turn 已进入执行 |
| `E4 PausedForLocal` | `DispatchMode=paused_for_local` | 观察到本地 VS Code 活动，远端暂停出队 |
| `E5 HandoffWait` | `DispatchMode=handoff_wait` | 本地刚结束，等待短窗口后恢复远端队列 |
| `E6 Abandoning` | `Abandoning=true` | surface 已放弃接管，等待已有 turn 收尾后最终 detach |

### 3.3 输入/交互门禁状态

这是“表面上看还在原路由态，但某些输入会被抢走或拦住”的状态。

| 代号 | 条件 | 作用 |
| --- | --- | --- |
| `G0 None` | 无附加门禁 | 普通输入按主路由走 |
| `G1 PendingHeadlessStarting` | `PendingHeadless.Status=starting` | headless 仍在启动 |
| `G2 PendingHeadlessSelecting` | `PendingHeadless.Status=selecting` | headless 已 attach，但等待用户选恢复 thread |
| `G3 SelectionPrompt` | `SelectionPrompt != nil` | 数字文本会被解释成选项，不是普通消息 |
| `G4 PendingRequest` | `PendingRequests` 非空 | 普通文本/图片会被确认卡片门禁挡住 |
| `G5 RequestCapture` | `ActiveRequestCapture != nil` | 下一条普通文本会被当成拒绝反馈 |

### 3.4 草稿状态

这层当前没有被当成“正式状态”建模，但它实际上会影响路由切换正确性。

| 代号 | 条件 | 作用 |
| --- | --- | --- |
| `D0 NoDraft` | 无 staged image，无 queued draft | 没有待绑定输入 |
| `D1 StagedImages` | `StagedImages` 中存在 `ImageStaged` | 图片已经上传，但还没有绑定到文本 turn |
| `D2 QueuedDrafts` | `QueuedQueueItemIDs` 非空 | 已冻结路由，等待派发 |

这里最关键的区别是：

1. `QueuedDrafts` 已经冻结了 thread/cwd。
2. `StagedImages` 还没有冻结路由。

这正是后面“图片会跟着 `/use` 串 thread”的根因。

## 4. 当前有效状态图

### 4.1 路由主状态图

```text
R0 Detached
  -- /attach(instance) --> R2 AttachedPinned
  -- /attach(instance, 默认 thread 不可拿) --> R1 AttachedUnbound
  -- /newinstance --> R0 + G1 PendingHeadlessStarting

R1 AttachedUnbound
  -- /use(thread) --> R2 AttachedPinned
  -- /follow --> R4 FollowBound 或 R3 FollowWaiting
  -- /detach --> R0 Detached
  -- instance offline --> R0 Detached

R2 AttachedPinned
  -- /use(other thread) --> R2 AttachedPinned
  -- /follow --> R4 FollowBound 或 R3 FollowWaiting
  -- selected thread 丢失 / claim 丢失 --> R1 AttachedUnbound
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

### 4.2 执行状态图

```text
E0 Idle
  -- enqueue --> E1 Queued
  -- local activity observed --> E4 PausedForLocal

E1 Queued
  -- dispatchNext --> E2 Dispatching
  -- local activity observed --> E4 PausedForLocal
  -- /stop 或 /detach --> discard drafts / detach

E2 Dispatching
  -- turn.started(remote) --> E3 Running
  -- dispatch failure / command rejected --> E0 Idle

E3 Running
  -- turn.completed(remote) --> E0 Idle
  -- /detach --> E6 Abandoning

E4 PausedForLocal
  -- local turn.completed 且 queue 空 --> E0 Idle
  -- local turn.completed 且 queue 非空 --> E5 HandoffWait
  -- 没有 completion 事件 --> 当前实现会一直卡住

E5 HandoffWait
  -- Tick 到期 --> E0 Idle 并继续 dispatchNext

E6 Abandoning
  -- 当前 turn 完成 / active queue fail / instance disconnect --> R0 Detached
  -- 没有收尾事件 --> 当前实现会一直卡住
```

### 4.3 Headless 叠加状态图

```text
G0 None
  -- /newinstance --> G1 PendingHeadlessStarting

G1 PendingHeadlessStarting
  -- instance connected --> G2 PendingHeadlessSelecting
  -- /killinstance --> G0 None
  -- Tick timeout --> 当前实现发送 kill，但状态清理不完整

G2 PendingHeadlessSelecting
  -- headless resume button --> R2 AttachedPinned + G0 None
  -- /killinstance --> G0 None
  -- 无 recoverable threads --> R0 Detached + G0 None
  -- 当前实现还允许 /use /follow /attach 等旁路动作
```

## 5. 命令矩阵

### 5.1 基础命令矩阵

这里只看路由主状态，不叠加 `PendingHeadless`、`Abandoning` 等覆盖门禁。

| 命令 | `R0 Detached` | `R1 AttachedUnbound` | `R2 AttachedPinned` | `R3 FollowWaiting` | `R4 FollowBound` |
| --- | --- | --- | --- | --- | --- |
| `/list` | 允许 | 允许 | 允许 | 允许 | 允许 |
| `/newinstance` | 允许 | 拒绝 | 拒绝 | 拒绝 | 拒绝 |
| `/killinstance` | 通常无效 | 仅 headless attach/launch 时有效 | 同左 | 同左 | 同左 |
| `/threads` `/use` `/sessions` | 拒绝 | 允许 | 允许 | 允许 | 允许 |
| `/useall` | 拒绝 | 允许 | 允许 | 允许 | 允许 |
| `/follow` | 拒绝 | 允许 | 允许 | 允许 | 允许 |
| 文本 | 拒绝 | 拒绝 | 允许 | 拒绝 | 允许 |
| 图片 | 拒绝 | 拒绝 | 允许 | 拒绝 | 允许 |
| 请求按钮 | 拒绝 | 拒绝 | 允许 | 拒绝 | 允许 |
| `/stop` | 允许但通常无效果 | 允许但通常无效果 | 允许 | 允许 | 允许 |
| `/status` | 允许 | 允许 | 允许 | 允许 | 允许 |
| `/detach` | 允许但通常只提示已 detached | 允许 | 允许 | 允许 | 允许 |
| `/model` `/reasoning` `/access` | 拒绝 | 允许 | 允许 | 允许 | 允许 |

### 5.2 临时门禁覆盖矩阵

| 门禁状态 | 当前代码实际效果 | 审计结论 |
| --- | --- | --- |
| `E6 Abandoning` | 只允许 `/status`；`/detach` 也只返回等待收尾 notice；其余全部拒绝 | 这是强门禁，模型清晰，但缺超时恢复 |
| `G1/G2 PendingHeadless` | 文本、图片、`/detach` 会被挡；`/use`、`/follow`、`/list`、`/model` 等仍可能执行 | 这是当前最危险的半门禁 |
| `G3 SelectionPrompt` | 只有“纯数字文本”会被抢走；其他命令大多照常执行 | 这是输入歧义层，不是安全门禁层 |
| `G4 PendingRequest` | 普通文本、图片被挡；`/use`、`/follow`、`/detach` 仍可执行 | 有意设计可以保留，但需要和其他门禁解耦 |
| `G5 RequestCapture` | 下一条文本会变成反馈；图片被挡 | 当前会被 `SelectionPrompt` 的数字优先级抢掉 |
| `D1 StagedImages` | 当前不是门禁，不会阻止 `/use` `/follow` | 这是实现 bug，不该继续当“无状态草稿”看待 |

## 6. 当前应该成立但没有被代码守住的状态不变量

下面这些不是未来设计，而是当前产品语义下本来就应该成立的不变量。

### 6.1 任何时刻都不应存在“用户看起来能发，但实际上发不出去”的状态

当前被破坏的组合：

1. `R2 AttachedPinned + G2 PendingHeadlessSelecting`
2. `R4 FollowBound + G2 PendingHeadlessSelecting`

这两种状态下，surface 看起来已经选中了 thread，但文本/图片仍然被 `PendingHeadless` 拦住。

### 6.2 任何时刻都不应存在“用户看不到恢复按钮，但普通输入也继续被挡”的状态

当前被破坏的组合：

1. `SelectionPrompt.Kind == new_instance_thread` 已过期或已失效
2. `PendingHeadless == nil` 或仍处于 selecting
3. 文本仍然被 `headless_selection_waiting` 挡住

### 6.3 route 改变时，未冻结的输入不能静默跟着串到新 thread

当前被破坏的组合：

1. 先上传图片，进入 `D1 StagedImages`
2. 再 `/use` 或 `/follow`
3. 图片最终跟着新 thread 的第一条文本被消费

### 6.4 一个 blocked 状态必须只有一个明确逃生口

当前被破坏的状态：

1. `PendingHeadlessSelecting` 同时还能 `/use`、`/follow`
2. `SelectionPrompt` 还能跟 `RequestCapture` 叠加
3. `PausedForLocal` 与 `Abandoning` 都没有 watchdog

## 7. 已确认的 bug 级问题与死/半死状态

这一节只列我已经从代码确认的问题，尽量不夹带“也许”。

### 7.1 `attachInstance` 没有做全局 instance 互斥

当前 `attachInstance()` 只检查：

1. 当前 surface 是否已经 attach 了别的 instance。
2. 当前 surface 是否已经 attach 了同一个 instance。

它**不会**检查：

1. 这个 instance 是否已经被别的 surface attach。

这和当前产品约束不一致，也直接放大了后面的复杂度：

1. 一个 instance 可以被多个 surface 同时 attach。
2. `pauseForLocal()`、`enterHandoff()`、`dispatchNext()` 都会对这批 surface 产生交叉影响。
3. `findAttachedSurfaces(instanceID)` 的存在本身就证明当前代码在走“共享 instance”模型。

审计结论：

1. 这是 bug，不是产品待决。
2. 状态机加固时必须先补 instance claim。

### 7.2 `PendingHeadless` 不是 dominant gate，会制造半死状态

当前 `PendingHeadless` 没有在 `ApplySurfaceAction()` 顶层统一拦截。

已经确认的坏路径至少有三条：

#### 路径 A：`starting` 时还能 attach 其他实例

路径：

1. `/newinstance`
2. 进入 `PendingHeadlessStarting`
3. 再 `/list` 然后 attach 某个普通 instance

结果：

1. `attachInstance()` 会直接清掉 `surface.PendingHeadless`
2. 不会发 `DaemonCommandKillHeadless`
3. 后续 headless 实例连回 relay 后，不再有 surface 认领

这会产生 orphan headless。

#### 路径 B：`selecting` 时还能 `/use` 或 `/follow`

路径：

1. headless 已 attach，`PendingHeadless.Status=selecting`
2. 用户不处理恢复卡片，直接 `/use` 或 `/follow`

结果：

1. 路由表面上已经切到 thread
2. 但文本/图片继续被 `PendingHeadless` 拦住
3. 用户会落入“有 thread 但不能发”的半死状态

#### 路径 C：timeout 后会留下失效的 headless 选择卡片

当前 `Tick()` 在 headless timeout 时只做：

1. `surface.PendingHeadless = nil`
2. 发 `KillHeadless`
3. 发 timeout notice

它**不会**同步清掉 `SelectionPrompt.Kind == new_instance_thread`。

于是会出现：

1. 卡片还在
2. 文本仍然可能被 `headless_selection_waiting` 挡住
3. 按按钮又会因为 `PendingHeadless == nil` 失败

这是一个明确的 stale modal bug。

### 7.3 `SelectionPrompt` 不是纯展示，而是会劫持输入的真实状态

当前 `SelectionPrompt` 不是“只是把按钮渲染成卡片”。

它会真实影响状态机：

1. 数字文本先走 `resolveSelection()`。
2. 按钮走 `resolveSelectionOption()`。
3. attach/use/headless resume/kick confirm 全都依赖它。

这导致三个已确认问题。

#### 问题 A：数字文本和普通消息冲突

只要 `SelectionPrompt` 还在，用户发送 `"1"`、`"2"` 这类纯数字文本，就不会进入普通消息队列。

这会制造不可见 modal state：

1. 用户不一定记得十分钟前开过 `/use` 卡片。
2. 但 `"1"` 仍会被当成“切第一个 thread”。

#### 问题 B：`SelectionPrompt` 会和 `RequestCapture` 冲突

`handleText()` 的优先级是：

1. 先处理数字 selection
2. 再看 `PendingHeadless`
3. 再看 `ActiveRequestCapture`

所以如果当前同时存在：

1. `SelectionPrompt`
2. `RequestCapture`

那么用户发送 `"1"` 这类数字反馈时，文本会先被 selection 抢走，而不是进入 request feedback。

这是实打实的输入优先级 bug。

#### 问题 C：它会跨上下文残留

`/model`、`/reasoning`、`/access` 这类命令不会统一清除 `SelectionPrompt`。

于是用户可能发生：

1. 先 `/use`
2. 再改模型
3. 过一会儿发一个数字文本
4. 结果 thread 被切走，而不是发出普通消息

审计结论：

1. `SelectionPrompt` 不是值得继续扩展的 feature。
2. 它应该从“服务端持久状态”退化成“无状态卡片动作”。

### 7.4 图片暂存没有冻结路由，`/use` 会把图片带到别的 thread

当前 staged image 只记录：

1. 图片文件路径
2. 原消息 ID

它不记录：

1. 上传时的 `threadID`
2. 上传时的 `RouteMode`
3. 上传时的 `cwd`

因此当前存在明确错误路径：

1. 在 `thread-A` 上传图片
2. 不发文本
3. `/use thread-B`
4. 再发一条文本
5. 图片会和 `thread-B` 的文本一起发送

这和“queue 不乱、图片和文本不串”的要求直接冲突。

审计结论：

1. 这是 bug，不是交互偏好。
2. 不能继续把 staged image 当成“和路由无关的临时附件”。

### 7.5 `PausedForLocal` 没有 watchdog，会把远端队列永久卡死

当前本地活动路径是：

1. 看到 `local.interaction.observed` 或本地 turn start
2. 所有 attach 到该 instance 的 surface 进入 `DispatchModePausedForLocal`
3. 只有等本地 turn completed，才可能进入 `handoff_wait`
4. 再由 `Tick()` 恢复

问题是：

1. `PausedForLocal` 自身没有超时
2. 如果 completion 事件丢了
3. surface 会永远停在 paused

这不是理论风险，因为 `dispatchNext()` 在 `DispatchMode != normal` 时直接不工作。

用户虽然还能 `/status`、`/stop`、`/detach`，但对远端出队来说它已经是 deadlock。

### 7.6 `Abandoning` 也没有 watchdog，会把整个 surface 锁死

`/detach` 遇到 live work 时会进入 `Abandoning`。

进入以后：

1. 只有 `/status` 还能正常用
2. `/detach` 也只会返回“仍在等待收尾”
3. 其他操作全部拒绝

退出依赖：

1. 当前 turn 正常收尾
2. active queue fail/reject
3. instance disconnect

如果这些异步事件丢了，surface 会永久停在 `Abandoning`。

所以它是另一个明确的 half-dead state。

### 7.7 `/new` 相关的空 thread 关联链路仍然没打通

这个问题现在还没有直接炸，是因为当前产品层压根不允许 remote 处于“下一条消息建新 thread”的稳定态。

但代码层已经确认：

1. `dispatchNext()` 可以发 `CreateThreadIfMissing=true`
2. translator 也可以在 `ThreadID == ""` 时下发 `thread/start`
3. 可是 orchestrator 的 `queuedItemMatchesTurn()` 对 `FrozenThreadID == ""` 的 turn 归属仍然不可靠

所以 `/new` 不是“补一个命令”就能上的 feature，必须等现有状态机先加固。

这一条暂时是 latent bug，不是当前主线 bug，但它是 `/new` 的硬前置。

## 8. `SelectionPrompt` 是否应该删除

结论先写前面：

1. **应该删掉的是“服务端持久 `SelectionPrompt` 状态 + 数字文本解析”这套机制。**
2. **不需要删掉“卡片列选项”这个 UI 形式。**

### 8.1 当前四类用法

`SelectionPrompt` 现在承载了四条业务流：

1. attach instance 列表
2. thread 列表 `/use`
3. headless resume 选择
4. kick confirm

### 8.2 哪些可以直接改成无状态按钮

下面两类最简单：

1. attach instance
2. use thread

按钮直接携带目标 ID 即可：

1. `attach_instance(instance_id)`
2. `use_thread(thread_id)`

点击后服务端按**当前实时状态**重新校验，不需要保留旧 prompt 记录。

### 8.3 哪些需要补一个明确动作，但依然不需要保留 prompt 状态

下面两类仍然可以无状态，但要把动作说清楚：

1. kick confirm
2. headless resume

推荐动作形态：

1. `use_thread(thread_id, force=true)` 或独立 `kick_thread_confirm(thread_id)`
2. 独立 `headless_resume(thread_id)` 或 `resume_headless_thread(thread_id)`

无论哪一种，按钮点击后都重新校验：

1. thread 现在还在不在
2. owner 有没有变化
3. headless 还在不在 selecting

### 8.4 应该彻底删除的部分

下面这些建议直接删掉，不要保留兼容分支：

1. `Surface.SelectionPrompt`
2. `ActionSelectPrompt`
3. `resolveSelection()`
4. “纯数字文本 = 选择第 N 项”
5. 以 `prompt_id` 作为业务状态依赖的路径

如果还想复用当前的卡片布局 helper，可以只保留纯 UI 层的“选项卡片渲染”，但它不再参与 surface 状态机。

## 9. 修改方向建议

这一节故意按“bug 修复优先级”来排，不按 feature 讨论顺序排。

### 9.1 第一步：删除服务端 `SelectionPrompt` 状态

这是这次修改最适合作为第一刀的地方。

原因不是因为它最大，而是因为它最能先把状态机减下来：

1. 去掉不可见 modal state
2. 去掉数字文本歧义
3. 去掉 request capture 与 selection 的输入优先级冲突
4. 让后面的 headless / instance / thread gate 都能只围绕“真实业务状态”建模

目标效果：

1. 卡片还能发
2. 按钮还能点
3. 但 surface 不再记住“上一次弹过一个选择 prompt”

### 9.2 第二步：补 instance claim，恢复 attach 全局互斥

建议和上一步同阶段或紧跟其后落地。

目标不变量：

1. 一个 instance 同时只能被一个 surface attach
2. attach 要么完整成功，要么明确失败
3. 不再允许“共享 instance + thread 互斥”的现状继续存在

附带要求：

1. `/list` 的实例列表需要显示占用状态
2. 失败时必须给出明确下一步

### 9.3 第三步：把 `PendingHeadless` 收成真正的 dominant modal state

目标不变量：

1. 只要 `PendingHeadless != nil`，除了 `/status`、`/killinstance`、headless resume 按钮外，不允许旁路路由动作
2. timeout、disconnect、cancel 必须同时清理 headless 相关 UI 与 surface 状态
3. 不允许留下“stale headless 选择卡片”

这一步本质上不是做新设计，而是把当前已经存在的 headless 流程真正闭环。

### 9.4 第四步：让未冻结草稿在 route 变化时有明确命运

图片和其他未冻结草稿不能再跟着 `/use` 串路由。

当前最简单且一致的方案是：

1. route-changing 命令执行前，如果存在 `StagedImages`
2. 直接丢弃这些草稿
3. 用 reaction / notice 明确告诉用户已经丢弃

原因：

1. 它和 `/detach`、`/stop` 的“丢草稿”语义一致
2. 比“把图片 freeze 到旧 thread，然后 route 又切到新 thread”更不绕
3. 不会引入新的隐式状态

如果后续想做“图片也冻结路由”，可以再演进，但第一轮加固不需要先把自己做复杂。

### 9.5 第五步：给 `PausedForLocal` 和 `Abandoning` 加 watchdog

这里不一定需要新增用户命令，先把恢复性补上。

最低要求：

1. `PausedForLocal` 超过阈值后要么自动回到 `handoff_wait`，要么直接恢复 `normal` 并告警
2. `Abandoning` 超过阈值后要么强制 finalize，要么明确标记 failed-detach 并允许用户继续操作

这一层的目标不是追求完美，而是防止 surface 永久锁死。

### 9.6 第六步：确认空 thread 关联链路后再做 `/new`

到这一步之前，不建议继续推进 `/new`。

因为 `/new` 需要的新能力不是“多一个命令”，而是：

1. 明确的新稳定路由态
2. 可靠的空 thread 创建归属
3. route change 时草稿和 queue 的确定语义

这些都建立在前五步先收口的基础上。

## 10. 需要补的测试清单

这次如果进入实现，我会把下面这些回归测试作为最低覆盖面。

### 10.1 instance / thread claim

1. 第二个 surface attach 同一个 instance 时必须明确失败
2. busy instance 不应进入任何“半 attach”状态
3. thread kick 只在 idle 时允许

### 10.2 headless gate

1. `PendingHeadlessStarting` 时 `/attach` `/use` `/follow` 全部被挡
2. `PendingHeadlessSelecting` 时 `/attach` `/use` `/follow` 全部被挡
3. headless timeout 必须同时清理 pending 状态和相关选择卡片
4. headless cancel / disconnect 后不得遗留 stale UI gate

### 10.3 SelectionPrompt removal

1. 不再存在“数字文本选第 N 项”
2. attach/use/headless/kick 都改为按钮直达动作
3. request capture 下发送 `"1"` 必须进入反馈，而不是切 thread

### 10.4 草稿路由

1. 上传图片后 `/use`，图片必须被明确丢弃或被明确阻止切换
2. follow 切 thread 时，staged image 不得串到新 thread
3. `/detach` `/stop` 必须继续明确丢弃草稿

### 10.5 watchdog

1. `PausedForLocal` 丢 completion 时不会永久卡住出队
2. `Abandoning` 丢 completion 时不会永久锁死 surface

### 10.6 `/new` 前置

1. `FrozenThreadID == ""` 的 pending remote turn 能正确归属到新 thread
2. 新 thread 建成后 surface 会被稳定绑定回去

## 11. 当前结论

这轮重查以后，结论比上一版更明确：

1. 当前大部分问题确实是实现 bug，不是产品层面还没想清楚。
2. 现有状态机真正复杂的地方，不在 route mode 本身，而在一堆“半门禁、半草稿、半输入 modal”的叠加。
3. `SelectionPrompt` 继续保留在服务端状态里只会放大复杂度，应该作为第一刀移除。
4. instance attach 全局互斥、headless dominant gate、草稿 route 收口、watchdog，这四项都是 `/new` 之前必须先修的基础设施。

所以在实现 `/new` 之前，这份文档建议先把当前 remote surface 状态机按上面的顺序收紧，先把现有行为做成一个不容易进入半死状态的系统。

## 12. 待讨论取舍

当前无。
