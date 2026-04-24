# 上游可重试失败自动恢复设计（草稿）

> Type: `draft`
> Updated: `2026-04-24`
> Summary: 固化 turn 因上游推理异常中断后的自动恢复方案，明确它与 `autowhip` 的拆分边界，并记录当前讨论对 `/stop`、Codex 内部 retry 与错误归因的判断。

## 0. 文档定位

这份文档只回答一件事：

- 当一个 turn 因上游推理侧的可重试问题中断时，系统是否应该自动继续，以及这件事应该怎样设计，才不会和现有 `autowhip`、`/stop`、Codex 自身 retry 语义混在一起。

它不是当前实现说明，也不是最终 PRD。

它的用途是把这轮讨论中已经澄清的事实、风险和取舍先固定下来，避免后面继续讨论时又回到“是不是直接改一下现有 `autowhip` backoff 就行”的局部 patch 思路。

## 1. 当前需求

当前希望增加一个新的 surface 级执行策略：

1. 默认关闭，显式开启后才生效。
2. 当 turn 因上游推理侧的可重试问题中断时，系统自动再发一次“继续”类输入。
3. 这次自动继续的语义，应接近“用户手动补发一条继续”，而不是“恢复原 turn id”。
4. 需要覆盖的失败面，优先是推理上游问题：
   - 上游明确报 `429`、`502`、`503` 等可重试错误；
   - 上游流自己断开，但 turn 最终仍以 retryable interruption 收尾，例如 `responseStreamDisconnected`。
5. 不希望误触发的场景：
   - Codex 自己在 turn 内做中途 retry，最终 turn 正常完成；
   - 用户主动 `/stop`；
   - 本地 relay / wrapper / daemon 自身的链路断开；
   - 普通 non-retryable failure。

## 2. 当前实现事实

### 2.1 `autowhip` 现在已经承接了两条不同语义

当前 surface 上的 `AutoContinueRuntimeRecord` 同时承接两条完全不同的触发链：

1. `incomplete_stop`
   - turn 正常结束，但最终文本不包含“老板不要再打我了，真的没有事情干了”这句停止口令；
   - 系统认为 Codex 可能“没把活干完”，于是补打一轮。
2. `retryable_failure`
   - turn 结束时挂着 `problem.Retryable=true`；
   - 系统认为上游不稳定，于是按 backoff 自动重试。

这条逻辑当前在这些位置：

- `internal/core/orchestrator/service_autocontinue.go`
- `internal/core/orchestrator/service_queue.go`
- `internal/core/state/types.go`

当前 backoff：

- `incomplete_stop`: `3s -> 10s -> 30s`
- `retryable_failure`: `10s -> 30s -> 90s -> 300s`

### 2.2 Codex turn 内部 retry 本身不会直接误触发当前机制

当前 translator 对 turn-bound `error` 的处理，不是收到错误就立刻判失败，而是：

1. 先把 `problem` 挂到 `pendingTurnProblems[turnID]`；
2. 等 `turn/completed` 到来时，再把该 `problem` 贴到终态 turn event 上；
3. 如果最终 `status == completed`，则这个挂起的 `problem` 会被丢掉，不会作为失败上浮。

这意味着：

- Codex 自己在 turn 内部做 retry，只要最后 turn 成功完成，当前不会触发自动恢复。

相关代码与测试：

- `internal/adapter/codex/translator_observe_server.go`
- `internal/adapter/codex/translator_internal_helper_test.go`

### 2.3 `/stop` 现在只有局部抑制，没有完整的“用户中断”终止原因

当前 `/stop` 会：

1. 向 Codex 发 `turn/interrupt`；
2. 清掉排队或暂存输入；
3. 对现有 `autowhip` 设置一次性的 `SuppressOnce`，避免本轮 turn 收尾时立刻再次自动续跑。

这意味着当前已有一层局部保护：

- 就算 turn 之后以 `interrupted` 结束，现有 `autowhip` 也会吞掉这一次 schedule。

但这层保护的局限很明显：

1. 它只是 `autowhip` 内部的局部 patch；
2. 它不是 turn 级终止原因分类；
3. 它没有在 active turn binding 上显式记录“用户主动请求过中断”。

相关代码与测试：

- `internal/core/orchestrator/service_surface_actions.go`
- `internal/core/orchestrator/service_autocontinue.go`
- `internal/core/orchestrator/service_config_prompt_test.go`

### 2.4 当前代码仍然存在“旧 retryable problem 粘到 stop 终态上”的设计风险

因为当前 `pendingTurnProblems` 是按 `turnID` 暂存，而 `/stop` 并不会：

- 清掉该 turn 上已有的 retryable problem；
- 或给该 turn 增加一个高优先级的 `user_interrupt_requested` 标记；

所以存在这样一种混淆路径：

1. turn 运行中先收到一次 retryable `problem`；
2. 用户后来点了 `/stop`；
3. 终态 `turn/completed(status=interrupted)` 到来时，旧的 retryable `problem` 仍被贴到这个终态上。

从代码上看，这类混淆现在没有被显式消解。

这也是为什么“手动 stop 后为什么还能看到 `stream disconnected before completion`”目前不能只靠读代码就精确判定来源：

1. 它可能真的是上游对 interrupt 的收尾语义；
2. 也可能是之前挂着的 retryable problem 在终态时被一起带上来了。

## 3. 当前实现为什么不适合直接补丁

### 3.1 语义已经混杂

`autowhip` 现在同时包含：

- “没干完继续催”
- “上游失败自动恢复”

这两条链的目标、提示文案、prompt 语义、backoff 节奏都不同。

如果继续在 `autowhip` 上补：

- 开关语义会越来越模糊；
- 状态字段会继续混杂；
- 用户也很难理解自己开的是“催活”还是“错误恢复”。

### 3.2 backoff 与 prompt 都不匹配当前需求

当前 retryable failure 的重试节奏过于保守：

- 第一轮要等 `10s`
- 第二轮还要等 `30s`

而用户这次明确希望的是：

1. 前两次尽量立即尝试；
2. 第三次之后再开始秒级 backoff；
3. prompt 更接近“继续”，而不是 `autowhip` 的专用补打文案。

### 3.3 `/stop` 保护现在只是 feature-local patch

当前的 `SuppressOnce` 只是在现有 `autowhip` 内部勉强避免“刚 stop 又自动继续”。

如果后面新功能直接复用 `problem.Retryable` 触发，而不补一层统一的终止原因分类，那么：

- 同样的问题还会换个名字再出现一次。

### 3.4 名字和字段也在误导实现

只要继续复用这些名字，后面的实现就很容易又滑回原来的混合语义：

- `AutoContinueRuntimeRecord`
- `AutoContinueReasonRetryableFailure`
- `RetryableFailureCount`
- `AutoWhip` notice 文案

这些命名都在暗示“错误恢复只是 autowhip 的一个分支”，这与本次需求的目标相反。

## 4. 这轮讨论的结论

### 4.1 `autowhip` 与“上游失败自动恢复”必须拆开

这是本轮最核心的结论。

`autowhip` 只保留“正常结束后的继续催活”语义。

上游失败自动恢复应成为独立能力，至少在实现层面独立：

- 独立开关
- 独立状态
- 独立 backoff
- 独立提示文案
- 独立终止原因分类

### 4.2 Codex 自己的中途 retry 不应触发新能力

这点当前实现其实已经基本满足：

- 只有 turn 终态失败时，才有资格进入自动恢复判断；
- turn 内部 retry 且最终成功，不应进入该路径。

后续新方案必须保留这个性质，不能退化成“看到 retryable error 就 schedule”。

### 4.3 `/stop` 必须升级成显式终止原因，而不是继续靠一次性 suppress

新方案不能仅依赖 `SuppressOnce`。

正确做法应是：

1. `/stop` 发出时，对当前 active turn 显式记账；
2. turn 收尾分类时，把“用户主动中断”作为高优先级终止原因；
3. 一旦命中 `user_interrupted`，即使终态上还带着 retryable problem，也不能触发自动恢复。

## 5. 设计方向

### 5.1 新能力：独立的“上游失败自动恢复”模式

建议把这次新能力定义成独立 surface 级开关，而不是 `autowhip` 的子选项。

工作方式：

1. 用户显式开启；
2. turn 终态被分类为 `upstream_retryable_failure` 时，系统按该模式自己的节奏 enqueue 一条恢复输入；
3. 恢复输入沿用原 queue item 的 route、thread、cwd、frozen override 与 reply anchor；
4. 只在 turn 真正结束后触发，不对 running turn 内的中途 error 直接动作。

用户可见层面，它可以挂在现有参数/配置区，但语义上不属于 model/provider 参数，而是 orchestrator 的执行恢复策略。

### 5.2 必须新增 turn 终止原因分类

建议引入一层明确的 turn terminal classification。

最少应区分：

1. `completed`
2. `user_interrupted`
3. `upstream_retryable_failure`
4. `nonretryable_failure`
5. `transport_lost`

建议的优先级：

1. 如果终态 `status == completed`，直接判 `completed`
2. 否则如果当前 turn 已记录 `interrupt_requested`，判 `user_interrupted`
3. 否则如果 `problem.Retryable == true`，且来源属于 Codex 上游推理侧，判 `upstream_retryable_failure`
4. 否则如果是 relay / wrapper / instance transport 断链，判 `transport_lost`
5. 其他都落 `nonretryable_failure`

这里的关键点不是枚举名，而是这层分类必须成为新的唯一判定入口，不能继续直接用 `problem.Retryable` 驱动产品行为。

### 5.3 `/stop` 要把“中断意图”记到 turn binding 上

建议在 active remote turn 运行时记录中增加至少这些信息：

- `InterruptRequested bool`
- `InterruptRequestedAt time.Time`

记录位置应该靠近 remote turn binding，而不是只放在 surface feature overlay 上。

原因：

1. 它是 turn 终止原因判断的输入；
2. 它不只服务 `autowhip`；
3. 它应该跟 turn 生命周期一起清理，而不是跟某个 feature 的 runtime overlay 耦合。

### 5.4 新能力的 backoff 应比现有 `retryable_failure` 更激进

建议 v1 使用更接近人工使用习惯的节奏：

1. 第 1 次：立即
2. 第 2 次：立即
3. 第 3 次：`2s`
4. 第 4 次：`5s`
5. 第 5 次：`10s`
6. 之后停止

这里的“立即”不要求在当前 call stack 里递归重入；只要通过现有 tick/queue 机制在下一拍发出即可，用户感知已经足够接近“立即”。

### 5.5 新能力的恢复 prompt 不能复用 `autowhip` 文案

`autowhip` 的专用 prompt 是“催活”语义，不适合错误恢复。

新能力应使用中性恢复 prompt，例如：

> 上一次响应因上游推理中断，请从中断处继续完成当前任务；如果其实已经完成，请直接说明结果。

最终文案可以再单独定，但原则已经明确：

- 不复用 `autowhip` 停止口令体系；
- 不把“老板不要再打我了”这一套混进错误恢复。

### 5.6 v1 范围建议

v1 只处理 turn 级、Codex 上游可重试失败，不扩大到所有错误面。

v1 建议纳入：

1. 终态 turn 挂着 `problem.Retryable=true`；
2. 已知上游断流类终态，例如 `responseStreamDisconnected`；
3. 显式的上游 `429` / `502` / `503` 这类 retryable 问题，只要最终作为 turn 失败收尾。

v1 暂不纳入：

1. relay transport degraded
2. wrapper / daemon 自身写管道失败
3. 实例 disconnect / detach
4. dispatch rejected

原因很简单：

- 这些错误面虽然也可能“能恢复”，但它们不属于“上游推理失败自动恢复”的单一问题域；
- 现在先把 turn 级上游失败做干净，比把所有错误都往一个恢复桶里塞更稳。

## 6. 与 `autowhip` 的关系

这次讨论后，对 `autowhip` 的方向建议如下。

### 6.1 认同：`autowhip` 不应再处理 Codex 自身错误

对下面这个想法，本文结论是：**同意**。

> 这版方案实施后，如果效果不行，就把自动编打功能中处理 Codex 自身错误的那一条链路彻底摘掉。

更激进一点说：

- 不是“如果效果不行再摘”，而是新能力一旦接管错误恢复语义，`autowhip` 中的 `retryable_failure` 路径就不应长期保留。

原因：

1. 两套功能同时吃同一类失败，会制造新的耦合和歧义；
2. 保留双路径只会让后续排错更难；
3. 这种“新旧都留着兜底”的过渡策略，本仓库历史上已经多次证明容易留下半死不活的 legacy 行为。

### 6.2 认同：`autowhip` 只处理正常结束 turn

对下面这个想法，本文结论也是：**同意**。

> 自动编打功能只处理正常结束的 turn 情况。

更精确的说法应是：

- `autowhip` 只在 `turn terminal cause == completed` 时参与评估；
- `interrupted`、`failed`、`transport_lost` 都不再进入 `autowhip` 判断。

这样分完之后：

- `autowhip` 只剩“正常结束后要不要再催一轮”的语义；
- 错误恢复则完全由新能力负责。

### 6.3 认同：原来的错误 turn 处理链路不应保留产品语义

对下面这个想法，本文结论是：**同意，但建议复用基础设施，不复用旧产品语义**。

> 原来的处理错误 turn 的功能就不要了，相关代码可以选择删除或改造后复用。

建议的处理方式：

1. 可以复用的部分：
   - `turn.completed -> schedule pending runtime -> tick dispatch -> enqueue special queue item`
   - reply anchor 复用
   - frozen route / override 复用
2. 应删除或重命名的部分：
   - `AutoContinueReasonRetryableFailure`
   - `RetryableFailureCount`
   - `AutoWhip` 错误提示文案
   - 任何把“错误恢复”解释成 `autowhip` 子能力的命名

换句话说：

- 可以复用调度骨架；
- 不应复用旧 feature 语义和命名。

## 7. 建议的数据与状态迁移方向

为了避免继续在 `AutoContinueRuntimeRecord` 上打补丁，建议最终形态至少做到：

1. `AutoContinueRuntimeRecord`
   - 只保留 `autowhip` 自己需要的运行时状态；
   - 去掉 `retryable_failure` 相关字段与计数。
2. 新增独立的恢复 runtime record
   - 承接“上游失败自动恢复”的 enable、pending、attempt、reply anchor 等状态。
3. active remote turn binding
   - 新增 `interrupt_requested` 类字段，用于 turn 终止原因分类。

如果这一步不做，只把现有 `AutoContinueRuntimeRecord` 再加几个字段，后面大概率还会再次回到“到底这算 autowhip 还是算错误恢复”的混乱状态。

## 8. 观察性要求

这类功能如果没有观察字段，很难长期维护。

建议在落地时至少补这些日志/调试面：

1. `/stop` 发出时：
   - surface / instance / thread / turn
   - 是否成功命中 active remote turn
2. turn 终态分类时：
   - terminal cause
   - status
   - problem.code
   - problem.retryable
   - interrupt_requested
3. 恢复调度时：
   - attempt count
   - due at
   - chosen prompt kind
4. 明确记录“为什么没有恢复”：
   - completed
   - user interrupted
   - non-retryable
   - gate blocked
   - max attempts exhausted

没有这些字段，后面再碰到“为什么 stop 后它又继续了”或者“为什么明明是 upstream 断流却没恢复”，排查成本会非常高。

## 9. 当前建议的执行顺序

如果后续要进入实现，建议顺序如下：

1. 先补 turn terminal cause classification
2. 再补 `/stop` 的 explicit interrupt marker
3. 再把上游失败自动恢复从 `autowhip` 中拆出来
4. 最后再清理 `autowhip` 中的 `retryable_failure` 旧链路

不要反过来先改 backoff 或 prompt。

如果分类层没先建立，再漂亮的 backoff 都只是更激进的 patch。
