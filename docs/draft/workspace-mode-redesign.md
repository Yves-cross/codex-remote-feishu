# Workspace 模型重构设计

> Type: `draft`
> Updated: `2026-04-09`
> Summary: 按 `#67` 最新评论重构为“surface 显式记忆 mode、normal mode 全面 workspace-first、`/list` 按 mode 分流”的新设计母稿。

## 1. 文档目标

这份文档服务于 `#67 Workspace 概念化`，描述的是**下一阶段要落地的产品模型**，不是当前代码已实现行为。

这次重写的原因很直接：

1. 上一版把 `normal mode` / `vscode mode` 写成了路径依赖，而不是 surface 自己显式持有的产品状态。
2. 上一版把 `normal mode /list` 错写成禁用，但最新结论是它应当成为 workspace 列表与切换入口。
3. 上一版 follow-up issue 的拆分依赖这些错误前提，因此当前不能再把 `#68` 到 `#73` 当成已定型实现计划。

本轮目标是先把新的产品骨架收敛清楚，再决定如何重新拆实现。

## 2. 最新修正前提

根据 `#67` 最新评论，本轮 redesign 先以以下前提为准：

1. surface 需要显式记录当前 `ProductMode`。
   - 首次出现默认是 `normal mode`
   - 用户必须显式切到 `vscode mode`
   - surface 可以记住上一次 mode
   - `/status` 必须展示当前 mode
2. `normal mode` 是全面的 workspace-first 叙事。
3. `normal mode /list` 展示的是可用 workspace，而不是 VS Code instance。
4. `normal mode /new` 不只是“准备一条新 thread 的第一条消息”，而是 surface 在当前 workspace 下拥有一个“待 materialize 的新 thread”。
5. `normal mode` 中 `/follow` 废弃。
6. workspace 和 thread 都有互斥语义，只是 normal mode 下最高层互斥改成了 workspace。

## 3. 当前实现约束

当前代码已经形成以下事实，这些事实决定了 redesign 不能只停留在 UI 文案层：

1. surface 当前先 attach instance，再在其上 `/use` 或 `/follow` thread。
2. `/use` / `/useall` 当前走 merged thread view，会跨 instance 汇总成 thread-first 全局列表。
3. claim 目前只有 `instanceClaims` 与 `threadClaims`，没有显式 workspace claim。
4. prompt 基础配置当前来自 `thread explicit -> Instance.CWDDefaults[cwd]`，surface override 再覆盖其上。
5. route 主状态仍围绕 `AttachedInstanceID + RouteMode + SelectedThreadID` 建模。
6. VS Code source 确实存在“workspace 可能在用户不知情时切换”的现实问题，所以不能把它直接并入 normal mode 的 workspace 语义。

因此这轮 redesign 需要同时重做：

1. surface 的产品 mode 模型。
2. workspace claim / thread claim 的层次。
3. `/list` / `/use` / `/new` / `/follow` 的命令语义。
4. workspace identity 与默认配置归属。

## 4. 重构后的总体模型

新的骨架不是“先 attach 什么，再推导 mode”，而是反过来：

1. surface 先有一个显式 `ProductMode`
   - `normal`
   - `vscode`
2. mode 决定 `/list`、`/use`、`/follow`、`/new`、状态提示与 attach 语义。
3. detached 不是 mode 之外的第三条路，而是**每个 mode 都可能存在 detached 状态**。

这意味着：

1. `normal detached` 是合法稳定态。
2. `vscode detached` 也是合法稳定态。
3. `/list` 不应该再承担“顺手切 mode”的隐式职责。
4. 用户看到的“当前处于哪种产品模式”必须独立于“当前有没有 attach 目标”。

## 5. 模式切换规则

### 5.1 显式切换

推荐把 mode 切换做成明确命令，而不是让 `/list` 或 `/use` 偷偷改变 surface 类型：

1. `/mode normal`
2. `/mode vscode`

也可以提供等价菜单按钮，但产品语义应当等价于这两个显式动作。

推荐规则：

1. 首次 materialize 的 surface 默认 `normal mode`。
2. 只有显式执行 mode 切换命令，才会改变 `ProductMode`。
3. `/list`、`/use`、卡片 attach 行为都遵从当前 mode，不负责隐式切 mode。

### 5.2 记忆与展示

surface 应记住最近一次 mode：

1. `/detach` 不改变 mode。
2. daemon 重启后，如果 surface 状态被恢复，mode 也应一起恢复。
3. `/status` 需要显式展示：
   - 当前 mode
   - 当前 attach 的对象类型
   - 当前是否已拥有 thread

### 5.3 mode switch 的清理语义

mode 切换本身应当是一次 detach-like 清理，而不是“带着旧路由语义穿越到新 mode”。

切换时建议统一清掉：

1. 当前 attach / claim
2. 当前 selected thread
3. `PromptOverride`
4. pending request / request capture
5. prepared new thread 状态
6. staged image 与未发送 draft

切换后的目标态：

1. `/mode normal` -> `normal detached`
2. `/mode vscode` -> `vscode detached`

这样做的原因是避免“上一种模式的草稿、claim、follow 语义”悄悄污染下一种模式。

## 6. Normal Workspace Mode

### 6.1 核心语义

normal mode 下，surface 面向的是 workspace，不是 instance：

1. 一个 surface 同时只拥有一个 workspace。
2. 一个 workspace 同时只允许一个 normal-mode surface 占有。
3. thread 互斥仍然存在，只是它退回为 workspace 内的第二层排他。
4. normal mode 下普通文本只能发往：
   - 当前已选中的 thread
   - 或当前已拥有的“prepared new thread”

因此 normal mode 里至少有四个稳定态：

1. `N0 NormalDetached`
2. `N1 WorkspaceAttachedNoThread`
3. `N2 WorkspacePinnedThread`
4. `N3 WorkspacePreparedNewThread`

其中 `N3` 对应的是：

1. surface 已拥有当前 workspace
2. surface 还没有 materialize 出一个新 thread id
3. 但下一条普通文本会在这个 workspace 下创建并接管新的 thread

这就是“不是要先选中它，而是要拥有一个”的产品语义。

### 6.2 `normal mode /list`

`normal mode` 下，`/list` 的职责重构为“列 workspace，并支持切换 workspace”：

1. detached 时，`/list` 展示可 attach 的 workspace。
2. 已 attach workspace 时，`/list` 仍然可用，用来直接切到另一个 workspace。
3. 用户点选 workspace 后：
   - 若当前是 detached，则 attach 到该 workspace
   - 若当前已在别的 workspace，则直接切换 workspace
4. attach / switch 成功后，统一进入 `N1 WorkspaceAttachedNoThread`。
5. 系统立即发送 thread attach 卡片或 `/use` 引导，明确要求用户继续选 thread。

展示规则建议：

1. 列出可用 workspace。
2. 已被其他 normal-mode surface 占有的 workspace 可显示为 busy，但不可选。
3. 若某个 workspace 当前有 VS Code 活动，可显示提示，但不作为硬阻塞。

这样 `/list` 就成为 normal mode 下的“workspace attach / workspace switch”标准入口。

### 6.3 `normal mode /use` / `/useall`

`/use` / `/useall` 在 normal mode 下仍然保留，但语义要按 attach 状态分层：

1. `normal detached`
   - `/use` / `/useall` 可以保留为全局 thread 入口。
   - 用户选中 thread 后，系统先解析其所属 workspace，再 attach 该 workspace，并 pin 到目标 thread。
   - 已被其他 normal-mode surface 占用的 workspace，其下 thread 不可选。
2. `normal attached`
   - `/use` 只列当前 workspace 最近 thread。
   - `/useall` 只列当前 workspace 全部 thread。
   - 不再通过 `/use` 静默跳到其他 workspace。

因此在 normal mode 里：

1. `/list` 是 workspace 入口。
2. `/use` 是 thread 入口。
3. detached `/use` 只是一个 thread-first 快捷入口，但落点仍然回到 workspace-first 语义。

### 6.4 `normal mode /new`

`/new` 在 normal mode 下保留，并且语义需要更明确：

1. 它不是“暂时没有 thread，只是等下一条消息再说”的弱状态。
2. 它是 surface 在当前 workspace 下显式拥有一个“待创建的新 thread”。
3. 这个 thread 可以 lazy create。
4. 但 ownership 已经成立，直到用户：
   - 发出第一条普通文本并 materialize 它
   - 或 `/use`
   - 或 `/list` 切 workspace
   - 或 `/detach`
   - 或切 mode

这要求运行时后续实现里把 `prepared new thread` 当成一个真正的 route/ownership 状态，而不是临时补丁。

### 6.5 `normal mode /follow` 与 `/detach`

normal mode 下：

1. `/follow` 废弃，不再作为合法长期路径。
2. `/detach` 释放当前 workspace 与 thread ownership，但不修改当前 mode。
3. `/list` 可以直接切 workspace，不强制要求先 `/detach`。

直接切 workspace 的前提是沿用现有的阻塞规则：

1. 当前有 queued / running work 时不能静默切走。
2. 当前有 request gate / abandoning / 其他 dominant gate 时也不能硬切。

## 7. VSCode Mode

### 7.1 核心语义

vscode mode 的产品目标不是“拥有一个稳定 workspace”，而是“观察并跟随一个 VS Code 实例当前正在看的会话”：

1. attach 的对象是 VS Code instance。
2. follow 默认开启，并且长期为准。
3. surface 不承诺自己拥有稳定 workspace 主心骨。
4. surface override 可以存在，但不持久化为 workspace 默认值。

### 7.2 `vscode mode /list`

`/list` 在 vscode mode 下继续承担“列 VS Code instance 并接管”的职责：

1. `vscode detached` 时，`/list` 展示在线 VS Code instance。
2. `vscode attached` 时，`/list` 允许切换当前 instance。
3. attach 或 switch 成功后：
   - 若已观测到 focused thread，则进入 follow-bound 状态
   - 若尚未观测到 thread，则进入 waiting 状态，并明确提示用户先在 VS Code 里操作一次会话

### 7.3 `vscode mode /use` / `/useall`

这里保留一个“手动强制切一次 thread”的能力，但它不能改写 follow-first 的总原则：

1. 如果当前 instance 已有可见 thread 列表，则允许 `/use` / `/useall` 手动选一个 thread。
2. 这个选择只是一次性的 rebind / force-pick。
3. 只要后续 VS Code 再观测到新的 focused thread，surface 仍然跟着切过去。
4. 如果 attach 上来时还没有任何 thread 元数据，则不盲目适配，直接提示用户先去 VS Code 里实际操作一次会话。

### 7.4 配置归属

vscode mode 下：

1. 不建立自己的 workspace defaults。
2. 默认沿用 VS Code / 当前 thread 已有配置。
3. `/model`、`/reasoning`、`/access` 只做 surface 级临时 override。
4. detach 或 mode switch 后清理这些 override。

## 8. Workspace Identity 与 Claim

### 8.1 产品层判断

我同意最新评论里的核心方向：**产品层不应该同时教用户两个概念，叫 `WorkspaceRoot` 和 thread `cwd`。**

如果当前 Codex 暴露出来的 thread `cwd` 本身就是“这个 thread 所属 workspace 的路径表达”，那产品层没有必要再把两者包装成两个相互竞争的用户概念。

### 8.2 推荐的技术落点

即便产品层只暴露一个“workspace 路径”概念，运行时仍然建议保留一个明确的 canonical key：

1. normal mode 使用单一 `WorkspaceKey` 作为：
   - workspace claim key
   - `/list` 的 workspace identity
   - `/status` 的 workspace 展示主体
2. 这个 `WorkspaceKey` 应优先来自“当前 runtime 真正用于 thread/workspace 归属判断的路径字段”。
3. `Instance.WorkspaceRoot` 可以作为：
   - fallback 来源
   - 对照来源
   - 恢复链路的辅助 metadata
4. `ThreadRecord.CWD` 仍可保留为执行与继承元数据，例如：
   - `/new` 时继承创建位置
   - queue item freeze
   - 恢复提示

关键点不是保不保留字段，而是：

1. 不把它们讲成两个不同的产品对象。
2. 最终只产出一个 workspace identity 给产品层使用。

### 8.3 claim 分层

normal mode 下建议明确两层互斥：

1. `workspace claim`
   - 最高层产品排他
   - 同一 workspace 同时只能被一个 normal-mode surface 占有
2. `thread claim`
   - 仍然保留
   - 负责当前已选 thread 或 prepared-new-thread 的线程级 ownership

vscode mode 下则保留 instance attach / follow 语义，但它不升级成 normal mode 的 workspace 排他规则。

## 9. 配置归属

### 9.1 normal mode

normal mode 下，持久默认配置应当升级为 workspace 级 source of truth：

1. `thread explicit`
   - 继续表示 thread 自己已有的明确配置
2. `workspace defaults`
   - 接替今天的 `CWDDefaults`
   - 成为 model / reasoning / access 的持久默认配置归属
3. `surface override`
   - 继续存在
   - 但只代表当前飞书 surface 的临时覆盖
   - detach 或 mode switch 后清理

### 9.2 vscode mode

vscode mode 不写自己的 workspace defaults：

1. 继续沿用实例 / thread 已有配置
2. 允许 surface override
3. 不把 override 持久化

## 10. 目标状态机骨架

这轮 redesign 最关键的状态机修正是：**mode overlay 与 route state 解耦。**

先有 `ProductMode`：

1. `M0 Normal`
2. `M1 VSCode`

再在 mode 内看 route：

1. normal mode
   - `N0 NormalDetached`
   - `N1 WorkspaceAttachedNoThread`
   - `N2 WorkspacePinnedThread`
   - `N3 WorkspacePreparedNewThread`
2. vscode mode
   - `V0 VSCodeDetached`
   - `V1 VSCodeAttachedWaiting`
   - `V2 VSCodeAttachedFollowing`

这里最重要的不是名字，而是两条原则：

1. detached 在两种 mode 下都存在。
2. `normal mode` 不再共享 `follow_local` 叙事。

## 11. 对现有命令的重新定义

按新的方向，命令矩阵应当变成：

1. `/mode normal`
   - 切到 normal mode，并做 detach-like 清理
2. `/mode vscode`
   - 切到 vscode mode，并做 detach-like 清理
3. `/list`
   - normal mode: 列 workspace / 切 workspace
   - vscode mode: 列 VS Code instance / 切 instance
4. `/use` / `/useall`
   - normal detached: 全局 thread 快捷入口
   - normal attached: 当前 workspace thread 列表
   - vscode mode: one-shot force-pick
5. `/follow`
   - 只在 vscode mode 保留意义
6. `/new`
   - 只在 normal mode 保留
7. `/status`
   - 必须把 mode 和 attach 对象类型一起展示

## 12. 对旧 follow-up issue 的影响

上一版设计拆出的 `#68` 到 `#73` 建立在旧前提上，当前至少有三处基础假设已失效：

1. mode 不是路径依赖，而是 surface 显式状态。
2. normal mode 下 `/list` 不是禁用，而是核心 workspace 入口。
3. `WorkspaceRoot` vs `thread cwd` 不应继续被当成两个并列产品概念。

因此当前更稳妥的做法是：

1. 先把母设计重新收敛。
2. 再决定旧 follow-up issue 是重写、复用还是废弃。

## 13. 下一步建议拆分

等这版母设计稳定后，建议按下面顺序重新拆实现：

1. `ProductMode` 基础设施
   - 显式 mode 字段
   - `/mode`
   - `/status` 展示
   - mode switch 清理语义
2. normal mode workspace attach / switch
   - `/list` workspace 列表
   - workspace claim
   - attach 后进入 `WorkspaceAttachedNoThread`
3. normal mode thread ownership
   - `/use` / `/useall` 语义重做
   - `prepared new thread` 升级为正式状态
   - `/follow` 在 normal mode 下移除
4. vscode mode 产品收窄
   - 显式进入
   - `/list` 实例列表
   - follow-first + one-shot `/use`
5. workspace identity 与默认配置迁移
   - `WorkspaceKey`
   - workspace defaults
   - 恢复链路与展示同步

## 14. 当前仍值得继续讨论的点

当前我建议继续围绕下面 3 个点收口：

1. detached 的 normal mode 是否保留全局 `/use` thread 快捷入口
   - 我的倾向是保留，因为它只是快捷入口，不会破坏 workspace-first 落点
2. `WorkspaceKey` 的来源优先级
   - 我的倾向是：谁更接近 runtime 真实 thread/workspace 归属，就优先信谁；产品层只保留一个结果
3. `normal mode /list` 是否展示 busy workspace
   - 我的倾向是展示但禁用，这样用户能理解“为什么某个 workspace 现在不可选”
