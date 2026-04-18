# 飞书 slash/menu 业务流 owner-card 差距审计（草稿）

> Type: `draft`
> Updated: `2026-04-18`
> Summary: 基于 issue #262 提炼出的 owner-card 目标模型，审计当前所有可由 slash command 或 `/menu` 启动的业务流，梳理哪些地方值得迁移、哪些需要按产品语义适配、哪些暂不值得单独改造。

## 1. 文档定位

这份文档只做一件事：

- 站在 `owner card` 目标模型下，审计当前 slash command 和 `/menu` 可启动业务流的交互承接差距。

它不是实现方案，也不是拆分后的执行计划。这里先回答的是：

- 当前每条业务流到底由哪张卡承接。
- 它和 `单一业务默认单 owner card 走到底` 的目标差距在哪。
- 哪些值得优先改，哪些应先挂起，哪些本来就不需要硬套 owner-card。

这份草稿当前先作为讨论输入落档，后续可继续迭代。

## 2. 审计依据

这次审计主要基于以下输入：

- GitHub issue `#262` 已确认的目标模型
- `docs/general/feishu-business-card-interaction-principles.md`
- `docs/draft/feishu-command-card-workflow-audit-2026-04.md`
- `docs/general/feishu-card-ui-state-machine.md`
- `docs/general/remote-surface-state-machine.md`
- `docs/general/feishu-card-api-constraints.md`
- 当前命令定义、菜单分组、ingress handoff、inline replace、submission anchor、业务 runtime 与 projector 代码路径

这次不重复展开 `#262` 的 picker 细节实现，而是把它当成目标模型，反看其他业务流。

## 3. 审计范围与分组

当前 `/menu` 可见命令共 22 个，但更适合按“业务流”而不是按“单命令”审计。这里采用下列 12 组：

1. `/menu`
2. `/help`
3. `/list` `/use` `/useall`
4. `/history`
5. `/sendfile`
6. `/new`
7. `/follow` `/detach`
8. `/status`
9. `/stop` `/compact` `/steerall`
10. `/mode` `/autowhip` `/reasoning` `/access` `/model` `/verbose`
11. `/cron`
12. `/debug` `/upgrade`

分组原则是：

- 同一组共享同一种用户问题或同一种 owner 形态。
- 真正需要迁移的是“业务流”，不是“按钮数量”。
- 如果一条命令只是瞬时切换，没有持续执行态，就不应被误判成必须拥有完整 owner-card runtime。

## 4. 快速总表

| 业务流 | 当前主承接形态 | 与 owner-card 目标的主要差距 | 初步判断 |
| --- | --- | --- | --- |
| `/menu` | 菜单卡，进入后常发生 ownership 断裂 | 菜单进入业务后仍常插提交锚点或旁路结果卡 | 高优先级共性基座 |
| `/help` | 结果说明卡 | 本身不是持续业务流 | 不建议单独改造 |
| `/list` `/use` `/useall` | target picker + 后续旁路执行/结果 | 典型的 owner-card 断裂流 | 最高优先级 |
| `/history` | 已较接近单卡流 | 仍缺统一 flow runtime 与统一 sealing 语义 | 高优先级 first adopter 候选 |
| `/sendfile` | 入口卡 + path/file picker + 发送结果分裂 | 子页面与最终结果 ownership 不稳定 | 高优先级 |
| `/new` | 触发创建/切换，结果常落到别处 | 短流但仍会发生 handoff 不清晰 | 中优先级 |
| `/follow` `/detach` | 立即切换或提示 | 更像状态切换，不是长业务流 | 低优先级 |
| `/status` | 状态展示卡 | 只读卡，不需要 owner-card runtime | 低优先级 |
| `/stop` `/compact` `/steerall` | 立即动作或启动执行 | `/compact` 的执行态可能需要 owner 化，其余偏瞬时 | 分裂处理 |
| 参数卡 6 项 | 参数选择卡 + 回执 | 缺统一“同卡选择 -> 同卡封卡”语义 | 高优先级共性短流 |
| `/cron` | 管理入口 / 外链 / reload 回执 | 更偏管理面，不一定值得上完整 owner-card | 中优先级，先观察 |
| `/debug` `/upgrade` | 特殊 continuation、异步 patch、长任务 | 长任务承接方式可借鉴 owner-card，但后台例外语义未定 | 中高优先级 |

## 5. 逐 flow 审计

### 5.1 `/menu`

当前定位：

- 它本身只是 launcher card，不应承担具体业务执行态。

当前问题：

- 进入业务后并没有统一的 handoff 规则。
- 某些路径会插入“命令已提交”锚点卡，某些路径会直接继续，某些路径又会让结果飘到别的卡。
- 用户从菜单点进业务后，经常不知道接下来该盯哪一张卡。

owner-card 视角下的差距：

- 菜单本身没有问题，问题在于菜单到业务 owner-card 的一次性交接没有被统一建模。

判断：

- `/menu` 不需要自己变成 owner-card。
- 但“菜单 handoff 基座”是这轮审计里最值得单独抽出的共性问题之一。

### 5.2 `/help`

当前定位：

- 说明性、导航性卡片。

当前问题：

- 主要是信息组织问题，不是 owner 断裂问题。

owner-card 视角下的差距：

- 它本来就不是需要执行、进度、取消、封卡的业务流。

判断：

- 不值得按 owner-card 模型单独重构。
- 如果未来要改，只应改信息结构和入口组织。

### 5.3 `/list` `/use` `/useall`

当前定位：

- 当前最典型的 target picker 业务流。
- 也是 `#262` 已经明确要迁移的主路径。

当前问题：

- 选择目标的卡与后续执行卡、结果卡、提示卡之间 ownership 经常断裂。
- 菜单进入后仍可能插提交锚点。
- 加工作区、本地目录、Git 导入、切换线程等路径的收口方式不一致。
- 用户在“业务还没完”的时候，就被迫去盯别的消息。

owner-card 视角下的差距：

- 这是当前最明显不符合目标模型的一组业务流。

判断：

- 最高优先级。
- 不应再继续按单路径补丁推进，应由统一 owner-flow runtime、child subpage、running/sealing 基座承接。

### 5.4 `/history`

当前定位：

- 目前最接近“单业务单卡走到底”的现有实现之一。

当前问题：

- 虽然产品感知已接近 owner-card，但底层仍偏专用实现。
- 缺统一 flow runtime、统一 final sealing 与共性 cancel/expired 规则。

owner-card 视角下的差距：

- 它不是“问题最大”的路径，但很适合作为 first adopter 去验证基座。

判断：

- 高优先级。
- 值得优先作为 owner-card runtime v1 的验证业务，而不是先拿最复杂的 picker 开刀。

### 5.5 `/sendfile`

当前定位：

- 是一个明确的业务流，不只是瞬时命令。
- 通常需要选目标、选路径或文件、提交发送，并等待发送结果。

当前问题：

- picker 子步骤与最终发送动作之间的 ownership 仍可能分裂。
- 从用户视角，它比纯参数卡更像完整业务流，但又没有稳定的单卡承接。
- 结果与错误回执如果散落到外部消息，会放大“到底这张卡还活没活着”的困惑。

owner-card 视角下的差距：

- 它非常适合迁移到单 owner-card，但当前缺少通用子页面和封卡机制。

判断：

- 高优先级。
- 适合作为 picker runtime 稳定后的第二批复用业务。

### 5.6 `/new`

当前定位：

- 是创建新会话/新工作流入口，但常与当前 attached thread、mode、workspace 状态耦合。

当前问题：

- 用户触发后，结果经常体现为“新的会话已创建/已切换”，但中间承接并不总在同一张卡内。
- 它介于“纯瞬时切换”和“有明确业务流”的中间地带。

owner-card 视角下的差距：

- 如果只看短路径，它未必需要很重的 runtime。
- 但只要创建过程需要选择目标或等待 ready，它又天然会掉进 owner 断裂问题。

判断：

- 中优先级。
- 更适合跟随 target picker / workspace 接入流统一收口，而不是单独先做。

### 5.7 `/follow` `/detach`

当前定位：

- 更像 surface routing 或附着关系切换，而不是复杂业务流。

当前问题：

- 主要问题是状态语义和可见提示是否清楚，不是卡片 ownership 被拆散。

owner-card 视角下的差距：

- 这类动作多数是立即完成或立即拒绝，不需要运行态、进度态、封卡态的完整模型。

判断：

- 低优先级。
- 只需要保持结果明确、旧卡失效规则清楚，不值得上完整 owner-card 改造。

### 5.8 `/status`

当前定位：

- 只读状态卡。

当前问题：

- 没有明显的 owner 断裂问题。

owner-card 视角下的差距：

- 它不拥有持续执行流。

判断：

- 低优先级。
- 不建议为了统一而强行 owner-card 化。

### 5.9 `/stop` `/compact` `/steerall`

当前定位：

- 这三者都作用于当前运行态，但业务强度并不一样。

`/stop`：

- 更像立即动作或确认性动作。
- 核心是结果是否明确，不是长业务 owner。

`/steerall`：

- 更像把消息路由进当前 turn 的动作。
- 主要是路由与权限/状态判断，不是 owner-card 流。

`/compact`：

- 如果只是短回执，也可视为简单命令。
- 但如果它存在明确的运行中、进度、取消和完成摘要，就已经更接近长业务流。

判断：

- `/stop`、`/steerall` 不建议单独 owner-card 化。
- `/compact` 值得纳入“长任务 owner-card 候选”。

### 5.10 `/mode` `/autowhip` `/reasoning` `/access` `/model` `/verbose`

当前定位：

- 这一组本质上是“参数选择短流”。
- 裸命令会回一张参数卡，选择后再给回执。

当前问题：

- 每条命令都在单独处理自己的“选择态 -> 提交态 -> 完成态”。
- 缺少统一的短流 sealing 语义。
- 菜单进入时，owner handoff 也不稳定。

owner-card 视角下的差距：

- 它们不需要长任务 runtime，但需要统一的“单卡完成”模型。
- 如果继续每条命令各写一套，会让用户感知不一致。

判断：

- 高优先级，但属于“轻量 owner-card / 参数卡基座”。
- 值得作为一组一起设计，而不是六条命令分别修。

### 5.11 `/cron`

当前定位：

- 更偏管理入口。
- 可能打开专属表格、执行 reload、显示最近运行状态。

当前问题：

- 它更像“管理面跳转/外链 + 少量命令回执”，不完全符合典型 owner-card 业务。
- 如果强行上完整 owner-card，可能只是在飞书里复制一层管理台语义。

owner-card 视角下的差距：

- 真正需要的可能不是单卡执行态，而是更清晰的入口、回执与失败提示。

判断：

- 中优先级。
- 先保留观察，不建议在第一批 owner-card 改造中优先投入。

### 5.12 `/debug` `/upgrade`

当前定位：

- 这两条都更接近“长任务 / 特殊 continuation / 异步 patch”。
- 现有实现已经说明这类业务需要跨阶段承接。

当前问题：

- 当前承接方式带有较强特例色彩。
- 同卡 running、后台例外、最终 patch/封卡之间的规则还不统一。
- 如果未来 owner-card runtime 稳定，这两条应能直接受益。

owner-card 视角下的差距：

- 它们不是最先要做的业务，但却是最能检验“长任务是否允许后台化、何时封卡、何时提醒”的样本。

判断：

- 中高优先级。
- `/upgrade` 更值得优先评估为 owner-card 长任务候选。
- `/debug` 是否值得跟进，要看产品上是否真需要把它做成用户可持续盯的业务流。

## 6. 跨 flow 的共性缺口

这次审计里真正反复出现的，不是某一个命令的细节问题，而是下面 6 类共性缺口：

### 6.1 菜单 handoff 不统一

- 从 `/menu` 进入业务后，谁接管 ownership 没有统一规则。
- “命令已提交”锚点卡仍在部分路径里承担了不该承担的中间层角色。

### 6.2 缺统一的 owner-flow runtime

- 现在多数业务还是专用 `activeXRecord` 或命令特判。
- 没有一个跨业务的 flow id / page / phase / sealing / freshness 载体。

### 6.3 短流缺统一封卡语义

- 参数卡类业务大多能做完，但“做完之后这张卡如何终结”不一致。
- 用户看到的是很多相似业务，却得到不同风格的提交回执。

### 6.4 长任务缺统一 running/progress/cancel/final 语义

- `/upgrade`、`/compact`、Git 导入等长任务样本都在说明这个问题。
- 现在运行态和结果态承接方式偏分裂。

### 6.5 全局动作与流内取消语义没完全分层

- `/stop`、业务取消、子页面取消、返回、失效提示，这几类动作在用户心智上需要更稳定的边界。

### 6.6 后台例外策略还未稳定

- 哪些业务允许脱离当前 owner card 去后台跑，哪些不能，目前还不是一条稳定规则。
- 这会直接影响 `/upgrade`、Git 导入、未来其他长任务的产品语义。

## 7. 候选 issue 桶

如果把这次审计拆成后续议题，比较合理的桶大致如下：

### 7.1 第一批，值得尽快展开

1. 菜单 handoff 与 submission-anchor 清理
2. 统一 owner-flow runtime v1
3. target picker `/list` `/use` `/useall` 迁移
4. 参数卡短流基座：`/mode` `/autowhip` `/reasoning` `/access` `/model` `/verbose`
5. `/sendfile` 单 owner-card 化

### 7.2 第二批，等第一批稳定后评估

1. `/history` 作为 runtime first adopter 或对齐项
2. `/new` 与 target/workspace 流的并入式改造
3. `/upgrade` 长任务 owner-card 化
4. `/compact` 是否纳入统一长任务模型

### 7.3 第三批，先挂起防遗忘

1. `/cron` 的 owner-card 适配是否有真实收益
2. `/debug` 是否值得做成前台 owner 业务
3. `/follow` `/detach` `/status` `/stop` `/steerall` 是否只需保持结果清晰即可

## 8. 初步优先级判断

按“用户可感知混乱程度”和“是否有共性复用价值”一起看，当前优先级大致是：

1. 最高优先级：
   - `/list` `/use` `/useall`
   - 菜单 handoff / submission-anchor 清理
2. 高优先级：
   - 参数卡 6 项的统一短流模型
   - `/sendfile`
   - `/history` 作为 runtime 验证样本
3. 中优先级：
   - `/new`
   - `/upgrade`
   - `/compact`
   - `/cron`
4. 低优先级：
   - `/help`
   - `/follow`
   - `/detach`
   - `/status`
   - `/stop`
   - `/steerall`
   - `/debug` 是否投入需再判断

## 9. 待讨论问题

在真正拆实现前，还需要先讨论清楚几件事：

1. 参数卡类短流到底是否需要统一 owner-card runtime，还是只要有一层更轻的“同卡封卡”基座就够。
2. `/history` 应该先作为 first adopter，还是让 picker 先推动 runtime 成形。
3. `/upgrade` 这类长任务哪些阶段必须留在当前 owner card，哪些可以作为明确后台例外。
4. `/sendfile` 的 path/file picker 是否要与 target picker 共享同一套 child subpage 机制。
5. `/cron` 和 `/debug` 这类偏管理面的命令，是否真的应该纳入 owner-card 改造主线。

## 10. 结论

这轮审计得出的结论不是“所有 slash/menu 命令都应该改成 owner-card”。

真正值得迁移的，是那些已经形成清晰业务流、但当前仍把 ownership 打散在入口卡、提交锚点卡、进度卡、结果卡之间的路径。

按这个标准看，当前最值得投入的是：

- 菜单 handoff 基座
- target picker 相关流
- 参数卡短流基座
- `/sendfile`
- 长任务样本中的 `/upgrade` 与可能的 `/compact`

而 `/help`、`/status`、`/detach`、`/follow` 这类本就不是持续业务流的命令，不应该为了形式统一而被硬改成 owner-card。
