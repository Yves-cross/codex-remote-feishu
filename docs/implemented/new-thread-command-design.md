# `/new` 新建会话命令设计

> Type: `implemented`
> Updated: `2026-04-10`
> Summary: 记录当前已实现的 `/new` 行为：通过 `new_thread_ready` 清空远端上下文、保留工作目录与飞书侧 override、由首条普通文本创建新 thread，并用显式 surface correlation 回绑到正确的飞书 surface；同时区分“已准备新建会话”和“已切换到现有会话”的飞书提示卡语义，避免把 ready 态误读成真实 thread 切换。

## 1. 文档定位

这份文档描述的是**当前已经实现**的 `/new` 功能，不再是草稿方案。

目标产品语义是 Claude Code `/clear` 那类“清上下文，但保留工作环境”的体验：

1. 保留当前 instance attachment。
2. 释放当前 thread claim，不再继续占用旧 thread。
3. 继承当前工作目录与飞书侧 prompt override。
4. 让下一条普通输入自动创建一个新 thread。

核心状态机基线见：

1. [remote-surface-state-machine.md](../general/remote-surface-state-machine.md)

## 2. 当前实现概览

飞书侧新增两个入口：

1. slash 命令：`/new`
2. 菜单命令：`new`

执行成功后：

1. surface 进入 `RouteModeNewThreadReady`。
2. `SelectedThreadID` 被清空。
3. 当前旧 thread claim 被释放。
4. `PreparedThreadCWD`、`PreparedFromThreadID`、`PreparedAt` 被保存。
5. 下一条普通文本会以 `CreateThreadIfMissing=true` 派发。

新 thread 真正落地后：

1. queue item 会回填真实 `threadID`。
2. surface 自动从 `new_thread_ready` 切回 `pinned`。
3. 新 thread claim 由当前 surface 占有。

## 3. 状态与数据模型

### 3.1 新增 route mode

当前新增：

1. `state.RouteModeNewThreadReady = "new_thread_ready"`

语义：

1. surface 仍 attach 在当前 instance 上。
2. 当前没有 selected thread。
3. 当前没有 thread claim。
4. 但下一条普通文本拥有合法 `cwd`，可以直接创建新 thread。

### 3.2 surface 字段

当前实现使用：

1. `PreparedThreadCWD string`
2. `PreparedFromThreadID string`
3. `PreparedAt time.Time`

其中：

1. `PreparedThreadCWD` 是执行必需字段。
2. `PreparedFromThreadID` 用于调试和状态可读性。
3. `PreparedAt` 用于调试和幂等 `/new` 复位时间。

## 4. `/new` 的前置条件

### 4.1 必须已经 attach，且当前真实持有一个 thread

只有同时满足下面条件时，`/new` 才会成功：

1. 当前 surface 已 attach 某个 instance。
2. 当前 `SelectedThreadID != ""`。
3. 当前 surface 真实持有该 thread claim。
4. 当前 selected thread 当前可见。
5. 当前 selected thread 的 `CWD` 非空。

这里保持一条硬约束：

1. `/new` 不允许 fallback 到 `Instance.WorkspaceRoot`。
2. `/new` 不允许 fallback 到 home。
3. `/new` 只能继承“当前真实工作 thread 的真实 cwd”。

### 4.2 强门禁

以下状态下 `/new` 会被拒绝：

1. `PendingHeadlessStarting`
2. `PendingHeadlessSelecting`
3. `Abandoning`
4. `ActiveRequestCapture != nil`
5. `PendingRequests` 非空

原因很直接：

1. headless / abandoning 本身已经是顶层 gate。
2. request capture / pending request 会和“下一条文本创建新 thread”的语义冲突。

### 4.3 dispatching/running 与 queued/staged 的区别

当前 `/new` 的 clear 语义是：

1. `dispatching` / `running` 时拒绝。
2. 仅有 `queued` draft 或 `staged image` 时允许。
3. 成功进入 `new_thread_ready` 时，会主动丢弃这些 unsent draft。

也就是说：

1. `/new` 不会切过一个已经开始派发的远端 turn。
2. 但它会清掉尚未真正发出的旧上下文草稿。

## 5. `new_thread_ready` 下的输入与命令语义

### 5.1 首条普通文本

第一条普通文本合法。

实现方式：

1. `freezeRoute()` 返回 `threadID=""`
2. `cwd = PreparedThreadCWD`
3. `createThread = true`
4. queue item 以“空 thread + 固定 cwd + routeMode=new_thread_ready”的形式冻结

随后沿现有链路：

1. `prompt.send`
2. translator 生成 `thread/start`
3. `thread/start` 成功后生成 `turn/start`
4. `turn.started` 到达后回填真实 `threadID`

### 5.2 图片

当前规则是：

1. 在首条文本尚未入队前，可以先暂存图片。
2. 首条文本一旦 queued/dispatching/running，新图片就会被拒绝。

这样可以支持：

1. 先准备新会话
2. 再发送第一条图文消息

同时避免：

1. “这张图到底属于新 thread 首条消息还是下一条消息”的歧义。

### 5.3 第二条文本

当前 V1 明确禁止。

规则：

1. `new_thread_ready` 且尚未有首条 queued/active item：允许第一条文本。
2. `new_thread_ready` 且已经存在首条 queued/dispatching/running item：拒绝第二条文本。

原因：

1. 当前每个 `FrozenThreadID == ""` 的 queue item 都会各自创建一个新 thread。
2. 不允许在 V1 做“多条消息自动合并到同一个待创建 thread”的隐式魔法。

### 5.4 `/use`、`/follow`、`/detach`

当前实现分三种情况：

1. 只有 `PreparedThreadCWD`，没有任何草稿：
   1. `/use`、`/follow`、`/detach` 都允许。
2. 有 staged image 或首条 queued draft，但尚未 dispatching：
   1. `/use`、`/follow`、`/detach` 允许。
   2. 若是 `/use`、`/follow`，会先 `discardDrafts()` 再执行目标动作。
3. 首条消息已经 `dispatching` / `running`：
   1. `/use`、`/follow`、重复 `/new` 会拒绝。
   2. `/detach` 仍可走现有 abandoning 语义。

### 5.5 重复 `/new`

当前实现是幂等的：

1. 已在 `new_thread_ready` 且没有 draft：回 `already_new_thread_ready`。
2. 已在 `new_thread_ready` 且只有 staged/queued draft：先丢弃 draft，再保持 `new_thread_ready`。
3. 已在 `new_thread_ready` 且首条消息已 `dispatching/running`：拒绝。

### 5.6 `/model`、`/reasoning`、`/access`、`/stop`

当前都继续可用：

1. `/model`、`/reasoning`、`/access` 只影响“之后从飞书发出的消息”的 override。
2. `/stop` 在没有 active turn 时，仍可以清掉 staged/queued draft。

## 6. 空 thread turn 的精确归属

这是 `/new` 实现里最关键的部分。

### 6.1 不再依赖 `ActiveThreadID` 猜归属

当前空 thread 的 turn 归属不再靠：

1. `FrozenThreadID == ""`
2. `threadID == inst.ActiveThreadID`

这种猜测逻辑。

### 6.2 当前实现方式

当前实现改成：

1. dispatch 时，`pendingRemote` 仍记住 surface 归属。
2. translator 在 `turn.started` 时提供 `InitiatorRemoteSurface + SurfaceSessionID`。
3. orchestrator promote pending remote 时，优先按 `Initiator.SurfaceSessionID` 命中 pending item。
4. 命中后回填真实 `threadID`，并把 surface 绑定回这个 thread。

结果：

1. 新 thread 不会再靠 `ActiveThreadID` 猜测归属。
2. `turn.started` 后 surface 会稳定切回正确的飞书会话。

### 6.3 失败回滚

如果 `thread/start`、`turn/start` 或 command dispatch 在 thread 真正落地前失败：

1. 当前首条 queue item 会按失败处理。
2. surface 保持在 `new_thread_ready`。
3. `PreparedThreadCWD` 保留。
4. 用户可以直接重试下一条文本，或改用 `/use` / `/detach` 离开。

不会发生：

1. 偷偷回到旧 thread。
2. 落回 `unbound` 但用户不知道下一步。

## 7. 本地 VS Code 仲裁

`/new` 不会脱离当前 instance attach。

所以在 `new_thread_ready` 下：

1. 如果 VS Code 本地有活动，首条消息仍可能进入 `paused_for_local` 队列。
2. 如果进入 `handoff_wait`，仍要等现有 watchdog / handoff 逻辑放行。

这不是 bug，而是当前 instance 级仲裁模型的自然结果。

## 8. Snapshot 与 Feishu 投影

当前 snapshot / projector 已经表达：

1. attachment 当前输入目标：`新建会话（等待首条消息）`
2. next prompt 目标：`新建会话`
3. next prompt `CreateThread = true`
4. next prompt `CWD = PreparedThreadCWD`
5. 成功 `/new` 后会投影明确 notice：`已清空当前远端上下文。下一条文本会创建新会话。`
6. `/new` 触发的 thread-selection 提示当前不会再写成“当前输入目标已切换到…”，而是明确提示：`已准备新建会话。当前还没有实际会话 ID；下一条文本会作为首条消息创建新会话。`

## 9. 当前测试覆盖

当前至少覆盖了下面几类测试：

1. gateway：`/new` 与菜单 `new` 正确映射到 action
2. projector：`new_thread_ready` 快照文案正确
3. orchestrator：`/new` 进入 ready、首条文本创建 thread、第二条文本被拒绝
4. orchestrator：`new_thread_ready` 下 `/use` / 重复 `/new` 会正确丢弃 queued draft
5. orchestrator：`turn.started(remote_surface)` 能把 surface 从 `new_thread_ready` 重新绑定到新 thread
6. daemon：`/new` 的 ready 态投影与 snapshot 正确

相关测试文件：

1. [gateway_test.go](../../internal/adapter/feishu/gateway_test.go)
2. [projector_test.go](../../internal/adapter/feishu/projector_test.go)
3. [service_test.go](../../internal/core/orchestrator/service_test.go)
4. [app_test.go](../../internal/app/daemon/app_test.go)

## 10. 相关实现

主要实现路径：

1. [gateway.go](../../internal/adapter/feishu/gateway.go)
2. [types.go](../../internal/core/control/types.go)
3. [types.go](../../internal/core/state/types.go)
4. [service.go](../../internal/core/orchestrator/service.go)
5. [projector.go](../../internal/adapter/feishu/projector.go)
