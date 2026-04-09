# Workspace 模型重构设计

> Type: `draft`
> Updated: `2026-04-09`
> Summary: 首次定稿 normal workspace mode 与 vscode mode 的双模式方案，明确 workspace identity、claim、命令语义与配置归属。

## 1. 文档目标

这份文档服务于 `#67 Workspace 概念化`，目标不是描述当前已实现行为，而是把下一阶段要落地的产品模型先定型。

本轮重点回答四个问题：

1. remote surface 以后到底围绕什么核心对象工作。
2. `normal mode` 和 `vscode mode` 是否需要显式拆开。
3. workspace identity、独占 claim、thread 选择和命令入口分别怎么定义。
4. model / reasoning / access 这些配置到底归谁管。

## 2. 当前实现约束

当前代码已经形成以下事实：

1. surface 当前先 attach instance，再在其上 `/use` 或 `/follow` thread。
2. `/use` / `/useall` 走的是 merged thread view，会跨 instance 汇总成 thread-first 全局列表。
3. thread claim 只有 `threadID`，没有显式 workspace claim。
4. prompt 基础配置当前来自 `thread explicit -> Instance.CWDDefaults[cwd]`，surface override 再覆盖其上。
5. `WorkspaceRoot` 目前更接近 instance 自报的根目录元数据，不等于产品层已经稳定建模的 workspace。
6. VS Code source 存在“workspace 可能在用户不知情时切换”的现实问题，不能直接套入普通 headless/workspace 语义。

这意味着如果继续沿着“全局 thread-first + instance attach”硬加需求，`/history`、`/use`、恢复链路和默认配置都会反复返工。

## 3. 设计结论摘要

本轮先收敛为两个显式产品模式，而不是继续试图用一套语义同时覆盖所有 source：

1. `normal workspace mode`
   - 面向 headless / managed headless / 其他稳定 workspace source。
   - 产品语义全面 workspace-first。
   - 一个飞书 surface 独占一个 workspace。
   - attach workspace 后默认先不选 thread，必须先在 workspace 内选 thread。

2. `vscode mode`
   - 面向 `source=vscode` 的实例。
   - 产品语义是“观察并跟随当前 VS Code 会话”，不是 bot 自己拥有一个稳定 workspace。
   - follow 默认开启且长期为准。
   - surface override 可以临时存在，但不持久化，也不作为 workspace 默认配置来源。

这两个模式之间允许共用底层 relay / wrapper / orchestrator 基础设施，但产品层不再强行共用一套叙事。

## 4. 模式切换规则

surface 以后需要显式区分当前处于哪种产品模式。

### 4.1 进入 `normal workspace mode`

以下路径进入 normal mode：

1. 用户 attach 一个稳定 workspace。
2. 用户在 detached 状态下通过 `/use` / `/useall` 选择了一个可解析到稳定 workspace 的 thread。
3. 后台恢复链路命中了一个可恢复的非-VSCode workspace。

进入 normal mode 时，surface 应执行一次带模式切换语义的清理：

1. 清掉旧的 `PromptOverride`。
2. 清掉 pending request / request capture。
3. 清掉 prepared new thread 状态。
4. 丢弃未发送草稿。
5. 释放旧模式留下的 attach / thread claim。

### 4.2 进入 `vscode mode`

以下路径进入 vscode mode：

1. 用户在 detached 状态下通过 `/list` 选择一个 VS Code 实例。
2. 用户已在 vscode mode 中，再次切到另一台 VS Code 实例。

进入 vscode mode 时，同样先做一次 detach-like 清理，但有两个额外规则：

1. follow 默认开启，不保留旧的 pinned thread 叙事。
2. surface override 只作为当前 surface 的临时覆盖，不写回任何持久默认配置。

### 4.3 模式切换后的可见原则

1. `normal mode` 不暴露 VS Code instance 列表。
2. `vscode mode` 不暴露 workspace attach 独占语义。
3. detached 状态允许用户自行选择进入哪条路径：
   - `/use` / `/useall` 走 normal path
   - `/list` 走 vscode path

## 5. Normal Workspace Mode

## 5.1 核心语义

normal mode 下，surface 面向的是一个 workspace，而不是一个 instance：

1. 一个 surface 只 attach 一个 workspace。
2. 一个 workspace 同时只能被一个 normal-mode surface 占有。
3. attach workspace 后默认处于“已选 workspace、未选 thread”状态。
4. 只有选中 workspace 内的某个 thread 后，普通文本才能继续发送。

## 5.2 detached 状态下的 `/use` / `/useall`

当 surface 当前没有 workspace 时：

1. `/use` / `/useall` 展示全局“可 attach 的 thread 候选”。
2. 列表需要按 workspace 过滤：
   - 已被其他 normal-mode surface 占用的 workspace，其下 thread 不可选。
   - 仅被 VS Code 当前使用的 thread 不构成硬阻塞，只做提示信息。
3. 用户选中后，系统先 attach 对应 workspace，再选中该 thread。

这意味着 detached `/use` 的全局列表，本质上是“workspace attach 的 thread 入口”，不再是“全局 thread-first 一视同仁”的旧语义。

## 5.3 已 attach workspace 时的 `/use` / `/useall`

当 surface 已有 workspace 时：

1. `/use` 默认展示当前 workspace 最近 thread。
2. `/useall` 展示当前 workspace 的全部可见 thread。
3. 不再借由 `/use` 静默跳到其他 workspace。
4. 若要切换到另一个 workspace，应先 detach，再从 detached 入口重新选择。

## 5.4 `/list` 与 `/follow`

normal mode 下：

1. `/list` 不再是合法入口；它只属于 vscode mode。
2. `/follow` 不再作为 normal mode 的长期产品路径。
3. `/new` 仍然保留，但语义变成“在当前 workspace 中，用当前 thread 的 cwd 准备一个新 thread”。

## 5.5 VS Code 占用的处理

normal mode 不把 VS Code 当前活动当成 workspace 独占者：

1. VS Code 当前 focused thread 可以作为“正在本地使用”的提示展示。
2. 这类提示不阻止 normal mode attach workspace。
3. normal mode 若要选中同一个 thread，可以直接按 normal-mode 规则继续，不必因为 VS Code 当前正在看它而阻塞。

这里的核心取舍是：

1. workspace 独占是 bot 侧产品规则。
2. VS Code 当前活动是本地运行时现象，不升级成 workspace 级排他 claim。

## 6. VSCode Mode

## 6.1 核心语义

vscode mode 是“跟着 VS Code 走”的模式，不给用户承诺一个稳定 workspace 主心骨：

1. attach 的对象是 VS Code instance，不是 workspace claim。
2. follow 默认开启，并且长期为准。
3. surface 不持久拥有自己的 workspace 默认配置。
4. detach 后，当前 surface 上的临时 override 一并清理。

## 6.2 `/list`

`/list` 以后只服务于 vscode mode：

1. detached 状态下，`/list` 展示在线 VS Code 实例，供用户进入 vscode mode。
2. 已在 vscode mode 时，`/list` 允许切换当前附着的 VS Code 实例。
3. normal mode 下调用 `/list`，直接提示该命令只适用于 VS Code 模式。

## 6.3 `/use` / `/useall`

vscode mode 下，`/use` / `/useall` 仍然保留，但语义不是切到一个稳定 pinned route，而是“手动强制切一次当前目标 thread”：

1. 如果当前实例已经观测到 thread 列表，则允许用户手动选一个 thread。
2. 这个手动选择只是一次性的 rebind / force-pick，不改变“follow 为准”的总规则。
3. 后续只要 VS Code 观测到新的 focused thread，surface 仍然跟着切过去。
4. 如果当前 attach 上来但还没有任何 thread 元数据，明确提示用户先在 VS Code 里实际操作一次会话。

## 6.4 配置策略

vscode mode 下：

1. 初始不写任何 workspace default。
2. 默认沿用 VS Code 实例 / 当前 thread 自己已有的配置状态。
3. `/model`、`/reasoning`、`/access` 允许做 surface 级临时 override。
4. 这些 override 不持久化，detach 或模式切换后直接清空。

## 7. Workspace Identity 与 Claim

## 7.1 canonical identity

normal mode 里的 workspace identity，应以“规范化后的 workspace root / cwd root”为主，而不是直接拿当前 thread 的精确 `cwd` 当最终身份。

原因：

1. `WorkspaceRoot` 更像 instance 自报的稳定根目录。
2. thread `cwd` 可能只是该 workspace 下的子目录。
3. 如果直接按 thread `cwd` 做 identity，同一个仓库会被拆成多个伪 workspace。

因此产品定义应当是：

1. canonical workspace key 代表“这个 surface 正在占有哪一个 workspace 根”。
2. thread `cwd` 只用于：
   - thread 内具体执行路径
   - 新 thread 继承路径
   - 恢复和候选匹配时的辅助 metadata
3. thread `cwd` 不应天然升级成长期 workspace identity。

## 7.2 对当前字段的解释

1. `WorkspaceRoot`
   - 倾向于作为稳定 workspace 根的候选来源。
   - 它是 instance 级元数据，不应被 thread 子目录反复缩窄。
2. 规范化 `cwd`
   - 更适合作为临时匹配、恢复提示和 fallback 候选。
   - 只有在当前确实缺少 workspace root metadata 时，才允许作为过渡阶段的 provisional key。

也就是说，产品要围绕 workspace root 建模，而不是围绕每个 thread 的工作子目录建模。

## 7.3 claim 模型

建议把 claim 拆成两层：

1. `workspace claim`
   - 只用于 normal mode。
   - key 是 canonical workspace key。
   - 一个 workspace 同时只能被一个 normal-mode surface 占有。

2. `thread claim`
   - 作为当前 workspace 内的线程路由锁。
   - 仍可保留，但不再承担“跨 workspace 的最高层产品排他语义”。

3. `vscode instance attach`
   - 继续保留 instance 级 attach 概念。
   - 只用于 vscode mode。
   - 不升级成 normal mode 的 workspace 排他规则。

## 8. 配置归属

当前代码里的配置来源可以拆成两层来看：

1. base default 解析
   - `thread explicit`
   - `CWDDefaults[cwd]`
2. 最终覆盖
   - `surface override` 最后覆盖前面的基础结果

这里真正需要重构的不是“有没有覆盖关系”，而是“默认配置归谁持久拥有”。

### 8.1 normal mode

normal mode 下，建议把用户真正想保存的默认值升级为 workspace 级 source of truth：

1. `thread explicit`
   - 继续保留，表示这个 thread 自己已有的明确配置。
   - 它是线程事实，不是 workspace 默认值。
2. `workspace defaults`
   - 接替今天的 `CWDDefaults`。
   - 成为 model / reasoning / access 的持久默认配置归属。
3. `surface override`
   - 继续存在。
   - 只代表当前飞书 surface 的临时覆盖。
   - detach 或 mode switch 后清理。

对应的新模型也保持同样结构：

1. base default 解析
   - `thread explicit`
   - `workspace defaults`
2. 最终覆盖
   - `surface override`

其中真正的 source of truth 是 `workspace defaults` 这一层，而不是今天挂在 instance 上、按 `cwd` 分散存储的 `CWDDefaults`。
这里补充一个边界：

1. `model` / `reasoning` 需要迁移到 workspace defaults。
2. `access mode` 目前更多还是 surface 级概念，但 redesign 目标是把它也纳入 normal mode 的 workspace default 体系。

### 8.2 vscode mode

vscode mode 不建立自己的 workspace defaults：

1. 继续沿用 VS Code / thread 当前已有配置。
2. 允许 surface override。
3. 不把 override 写成持久 workspace 默认值。

## 9. 状态机映射

执行态、gate、queue overlay 这一层可以基本沿用当前状态机；主要变化发生在 route 主状态。

建议把当前 route 主状态收敛成下面两组：

1. normal mode
   - `N0 Detached`
   - `N1 WorkspaceAttachedNoThread`
   - `N2 WorkspacePinnedThread`
   - `N3 WorkspaceNewThreadReady`

2. vscode mode
   - `V1 VSCodeAttachedWaiting`
   - `V2 VSCodeAttachedFollowing`

说明：

1. `V2` 下允许 `/use` 手动 rebind，但它不是一个新的稳定 pinned state。
2. 只要观测到新的 local focused thread，`V2` 仍然跟随切换。
3. 当前 `follow_local` 的长期叙事以后只属于 vscode mode，不再是 normal mode 的主路径。

## 10. 对现有命令的影响

建议按下面的方向拆语义：

1. `/list`
   - 只保留给 VS Code 模式使用。
2. `/use` / `/useall`
   - detached 时是 normal mode 的全局 workspace attach 入口。
   - normal mode attach 后只看当前 workspace 内 thread。
   - vscode mode 下保留“强制切一次 thread”的能力，但 follow 仍然最高优先。
3. `/follow`
   - 只在 vscode mode 下保留。
4. `/new`
   - 只在 normal mode 下保留。
5. `/model` `/reasoning` `/access`
   - normal mode 写 workspace defaults 或 surface override。
   - vscode mode 只写 surface override。

## 11. 建议的后续拆分

建议按下面顺序拆 follow-up issue：

1. surface mode / workspace claim 基础设施
   - 新增 surface product mode
   - 引入 workspace claim
   - 梳理 mode switch 清理语义

2. workspace identity 与 metadata plumbing
   - 明确 canonical workspace key
   - 补齐非-VSCode source 的 workspace root 透传与持久化
   - 处理只有 `cwd` 时的 provisional key 过渡

3. normal mode `/use` / `/useall` 重构
   - detached 全局列表按 workspace 过滤
   - attached 后只看当前 workspace 内 thread
   - `/list` 在 normal mode 下禁用

4. vscode mode 产品拆分
   - `/list` 进入和切换 vscode mode
   - follow-first 路由
   - thread 未观测到时的明确提示

5. 配置归属迁移
   - `CWDDefaults` 迁移到 workspace defaults
   - `access mode` 纳入 workspace default 体系
   - vscode mode 保持 override-only

6. 状态机与文档同步
   - 更新 `docs/general/remote-surface-state-machine.md`
   - 更新 `docs/general/feishu-product-design.md`
   - 更新用户文档与帮助文案

## 12. 仍待确认的点

这轮先把主方向定下来，仍保留三个实现前需要拍板的小点：

1. detached 状态是否需要一个显式的“workspace chooser”入口，而不只依赖 `/use` / `/useall`。
2. vscode mode 下 `/useall` 的范围，是只看当前 attached instance，还是允许看该 instance 已知的全部历史 thread。
3. canonical workspace key 的规范化规则是否需要纳入 symlink / 大小写 / 挂载点归一化。
