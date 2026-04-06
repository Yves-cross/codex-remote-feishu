# 多飞书 App 功能设计

> Type: `draft`
> Updated: `2026-04-06`
> Summary: 补充 threadID 单机全局唯一与单机单 relayd 约束，并统一多飞书 App 的 attach、/use、强踢与恢复态语义。

## 1. 文档定位

这份文档讨论的是“多飞书 App 功能”整体设计，不只讨论隔离。

文档要回答四类问题：

1. 多个飞书 App 同时在线时，系统对用户呈现什么能力。
2. instance、thread、queue、preview、follow 的所有权如何定义。
3. 哪些行为允许抢占，哪些行为必须互斥。
4. 哪些复杂路径不建议在第一版就做。

这份文档当前仍是讨论稿，不是施工说明书；本轮目标是先把产品语义收口，不直接开工。

## 2. 目标与非目标

### 2.1 目标

这轮设计收口后的目标是：

1. 一个 daemon 可以同时接多个飞书 App。
2. 所有 app 都能看到全局在线 instance。
3. 所有 app 都能看到全局可见 thread。
4. instance attach 互斥。
5. thread attach 互斥。
6. 不同 app 的 surface 状态、queue、图片暂存、preview rewrite 不串。
7. 对 VS Code 共享实例保留 follow 模式。
8. 内部上 instance claim 与 thread claim 仍分离建模，并允许进入“已 attach instance、未绑定 thread”的受限状态。
9. `/detach` 时废弃当前 surface 的草稿态，并正确处理中断中的 active turn。

### 2.2 非目标

这轮不打算解决：

1. 阻止本地 VS Code 与 remote 同时操作同一 thread。
2. 让多个 remote surface 共享同一个 instance 并做公平调度。
3. 做完整的“多租户权限系统”。
4. 把 env override 扩展成完整的多 app 配置源。
5. 做一个独立的“全局 thread catalog 服务”；V1 仍允许 thread 可见性来自各 instance 的线程快照。

## 3. 当前代码已经具备的基础

### 3.1 多 gateway runtime 已落地

当前 daemon 已不是单 gateway 模型：

- [internal/app/daemon/entry.go](../../internal/app/daemon/entry.go)
- [internal/app/daemon/startup.go](../../internal/app/daemon/startup.go)
- [internal/adapter/feishu/controller.go](../../internal/adapter/feishu/controller.go)

现状：

1. `feishu.apps[]` 会展开成多个 `GatewayAppConfig`。
2. `MultiGatewayController` 为每个 `gatewayID` 维护独立 worker。
3. 单 app 支持热更新、热停用、热重连。

### 3.2 surface 已是 gateway-aware

当前 surface id 已包含 gateway 维度：

- `feishu:{gatewayID}:chat:{chatID}`
- `feishu:{gatewayID}:user:{userID}`

对应代码：

- [internal/adapter/feishu/identity.go](../../internal/adapter/feishu/identity.go)
- [internal/adapter/feishu/gateway.go](../../internal/adapter/feishu/gateway.go)

这意味着不同 app 下，即使 `chatID` 或 `actorUserID` 相同，也会进入不同 surface。

### 3.3 surface 内部状态已经天然分离

当前 orchestrator 的主要状态都挂在 `root.Surfaces[surfaceSessionID]`：

- [internal/core/state/types.go](../../internal/core/state/types.go)
- [internal/core/orchestrator/service.go](../../internal/core/orchestrator/service.go)

已经按 surface 分离的内容包括：

1. `AttachedInstanceID`
2. `SelectedThreadID`
3. `QueueItems`
4. `StagedImages`
5. `PromptOverride`
6. `PendingRequests`
7. `ActiveRequestCapture`

### 3.4 出站投影已经按 gateway 路由

当前 UIEvent 到 Feishu 的投影链路已经会带 `gatewayID`：

- [internal/app/daemon/app.go](../../internal/app/daemon/app.go)
- [internal/adapter/feishu/projector.go](../../internal/adapter/feishu/projector.go)
- [internal/adapter/feishu/controller.go](../../internal/adapter/feishu/controller.go)

`MultiGatewayController` 在多 worker 场景下不会允许“漏 gatewayID 然后静默发错 app”。

### 3.5 preview / image staging 已有 per-gateway 隔离

对应代码：

- [internal/app/daemon/entry.go](../../internal/app/daemon/entry.go)
- [internal/adapter/feishu/controller.go](../../internal/adapter/feishu/controller.go)
- [internal/adapter/feishu/markdown_preview.go](../../internal/adapter/feishu/markdown_preview.go)

当前已经做到：

1. image staging 目录按 gateway 拆分。
2. markdown preview state 文件按 gateway 拆分。
3. preview scope key 本身带 gatewayID。

这意味着多 app 下：

1. preview rewrite 不会复用同一份本地状态。
2. 权限授权不会天然串到另一个 app 的 surface。
3. image staging cleanup 可以做成 per-gateway-aware。

### 3.6 当前 orchestrator 仍是“单 owner 假设”

这部分是这轮设计必须正视的现状：

1. `attachInstance()` 当前会直接覆盖 `surface.AttachedInstanceID` / `SelectedThreadID`，没有 claim 冲突检查。
2. `pendingRemote` / `activeRemote` 仍按 `instanceID` 建索引，而不是按显式 claim 建索引。
3. `findAttachedSurfaces(instanceID)` 当前允许一个 instance 同时挂多个 surface。
4. 本地 VS Code turn 开始时，当前实现会把同一 instance 下的多个 attached surface 一起推到该 thread。

对应代码：

- [internal/core/orchestrator/service.go](../../internal/core/orchestrator/service.go)

这说明当前系统虽然已经多 gateway 了，但远没到“多 app 功能已经完整闭环”的程度。

## 4. 设计原则

### 4.1 全局可见与全局仲裁分开

原则：

1. instance / thread 的“可见性”是全局的。
2. instance / thread 的“占用权”也必须是全局仲裁的。
3. 但 queue、staged image、preview rewrite、notice 投影仍是 surface-local 的。

### 4.2 remote claim 只约束 remote，不约束本地 VS Code

这点需要明确写进设计：

1. `instanceClaim` / `threadClaim` 只解决 remote surface 之间的竞争。
2. 本地 VS Code 仍可能在同一个 instance 或 thread 上继续操作。
3. follow 只能做 claim-aware best effort，不能承诺阻止本地端。

### 4.3 attach instance 与 attach thread 是两个动作

这是本文最核心的收口：

1. 在内部实现上，instance claim 与 thread claim 必须分开建模。
2. 在用户语义上，`attach instance` 与 `attach thread` 可以拆开完成。
3. 冲突大多发生在 thread claim，而不是 instance claim。

### 4.4 一切选择动作都要二次校验

任何 prompt、按钮、卡片都是“旧快照”。

因此：

1. `/list` 选项点击时要重新校验 instanceClaim。
2. `/use` 选项点击时要重新校验 threadClaim。
3. `强踢` 点击时要重新校验目标 thread 的最新 running / queued 状态。

### 4.5 允许受限状态，但必须给出明确下一步

这是这轮讨论新增的硬约束：

1. attach 可以把用户带到“已接管 instance，但当前没有 threadClaim”的状态。
2. 但这个状态必须是显式、可理解、且有明确下一步的。
3. 系统进入这个状态时，必须立刻给 notice，并尽量主动发 `/use` 选择卡片。

## 5. 当前实现为什么还不能直接算“完整支持”

虽然基础设施已经在，但核心所有权模型还没有立起来。

当前仍存在这些问题：

1. `instance` 虽然全局可见，但没有显式 owner map。
2. `thread` 虽然可见，但没有显式 owner map。
3. remote turn 仍按 `instanceID` 串行绑定，而不是按 claim 模型显式收口。
4. 某些 fallback 仍靠“按 instance 找一个 surface”。
5. attach instance 时不会检查默认 thread 是否已被其他 surface 占用。
6. detach 遇到 active remote turn 时，旧 turn 尾流仍有回错 surface 的风险。
7. 本地 VS Code turn 会推动同 instance 的所有 attached surface 跟着切 thread，这和“thread attach 互斥”目标相冲突。

所以这轮设计的重点不是“继续加 gateway”，而是：

- **把 instance claim / thread claim 变成 orchestrator 的第一性状态。**

## 6. 产品语义

### 6.1 全局可见

产品语义：

1. 所有在线 instance 全局可见。
2. 所有可见 thread 全局可见。
3. “全局可见”只代表可列出，不代表可同时占用。

这意味着：

1. `/list` 不做 app 级实例过滤。
2. `/use` 不做 app 级 thread 过滤。
3. UI 必须同时表达 `available` / `busy` / `current` 三类状态。

说明：

1. 这里的“全局可见 thread”是产品语义，不等于 V1 立即拥有一个完备的全局 thread catalog。
2. V1 允许 thread 展示仍主要依赖当前 attached instance 已观察到的 thread 快照。
3. 只要 `threadID` 相同，claim 语义就按同一个全局 thread 处理。

### 6.2 instance attach 互斥

产品语义：

1. 一个 instance 同一时刻只能被一个 remote surface 接管。
2. busy instance 仍显示在 `/list`，但不可 attach。
3. remote 之间不支持共享同一个 instance。

说明：

这里的互斥只约束 remote surface。
本地 VS Code 仍可能继续使用这个 instance。

### 6.3 thread attach 互斥

产品语义：

1. 一个 thread 同一时刻只能被一个 remote surface 选中并继续发送。
2. busy thread 仍显示在 `/use`，但不可直接切换。
3. thread 的 owner 是 surface，不是 instance。

说明：

1. 这个设计依赖一个明确假设：`threadID` 在同一台机器、同一个 `relayd` 的仲裁域内全局唯一；这与当前 Codex 的实际设计一致。
2. 产品运行约束也必须保持一致：同一台机器上只能有一个 `relayd` 负责 remote surface 的 claim 仲裁。
3. 如果未来发现 threadID 只在 instance 或 workspace 内唯一，或未来要支持单机多 `relayd` 并存，就必须把 claim key 升格成复合键。

### 6.4 多 app 资源与权限隔离

这是多 app 功能里必须成立的底线：

1. staged image 只属于当前 surface，不可跨 surface 复用。
2. text queue 只属于当前 surface，不可跨 surface 复用。
3. preview rewrite / preview state / preview 文档权限都必须带 `gatewayID` 维度。
4. 出站消息、卡片、文件、图片都必须按当前 `surfaceSessionID -> gatewayID` 路由。
5. `busy` 信息可以全局显示，但默认不暴露占用方的 chat/user 标识。

这意味着：

1. “全局可见”不等于“全局共享草稿态”。
2. 强踢 thread 时只能移动 thread claim，不能顺手把对方 surface 的其他状态搬过来。

### 6.5 follow_local 保留

产品语义：

1. 对 VS Code 共享实例保留 `follow_local`。
2. 观察到本地聚焦 thread 改变时，remote surface 尝试跟随。
3. 若目标 thread 已被别的 remote surface claim，则不自动抢占。

说明：

1. follow 是“尽量跟随”，不是“强一致跟随”。
2. 本地端是不可控外部输入。

### 6.6 `/stop`

产品语义不变，继续作为“中断并清队列”的硬操作：

1. 若当前有 active turn，发送 `turn.interrupt`。
2. 丢弃当前 surface 的 queued text。
3. 丢弃当前 surface 的 staged image。
4. 对丢弃项给出明确投影反馈。

### 6.7 `/detach`

产品语义：

1. `/detach` 废弃当前 surface 的所有草稿态。
2. 若没有 active remote turn，则立即释放 instance/thread claim。
3. 若有 active remote turn，则先 interrupt，再延迟释放 claim。

这意味着 `/detach` 不是简单的“清字段”，而是一个小状态机。

### 6.8 用户可见稳定状态

建议把用户可见稳定状态收敛成下面几类：

1. `detached`
   - 没有 instanceClaim，也没有 threadClaim
   - 用户下一步是 `/list` 或 `/newinstance`
2. `attached_pinned`
   - 有 instanceClaim，也有 threadClaim
   - 用户可以正常发消息、`/use`、`/stop`、`/detach`
3. `attached_unbound`
   - 有 instanceClaim，但没有 threadClaim
   - 常见来源是：attach 到一个默认 thread 已被占用的 instance、attach 时实例没有可直接接管的 thread、被强踢、或 thread 消失
   - 系统进入该状态时，必须立刻给 notice，并尽量主动发 `/use` / `/useall` 选择卡片
   - 在该状态下，普通文本、图片、请求响应、队列派发都应被拒绝并提示先选 thread
   - 用户下一步应当清晰可见：`/use`、`/follow_local`、等待 thread 可用、或 `/detach`
4. `attached_follow_local`
   - 有 instanceClaim
   - 线程目标由 follow 逻辑驱动
   - 若当前本地焦点可接管，则也持有对应 threadClaim
   - 若当前没有可接管 thread，必须明确告诉用户这是“等待跟随”；同时允许用户 `/use` 或 `/detach`
5. `detaching`
   - claim 尚未释放完，但用户侧已进入 abandon 流程

这意味着“已 attach instance 但没 attach thread”不再是异常，而是一个正式的受限稳定态；关键约束只有一条：

- **它必须始终伴随明确的下一步。**

## 7. Claim 模型

### 7.1 需要新增的核心结构

建议在 orchestrator 中新增：

1. `instanceClaims[instanceID] = record`
2. `threadClaims[threadID] = record`

record 至少建议带：

1. owner surface
2. owner gateway
3. owner actor
4. owner instance
5. claimed at
6. claim reason
7. mode: `pinned` / `follow_local`

推荐按 record 建，而不是只存裸字符串。原因：

1. 更方便做冲突提示与审计。
2. 更方便做 gateway disable / instance disconnect cleanup。
3. 更方便处理 follow 与强踢的差异化逻辑。

### 7.2 与 surface 状态的一致性约束

surface 仍保留：

1. `AttachedInstanceID`
2. `SelectedThreadID`
3. `RouteMode`

但必须满足：

1. `AttachedInstanceID != ""` 时，`instanceClaims` 中必须有当前 surface。
2. `SelectedThreadID != ""` 时，`threadClaims` 中必须有当前 surface。
3. 释放 claim 时必须同步清 surface 字段。
4. surface 被销毁、gateway 被禁用、instance 离线时必须清理 claim。
5. `threadClaims[threadID]` 的 owner surface 必须同时持有自己的 `instanceClaim`。

### 7.3 queue 是 surface-owned，但 thread-affine

这里建议把语义写死：

1. queue 的 owner 是 surface。
2. queue item 在 enqueue 时冻结 `threadID` / `cwd` / override。
3. 只要 queue item 已冻结到某个 thread，这个 thread claim 就不能被隐式转移。
4. thread 切换、强踢、follow 都不能偷偷搬运已有 queue。

这也是为什么本文推荐：

- `queued` 时禁止 `/use`
- `queued` 时禁止强踢

### 7.4 旧 prompt 点击必须重验

不论是 attach、`/use` 还是强踢，执行时都要重新检查：

1. attach 时目标 instance 是否仍然 free
2. 目标 thread 是否仍然 free，或是否仍处于允许强踢的 idle-claimed 状态
3. 目标 surface 是否出现了 running
4. 目标 surface 是否出现了冻结在该 thread 上的 queued item

## 8. 关键流程设计

### 8.1 `/list` 与 instance attach

`/list` 卡片建议展示：

1. display name
2. workspace
3. source
4. status:
   - `available`
   - `busy`
   - `current`

attach 流程建议收口成“先拿 instance，再尽量绑定 thread”：

1. 用户从 `/list` 选择一个 `available` instance。
2. 若当前 surface 已 attach 其他 instance，先要求显式 `/detach`，不做隐式换绑。
3. 只要 `instanceClaim` 可用，就先提交 instance attach。
4. attach 完成后，再根据当前 thread 情况决定进入 `attached_pinned` 或 `attached_unbound`。

这里的关键点是：

1. `instanceClaim` 是 attach 的主结果。
2. `threadClaim` 是 attach 后的附加绑定结果。
3. thread 拿不到时，attach 仍然成功，但系统必须把 surface 放到一个明确的受限状态里。

### 8.2 attach 时默认 thread 处理

attach 后，先解析候选默认 thread。

候选默认 thread 的来源建议保持：

1. `ObservedFocusedThreadID`
2. fallback 到 `ActiveThreadID`

然后按四种情况处理：

1. 候选默认 thread 存在，且未被 claim
   - 直接建立 `threadClaim`
   - surface 进入 `attached_pinned`
2. 候选默认 thread 存在，但已被别人 claim
   - attach 仍成功
   - surface 进入 `attached_unbound`
   - 系统立即 notice“实例已接管，但当前 thread 被其他会话占用”
   - 系统主动发 `/use` 或 `/useall` 选择卡片
3. 候选默认 thread 不存在，但当前 instance 还有其他可见 thread
   - attach 仍成功
   - surface 进入 `attached_unbound`
   - 系统主动发 `/use` 或 `/useall` 选择卡片
4. 当前 instance 没有任何可见 thread
   - attach 仍成功
   - surface 进入 `attached_unbound`
   - notice 明确提示：当前没有可用 thread，请等待 VS Code 进入 thread，或稍后 `/use`，或 `/detach`

这版的核心调整是：

1. attach 成功与否只看 instance。
2. thread 无法绑定不再导致 attach 回滚。
3. 但 thread 无法绑定时，系统必须立即把用户引导到下一步，而不是静默卡住。

### 8.3 `attached_unbound` 的语义

`attached_unbound` 是一个正式状态，不是异常态。

它的定义是：

1. 已持有 `instanceClaim`
2. 没有 `threadClaim`
3. 当前 surface 不能产生新的 remote work

在这个状态下：

1. 普通文本消息拒绝，并提示先 `/use`
2. 图片消息拒绝，并提示先 `/use`
3. request 响应拒绝，并提示先 `/use`
4. 不允许把新的 queue item 冻结到“空 thread”
5. 允许 `/use`、`/useall`、`/follow_local`、`/detach`、`/status`

我建议系统在进入这个状态时始终做两件事：

1. 发 notice 解释为什么当前不能继续发送
2. 如果有可见 thread，就立刻发一张 `/use` 选择卡片

### 8.4 `/use` 的 V1 语义

`/use` 建议收敛成下面的语义：

1. `available` thread 可直接切换并建立 `threadClaim`
2. `busy` thread 仍然显示，但不能直接接管
3. 若用户显式选择一个 `busy` thread，再进入强踢判断流程
4. 若当前 surface 处于 `attached_unbound`，`/use` 是恢复到可发送状态的主入口

这样 attach 主流程和 kick 主流程就分开了。

### 8.5 attach 结果矩阵

为了避免边界语义继续漂移，建议把 attach 的结果直接写成矩阵：

| 当前状态 | attach 目标 | 目标 thread 状态 | 结果状态 | 用户下一步 |
| --- | --- | --- | --- | --- |
| `detached` | available instance | default thread available | `attached_pinned` | 直接发送消息 |
| `detached` | available instance | default thread busy | `attached_unbound` | 看系统发出的 `/use` 卡片，切到其他 thread 或等待 |
| `detached` | available instance | no default thread, but has other visible thread | `attached_unbound` | 通过 `/use` 选择 thread |
| `detached` | available instance | no visible thread | `attached_unbound` | 等待 thread 出现、稍后 `/use`、或 `/detach` |
| `attached_pinned` | another instance | 任意 | 拒绝 | 先 `/detach` 再 attach |

这张表背后的原则是：

- **attach 先解决 instance 所有权；thread 所有权单独收口，但 thread 未绑定时必须让用户知道下一步。**

## 9. 强踢设计

### 9.1 强踢触发点

当前最自然的强踢触发点不再是 attach，而是用户在 `/use` 里显式选择一个 `busy` thread。

建议交互：

1. 用户已经 attach 了目标 instance
2. 用户在 `/use` 卡片中选择一个 `busy` thread
3. 系统检查该 thread 当前是 idle-claimed、queued 还是 running
4. 若允许强踢，则再发一张确认卡片
   - `取消`
   - `强踢并占用`

### 9.2 我建议的 V1 强踢规则

我建议 V1 把强踢定义得保守一点，而且只看“目标 thread 上的真实占用状态”。

| 目标 thread 上的占用状态 | 是否允许强踢 | 处理方式 |
| --- | --- | --- |
| idle | 允许 | 用户确认后，转移 `threadClaim`，当前 surface 进入 `attached_pinned` |
| queued | 不允许 | 当前 surface 保持原状态；提示等待或让对方先清队列 |
| running | 不允许 | 当前 surface 保持原状态；提示该 thread 正在执行 |

这里的 `queued` 需要精确定义：

1. 不是简单看“对方 surface 还有没有任何 queue”。
2. 而是看“对方是否还有冻结在这个目标 thread 上的 queued work”。
3. 但在本文推荐的最终规则下，surface 有 queued/running 时本就不能切 thread，所以大多数正常状态下二者会一致。

原因：

1. running 时强踢会直接碰到 active turn 尾流归属问题。
2. queued 时强踢会碰到“要不要顺手丢掉对方 queue”的复杂语义。
3. staged images 还没绑定到 prompt，允许保留即可，只要给受害 surface 明确 notice。

### 9.3 强踢成功后的受害 surface 处理

若强踢成功，被踢 surface 不应被整体 detach。

建议处理：

1. 释放它的 `SelectedThreadID`
2. 若原来是 `pinned`，转成 `attached_unbound`
3. 若原来是 `follow_local`，保留 `follow_local`，但清空当前 thread claim
4. 保留它的 instance attach
5. 保留它的 staged images
6. 保留与其他 thread 无关的状态
7. 立即给 notice，说明当前 thread 已被其他会话接管
8. 如果当前 instance 还有其他 `available` thread，自动弹出 thread 选择 prompt
9. 如果没有其他 `available` thread，notice 里明确告诉用户下一步是等待、`/use`、或 `/detach`

如果它原本是 `follow_local`：

1. 模式可继续保留为 `follow_local`
2. 但当前不自动重新抢占该 busy thread
3. 需要等下次本地焦点变化，或 busy 状态消失后再评估

这里我建议把“被踢后进入 `attached_unbound`”解释成一种有明确后续动作的受控状态，而不是无提示悬空状态：

1. 系统必须立刻投影 notice
2. 系统最好立刻补一个 thread 选择 prompt
3. 若没有可选 thread，也要告诉用户只能等待或 `/detach`

### 9.4 为什么暂不支持“强踢并顺手扔掉对方 queue”

这正是你提到的犹豫点。我建议 V1 明确不做。

如果允许强踢同时清掉对方 queue，会立刻引入这几个问题：

1. 对方 queue 里可能有多条已冻结到该 thread 的 prompt，需要统一废弃。
2. 对方 staged image 可能是下一条 prompt 的一部分，语义会突然断裂。
3. 用户会更难理解“我只是抢一个 thread，为什么你把别人的队列也清了”。
4. 这会把“强踢 thread”升级成“跨 surface 执行一次近似 `/stop`”。
5. 一旦 notice 投影失败，受害方会直接失去上下文。

因此当前建议是：

- **V1 不允许在对方有 queued work 时强踢。**

如果未来一定要支持，也建议把它定义成一个单独的 destructive admin 动作，而不是默认强踢语义。

### 9.5 强踢动作必须二次校验

用户点击“强踢并占用”时，必须重新检查：

1. 目标 thread 是否已经 free
2. 目标 thread 是否已经切到 running
3. 目标 thread 是否出现了新的 queued item
4. 当前 surface 是否仍然 attach 在这个 instance 上

任何一项不满足，都应该返回最新 notice，而不是按旧 prompt 强行执行。

若重验失败：

1. 当前 surface 保持原状态不变
2. 当前 surface 若原本是 `attached_unbound`，仍保持 `attached_unbound`
3. 当前 prompt 失效

## 10. Thread 切换与 queue 规则

### 10.1 running 时禁止切换

这点已经基本拍定：

- surface 有 active running turn 时，禁止 `/use` 切换 thread。

### 10.2 queued 时也建议禁止切换

我建议 queued 也一起禁止。

原因：

1. queued item 已冻结 `threadID`。
2. 若切走新 thread，就要决定是否释放旧 thread claim。
3. 若不释放旧 thread claim，一个 surface 可能同时占多个 thread。
4. 若释放旧 claim，别人就能接手一个仍有本 surface queued work 的 thread。

所以当前建议规则是：

- **只有在当前 surface 没有 active/queued remote work 时，才允许 `/use`。**

### 10.3 queue 不能被 thread 切换隐式搬家

建议明确禁止这三种隐式语义：

1. thread 切换时自动把 queued item 改绑到新 thread
2. thread 切换时自动释放旧 thread claim，但旧 queue 还留在旧 thread
3. 强踢时自动帮受害 surface 清 queue

因为这三种都会让 queue 所属关系变得不可预测。

### 10.4 用户如果确实想切 thread 怎么办

建议通过这三种路径解决：

1. 等当前 queue 清空
2. `/stop`
3. `/detach`

不要让 thread 切换本身隐式地做 queue discard。

## 11. `/detach` 详细语义

### 11.1 无 active turn

若 surface 没有 active remote turn：

1. 丢弃 queued text
2. 丢弃 staged image
3. 清 request capture / pending request
4. 释放 thread claim
5. 释放 instance claim
6. 进入 detached

### 11.2 有 active turn

若 surface 有 active remote turn：

1. 先丢弃未发送的 queued text 与 staged image
2. 对 active turn 发送 `turn.interrupt`
3. surface 进入 `detaching` / `abandoning` 状态
4. active turn 结束或超时后，才释放 instance/thread claim

### 11.3 为什么要延迟释放 claim

否则会有两个高风险问题：

1. 旧 turn 尾流事件可能找不到原 surface
2. 新 owner 若马上 attach 同一 instance，旧 turn 的回复可能误投给新 owner

所以这里我建议宁可把 detach 做成“稍慢一点但正确”，也不要立刻清 claim。

### 11.4 detach 只处理当前 surface，不碰别人

`/detach` 的边界也要写清楚：

1. 只废弃当前 surface 的 queue / image / pending request / prompt override。
2. 只释放当前 surface 持有的 claim。
3. 不清理别的 surface 的 queue。
4. 不因为当前 surface detach 就顺手把同 instance 的其他 remote surface 改成 `attached_unbound`。

## 12. Follow 细化

### 12.0 当前代码里的真实行为

这里需要先把现状说清楚，避免设计时把“当前代码行为”和“目标行为”混在一起。

当前代码里：

1. 显式进入 `follow_local` 的入口只有飞书 `/follow`。
2. `/follow` 当前只会清空 `SelectedThreadID`，并把 `RouteMode` 设成 `follow_local`。
3. `freezeRoute()` 只会在“下一条远端 prompt 真正冻结路由”时读取 `ObservedFocusedThreadID`。
4. `local.interaction.observed` 只更新 `ObservedFocusedThreadID`，不会立刻切 surface。
5. 但一旦本地 VS Code 真正开始一个 turn，当前代码会把同 instance 下所有 attached surface 都直接 `bindSurfaceToThread()`，并改成 `pinned`。
6. 远端 turn 真正跑起来，或文本输出落到某个 thread 时，也会把 surface 直接改成 `pinned`。

这意味着：

1. 当前的 `follow_local` 更像“下一次发送前的路由模式”。
2. 它不是一个稳定持续的“跟随态”。
3. 现在其实不止 `/follow` 会影响 thread 选择。

### 12.1 为什么这会在多 remote app 语义下变乱

在“instance remote 独占、thread remote 独占、本地 VS Code 不受控”的前提下，当前行为有几个明显问题：

1. `follow_local` 和 `pinned` 的语义边界不清。
2. 本地 turn start 会把所有 attached surface 强行 pin 到本地 thread，这不应该发生在普通 `attached_pinned` 或 `attached_unbound` 上。
3. 本地输出当前按“找一个 attached surface”回落，这在多 remote surface 时代会有歧义。
4. 若 follow 目标 thread 已被别的 remote claim，当前代码没有一个明确的“等待跟随但不抢占”的状态机。
5. follow_local 一旦被本地 turn 或远端输出改成 `pinned`，用户会误以为系统还在跟随，实际上已经不是了。

### 12.2 我建议的核心收口

我建议把逻辑收口成一句话：

- **只有显式 `follow_local` surface 才会响应本地 thread 变化；其他 attached surface 不应该被本地 VS Code 自动改绑。**

展开后是：

1. `/follow` 仍然是 follow 模式的唯一显式入口。
2. `attached_pinned` 只响应用户 `/use` 或远端自身发送结果，不响应本地 thread 切换。
3. `attached_unbound` 不响应本地 thread 切换，除非用户显式执行 `/follow`。
4. `attached_follow_local` 才会根据本地 `ObservedFocusedThreadID` 去尝试变更当前 threadClaim。

### 12.3 follow_local 的建议状态机

我建议 follow_local 用下面的规则：

1. 用户执行 `/follow`
   - 清空显式 pin
   - `RouteMode = follow_local`
   - 立即按当前 `ObservedFocusedThreadID` 尝试跟随一次
2. 若当前没有 `ObservedFocusedThreadID`
   - 保持 `attached_follow_local`
   - 不持有 threadClaim
   - 给 notice：当前没有本地焦点 thread，等待 VS Code 进入一个 thread
3. 若目标 thread free
   - 建立 threadClaim
   - UI 显示“正在跟随 <thread>”
4. 若目标 thread 被别的 remote claim
   - 不自动抢占
   - 释放自己之前持有的 follow threadClaim
   - 保持 `attached_follow_local`
   - 给 notice：本地已切到一个被其他 remote 占用的 thread，当前暂停跟随

### 12.4 什么时候重新评估 follow

建议只有这几个时机触发“重评估 follow”：

1. 用户刚执行 `/follow`
2. 收到 `local.interaction.observed`
3. 收到本地 `turn.started`
4. 当前 remote queue 清空
5. 被占用的目标 thread 释放

除此之外，不要让渲染输出或其他副作用偷偷改变 follow 状态。

### 12.5 idle 时

surface 空闲且处于 `follow_local` 时：

1. 本地焦点 thread 未被 claim
   - 自动切过去
2. 本地焦点 thread 已被 claim
   - 不自动切换
   - 保持 follow 模式

### 12.6 busy 时

surface 仍有 queued/running remote work 时：

1. 不要立即改变 selected thread claim
2. 只记录最新观察到的本地焦点
3. queue 清空后再评估是否切换

这能避免 UI 已显示新 thread，但真正执行还在旧 thread 上。

### 12.7 本地 turn 与本地输出应该怎么路由

这里我建议把“本地 VS Code 的活动”和“surface 绑定变化”分开：

1. `local.interaction.observed`
   - 只更新 `ObservedFocusedThreadID`
   - 不直接改 surface thread
2. `local turn started`
   - 不再把所有 attached surface 强行改成 `pinned`
   - 只对 `attached_follow_local` surface 触发一次 follow 重评估
3. 本地 UI turn 的输出 / request / error
   - 只路由给“当前确实应该看到这个 thread”的 surface

建议的路由规则是：

1. 若该 turn 是远端 surface 自己发起的，继续按 remote binding 路由
2. 若该 turn 是本地 UI 发起的：
   - `attached_follow_local` 且当前已成功跟随到该 thread 的 surface 可以看到
   - `attached_pinned` 且 `SelectedThreadID == turn.ThreadID` 的 surface 可以看到
   - `attached_unbound` 不应看到
3. 不要再用“找一个 attached surface 就回退过去”的宽松策略

### 12.8 本地 VS Code 竞争仍是接受边界

这点需要明确写成限制，而不是隐含期望：

1. 本地 VS Code 仍可能切到一个 remote 已 claim 的 thread。
2. 本地 VS Code 仍可能在该 thread 上发起 turn。
3. V1 不试图阻止它，只要求 remote 的 claim 与投影不互相串。

## 13. 其他生命周期问题

### 13.1 gateway disable / delete

若某个 gateway 被禁用或删除：

1. 遍历该 gateway 下所有 attached surface
2. 对它们执行等价于 `/detach` 的 abandon 流程
3. 确保 instance/thread claim 被释放

### 13.2 instance disconnect

若 instance 离线：

1. 清理它的 `instanceClaim`
2. 清理相关 `threadClaim`
3. 通知对应 surface 当前接管失效

### 13.3 headless

headless 也应走同一套 claim 模型：

1. 创建出来后先拿 `instanceClaim`
2. 若恢复 thread 可用，再拿 `threadClaim`
3. 若恢复 thread busy、或当前没有可恢复 thread，则进入 `attached_unbound`
4. 仍然通过 `/use`、`/follow_local` 或等待后续 thread 可用来恢复

### 13.4 当前 thread 消失或变不可见

如果一个 surface 当前持有的 thread 后来被归档、删除、隐藏，或者不再可见：

1. 释放对应 `threadClaim`
2. 若当前是 `attached_pinned`，转成 `attached_unbound`
3. 若当前是 `attached_follow_local`，保留 follow 模式并进入等待跟随
4. 立即给 notice，说明原 thread 不再可用
5. 如果还有其他可见 thread，主动发 `/use` 选择卡片

## 14. 仍需显式接受的限制

### 14.1 V1 不做“全局 thread 真相表”

V1 可以接受：

1. thread claim 是全局的
2. 但 thread 的可见性仍主要来自各 instance 的线程快照
3. 某个 thread 可能暂时还没被当前 instance 列出来，需要刷新或等待同步

### 14.2 V1 不做“带清队列的强踢”

V1 可以接受：

1. idle thread 允许强踢
2. queued / running thread 不允许强踢
3. 真正 destructive 的跨 surface 抢占动作以后再单独设计

### 14.3 V1 不解决本地端与 remote 的强一致竞争

V1 只保证：

1. remote surface 之间的 owner 明确
2. 多 app 的消息、图片、preview、notice 不串
3. 本地端仍是外部不受控输入

## 15. 测试建议

### 15.1 多 app 主链路

1. 两个 gateway，同一个 `chatID` 或同一个 `actorUserID`
   - 断言 surface 仍不同
   - queue / image / preview 不串
   - preview 授权与上传目标不串
2. 两个 gateway，各自 attach 不同 instance
   - 断言出站完全隔离

### 15.2 claim 主链路

1. `/list` 全局可见 + busy 标记正确
2. attach instance 后 `instanceClaim` 建立
3. attach 后默认 thread busy 时进入 `attached_unbound`
4. 进入 `attached_unbound` 后会自动收到 notice 与 `/use` 提示
5. `attached_unbound` 下普通文本 / 图片 / request 响应被拒绝
6. 当前 selected thread 后续消失时会释放 `threadClaim` 并退回 `attached_unbound`

### 15.3 强踢主链路

1. `/use` 选择 idle-claimed thread 时，会出现强踢确认
2. 目标 surface running 时，强踢拒绝
3. 目标 surface queued 时，强踢拒绝
4. 强踢成功后，对方 surface 变 `attached_unbound` 或保持 `follow_local`，但不整体 detach
5. 旧 prompt 过期或状态变化后，点击强踢会重新校验并给出新 notice

### 15.4 detach 主链路

1. 无 active turn 时立即释放 claim
2. 有 active turn 时先 interrupt，再延迟释放 claim
3. old turn completion 不会误投给新 owner

### 15.5 follow 主链路

1. idle + free thread 会自动跟随
2. idle + busy thread 不会自动抢占
3. 跟随目标当前不可接管时会留在等待态，而不是错误切到空 thread
4. busy 时不会即时改 claim
5. 本地端切 thread 不会导致别的 remote surface 自动被改绑

## 16. 建议的实现顺序

### 16.1 第一步

先把 claim 模型立住：

1. `instanceClaims`
2. `threadClaims`
3. `/list` busy 展示
4. `/use` busy 展示

### 16.2 第二步

再做 attach / 强踢 / 切换规则：

1. attach 后默认 thread 冲突进入 `attached_unbound`
2. `attached_unbound` 的 notice / 自动 `/use` 提示 / 动作限制
3. `/use` 里的强踢卡片与动作
4. busy 时禁止 thread 切换
5. 禁止同 surface 在未 detach 时直接换绑新 instance

### 16.3 第三步

再做 detach / active-turn abandon：

1. interrupt
2. detaching 状态
3. claim 延迟释放

### 16.4 第四步

最后做 follow 与 admin 生命周期兜底：

1. claim-aware follow
2. gateway disable/delete cleanup
3. instance disconnect cleanup
4. headless 对齐 claim 模型

## 17. 这轮讨论后我建议先确认的点

1. 我建议“强踢只在目标 thread idle 时允许；目标 thread 有 queued 或 running 都不允许”，你是否同意？
2. 我建议 queued 时也禁止 `/use`，并且 thread 切换不隐式清 queue，你是否同意？
3. 我建议 attach 成功后如果拿不到 thread，就进入 `attached_unbound`，并且除 `/use`、`/follow_local`、`/detach`、`/status` 外都不能继续工作，你是否同意？
4. 我建议进入 `attached_unbound` 时默认主动发一次 `/use` 卡片；如果没有可见 thread，就明确提示等待或 `/detach`，你是否同意？
5. 我建议 V1 先要求“切新 instance 前先 `/detach`”，不做隐式换绑，你是否同意？
6. 我建议被强踢的一方统一进入 `attached_unbound` 或保留 `follow_local`，并立刻收到恢复提示，而不是静默悬空，你是否同意？
7. 我建议强踢提示默认不显示占用方 chat/user 身份，只显示“已被其他会话占用”，你是否同意？

如果这几条确认下来，后续实现边界会稳定很多。
