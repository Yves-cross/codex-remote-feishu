# `/new` 新建会话命令设计

> Type: `draft`
> Updated: `2026-04-06`
> Summary: 将 `/new` 收口为 `new_thread_ready` 稳定态方案，由首条输入触发真实 thread 创建。

## 1. 文档定位

这份文档讨论飞书 remote 端的 `/new` 命令，以及对应菜单命令 `new`。

目标是提供一个类似 Claude Code `/clear` 的产品语义：

1. 保留当前 instance attachment。
2. 清空“之后要继续在哪个 thread 上工作”的上下文。
3. 继承当前工作目录和飞书侧覆盖参数。
4. 让用户下一条输入自动落到一个全新的 thread。

这份设计建立在 [多飞书 App 功能设计](./multi-feishu-app-design.md) 之上，沿用其中两条硬约束：

1. `threadID` 在同一台机器、同一个 `relayd` 仲裁域内全局唯一。
2. 同一台机器上只运行一个 `relayd`，由它负责 remote claim 仲裁。

## 2. 当前代码事实

这部分只写已经从代码确认的事实，不做推断。

### 2.1 wrapper 观察到的本地 `thread/start` 来自真实上游输入

在 wrapper 中：

1. `stdinLoop()` 先从 `parent.stdin` 读原始 JSONL。
2. 同一行数据先交给 `translator.ObserveClient()`。
3. 然后原样转发到 `codex.stdin`。

对应代码：

- [internal/app/wrapper/app.go](../../internal/app/wrapper/app.go)

这意味着：

1. `ObserveClient()` 看见的 `thread/start` / `turn/start` 不是 translator 在观察路径上伪造出来的。
2. 它们来自真实上游客户端。

### 2.2 remote 首条消息创建新 thread 的链路已经存在

当前 orchestrator 和 translator 已经有一条现成链路：

1. 如果 queue item 的 `FrozenThreadID == ""`，`dispatchNext()` 会下发 `prompt.send`，并带 `Target.CWD`。
2. translator 收到 `prompt.send` 且 `Target.ThreadID == ""` 时，会主动向 codex 下发 `thread/start`。
3. `thread/start` 成功后，translator 会再跟一个 `turn/start`。
4. 远端 turn 真正开始后，server 会把当前 surface 绑定到新 thread。

对应代码：

- [internal/core/orchestrator/service.go](../../internal/core/orchestrator/service.go)
- [internal/adapter/codex/translator.go](../../internal/adapter/codex/translator.go)

### 2.3 当前 remote 产品层还没有稳定的“待新建 thread”状态

当前 surface 只有三种 route mode：

1. `pinned`
2. `follow_local`
3. `unbound`

`unbound` 目前会直接拦截文本和图片输入，不允许进入“等第一条消息再创建新 thread”的工作流。

## 3. 方案结论

`/new` 的主方案定为：

1. 不在命令触发时立刻显式创建空 thread。
2. 而是让 surface 进入一个新的稳定态：`new_thread_ready`。
3. 在这个状态里，下一条普通文本才真正触发 `thread/start -> turn/start`。
4. 新 thread 落地后，surface 自动切回 `pinned` 并绑定到新 thread。

不选“`/new` 一执行就立刻 create thread”的原因是：

1. 当前产品目标更像“清上下文并等待下一条消息”，不是“先造一个空 thread”。
2. 立即 create 需要额外引入一整套 `thread/start only` 的 pending、超时、回滚状态机。
3. 它更容易制造用户没有实际使用过的空 thread。
4. 当前代码已经天然支持“首条消息触发创建”这条路径。

## 4. 用户语义

### 4.1 表面行为

飞书侧新增两个入口：

1. slash 命令：`/new`
2. 菜单命令：`new`

触发后：

1. 不立刻发消息。
2. 不立刻创建真实 thread。
3. 当前 surface 进入“准备新建会话”的稳定态。
4. 下一条普通文本会自动创建新 thread 并发送到那里。
5. 期间用户仍然可以改成 `/use` 切去别的 thread。

### 4.2 和 `/clear` 的关系

`/new` 的产品意图是“之后不要再沿用当前上下文”，不是“复制旧 thread”。

所以 `/new` 成功进入稳定态后：

1. 保留 instance attachment。
2. 保留 `surface.PromptOverride`。
3. 不再保留当前 thread claim。
4. 之后第一条输入进入新 thread。

### 4.3 和 `follow_local` 的关系

如果当前 surface 处于 `follow_local`，但此刻已经实际跟随到了一个可用 thread，并持有该 thread claim，则允许执行 `/new`。

执行后应当：

1. 退出 `follow_local`。
2. 进入 `new_thread_ready`。

原因是：

1. `follow_local` 表示 thread 目标由 VS Code 焦点驱动。
2. `/new` 表示“从飞书侧准备开启一条新的独立上下文”。
3. 二者语义冲突，不能同时保持。

## 5. 新增稳定态

### 5.1 状态定义

建议新增：

1. `state.RouteModeNewThreadReady = "new_thread_ready"`

并在 `SurfaceConsoleRecord` 中新增至少一个字段：

1. `PreparedThreadCWD string`

推荐含义：

1. surface 仍然 attach 在某个 instance 上。
2. 当前没有 `SelectedThreadID`。
3. 当前没有 thread claim。
4. 但已经保存好了“下一条消息创建新 thread 时必须使用的 cwd”。

### 5.2 为什么不用 `unbound`

`unbound` 的产品语义是：

1. 当前没有 thread。
2. 也没有确定的新 thread 创建上下文。
3. 用户必须 `/use` 或 `/follow` 才能继续。

`new_thread_ready` 的语义是：

1. 当前没有 thread。
2. 但已经拥有合法 `cwd`。
3. 用户可以直接发送下一条文本来创建新 thread。

所以它不能复用 `unbound`。

## 6. `/new` 前置条件

只有满足下面条件时，才允许进入 `new_thread_ready`：

1. 当前 surface 已 attach 某个 instance。
2. 当前 surface 已有 `SelectedThreadID`。
3. 当前 surface 真实持有该 thread claim。
4. 当前 selected thread 仍然可见。
5. 当前 selected thread 的 `CWD` 非空。
6. 当前没有 `dispatching` / `running` work。

对 queue 的规则更细一点：

1. 普通 pinned/follow_local 状态下已有 queued item 时，`/new` 仍然拒绝。
2. 因为这表示用户还在用旧 thread 的队列，不能静默改语义。

必须明确写死：

1. `/new` 不允许 fallback 到 `Instance.WorkspaceRoot`。
2. `/new` 不允许 fallback 到 home。
3. `/new` 必须拿到当前 selected thread 的真实 `CWD`。

## 7. 进入 `new_thread_ready`

`/new` 执行成功时，建议按下面顺序收口：

1. 校验前置条件。
2. 读取当前 selected thread 的 `CWD`。
3. 清掉当前 surface 的 staged image 和 queued item。
   - 当前这一步应只清“还没发送的草稿”。
   - 因为 `/new` 的语义就是清上下文。
4. 清掉当前 surface 对旧 thread 的 claim。
5. `SelectedThreadID = ""`
6. `RouteMode = new_thread_ready`
7. `PreparedThreadCWD = 当前 thread.CWD`
8. 发 notice，明确提示“下一条文本会创建新会话”

这意味着：

1. `/new` 成功后，旧 thread 立刻可被别的 surface 接管。
2. surface 不会停留在“还绑定旧 thread，但嘴上说下条要开新 thread”的矛盾状态。

## 8. `new_thread_ready` 下的输入规则

### 8.1 文本

`new_thread_ready` 下，第一条普通文本是合法的。

处理方式：

1. `freezeRoute()` 返回 `threadID=""`
2. `cwd = surface.PreparedThreadCWD`
3. `createThread = true`
4. queue item 以“空 thread + 固定 cwd”的形式冻结

随后沿现有远端发送链路：

1. `prompt.send`
2. translator 生成 `thread/start`
3. 成功后跟 `turn/start`
4. 新 turn 启动时回填真实 thread

### 8.2 图片

`new_thread_ready` 下允许先暂存图片。

原因：

1. 这和“先准备新会话，再发送第一条图文消息”的用户预期一致。
2. 当前 staged image 本来就是 surface-local。

### 8.3 第二条文本限制

V1 必须限制：

1. 在新 thread 真正落地前，不允许第二条普通文本继续入队。

原因：

1. 当前代码里，`FrozenThreadID == ""` 的 queue item 会各自触发一次新 thread 创建。
2. 如果允许连续两条文本排队，就会创建两个新 thread。

所以 V1 的规则是：

1. `new_thread_ready` 且还没有任何 queued/active item：允许第一条文本。
2. `new_thread_ready` 且已经存在 queued/dispatching item：拒绝后续文本，并提示等待新会话落地，或 `/use` 切走。

### 8.4 第一条文本已排队但尚未落地时的图片

V1 建议也直接拒绝。

原因：

1. 第一条文本已经决定了“这次创建新 thread 的首条消息”内容。
2. 再追加 staged image 会制造“图片到底属于新 thread 首条消息，还是下一条消息”的歧义。

所以 V1 收口为：

1. 进入 `new_thread_ready` 后，可以先攒图片再发第一条文本。
2. 一旦第一条文本已经 queued/dispatching，直到新 thread 真正落地前，新的图片也先拒绝。

## 9. `/use`、`/follow`、`/detach` 在该状态下的规则

这是这次实现最关键的边界之一。

### 9.1 没有草稿时

如果当前只是单纯处于 `new_thread_ready`，但还没有 staged image / queued item：

1. `/use` 正常切到目标 thread
2. `/follow` 正常进入跟随
3. `/detach` 正常断开

### 9.2 只有 staged image 或 queued 首条消息时

如果当前还没有真正落地到新 thread，只是存在：

1. staged image
2. 或第一条 queued 但尚未 running 的文本

则 `/use`、`/follow`、`/detach` 允许继续，但处理方式是：

1. 先 `discardDrafts()`
2. 再执行目标动作
3. 保留现有 discarded reaction / thumbs-down 反馈

这符合产品预期：

1. 用户知道自己是在放弃这次“准备开启新会话”的草稿。
2. 系统不会卡在“有一条未落地的新会话首条消息，所以你什么也做不了”的死状态。

### 9.3 已经 dispatching / running 时

如果首条消息已经进入 `dispatching` 或 `running`：

1. `/use`
2. `/follow`
3. `/new`

都继续禁止。

因为这时新 thread 创建已经在进行或已经绑定到 turn，不能再当作“未落地草稿”对待。

## 10. 路由冻结与绑定

### 10.1 `freezeRoute()`

需要新增一条分支：

1. 当 `surface.RouteMode == new_thread_ready` 时
2. 返回 `threadID=""`
3. 返回 `cwd=surface.PreparedThreadCWD`
4. 返回 `createThread=true`

### 10.2 queue item 冻结

第一条文本 enqueue 时：

1. `FrozenThreadID = ""`
2. `FrozenCWD = PreparedThreadCWD`
3. `RouteModeAtEnqueue = new_thread_ready`

### 10.3 新 thread 落地

当前 `markRemoteTurnRunning()` 已经会在 `FrozenThreadID == ""` 时：

1. 用真实 `threadID` 回填 queue item
2. 再把 surface 绑定到这个 thread

这部分可以复用，但需要加一条收口规则：

1. 如果 `RouteModeAtEnqueue == new_thread_ready`
2. 真正绑定 surface 时必须改成 `pinned`

不能继续保留 `new_thread_ready`。

## 11. 快照与投影

### 11.1 Snapshot

`buildSnapshot()` 和 `resolveNextPromptSummary()` 需要能表达：

1. 当前 attachment route mode 是 `new_thread_ready`
2. 当前 next prompt `CreateThread = true`
3. 当前 next prompt `CWD = PreparedThreadCWD`

### 11.2 Feishu 文案

状态文案建议改成：

1. attachment 当前输入目标：`新建会话（等待首条消息）`
2. next prompt 目标：`新建会话`

这样用户能同时看见：

1. 自己当前不在旧 thread 上
2. 下一条消息会创建新 thread

## 12. 不选的备选方案

这次不作为主方案的备选是：

1. `/new` 立刻下发显式 `thread/start`
2. thread 建好后再切过去

这个方案不是做不到，而是当前不作为主方案。原因：

1. 它要求新增 `thread/start only` 的 canonical command 和完成事件。
2. 它要求补一套独立的 pending、超时和失败回滚状态机。
3. 它会更容易积累“从未发送任何消息的空 thread”。
4. 当前需求并不要求 `/new` 一触发就必须拿到真实 thread id。

如果未来产品明确要求“`/new` 后立刻可见一个真实 thread id”，再单独升级到该方案。

## 13. 实施计划

这项功能建议一次性完成，不拆成对外可见阶段。

顺序建议：

1. 更新文档与状态模型。
2. 增加 `/new` 的 gateway 入口。
3. 实现 `new_thread_ready` 路由与草稿约束。
4. 调整 `/use`、`/follow`、`/detach` 的切走语义。
5. 补测试并做独立复核。

## 14. 测试矩阵

至少覆盖下面这些测试：

1. gateway 单测：`/new` 和菜单 `new` 正确映射到 action
2. orchestrator 单测：未 attach 时 `/new` 被拒绝
3. orchestrator 单测：没有 selected thread 时 `/new` 被拒绝
4. orchestrator 单测：selected thread 无 `cwd` 时 `/new` 被拒绝，且不会 fallback 到 `WorkspaceRoot`
5. orchestrator 单测：已有旧 thread queue 时 `/new` 被拒绝
6. orchestrator 单测：`follow_local` 且已拥有 thread 时 `/new` 成功进入 `new_thread_ready`
7. orchestrator 单测：`new_thread_ready` 下首条文本会以 `CreateThreadIfMissing=true` 下发
8. orchestrator 单测：`new_thread_ready` 下第二条文本被拒绝
9. orchestrator 单测：`new_thread_ready` 下首条文本 queued 后，新增图片被拒绝
10. orchestrator 单测：`new_thread_ready` 下 `/use` 会先丢弃草稿再切走
11. orchestrator 单测：`new_thread_ready` 下 `/detach` 会先丢弃草稿再断开
12. orchestrator 单测：空 thread queue item 在真正 `turn.started` 后会把 surface 绑定到新 thread，并切回 `pinned`
13. translator 单测：空 thread 的 remote `prompt.send` 仍先发 `thread/start` 再跟 `turn/start`
14. 集成测试：`/new` 后首条图文消息进入新 thread
15. 集成测试：`/new` 后改用 `/use` 切去其他 thread 时，未落地草稿被正确丢弃并有反馈
