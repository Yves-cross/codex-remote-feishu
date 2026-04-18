# 飞书 owner-card 旁路提示与发卡触发源审计（草稿）

> Type: `draft`
> Updated: `2026-04-18`
> Summary: 在 `#267` 下按业务流盘点状态变化、旁路提示源与发卡载体，补上 async / tick / recovery 三类容易打散 owner card 焦点的触发面。

## 0. 文档定位

这份草稿不是实现方案，也不是卡片样式稿。

它回答的是 `#267` 现在真正缺的那一层闭包：

- 不是只问“哪些提示应该收回 owner card”
- 而是先问“**哪些状态变化会打出新的提示/新卡，它们是从哪里发出来的**”

这层必须先盘清楚，因为当前问题不只存在于业务流自己的 confirm / failure / success：

- 有些提示来自业务流内部的 async 完成和失败
- 有些提示来自 queue / dispatch / remote turn 的状态变化
- 还有一些提示根本不在业务 handler 里，而是由 tick / 恢复 / 链路降级检查旁路弹出

如果不把这些触发源单独列出来，后面很容易把三种完全不同的东西混成一个“统一收口提示”的大 patch：

1. 当前业务流自己应该吃掉的提示
2. 当前 turn 自己应该承接的提示
3. 全局运行时本来就应该独立存在的提示

## 1. 审计口径

这次沿用 `docs/draft/feishu-slash-menu-owner-card-audit-2026-04.md` 的 12 组业务流分组，但每组新增一层“触发源”审计。

每组都回答 5 个问题：

1. 这条业务流当前的主承载是谁
2. 哪些状态变化会触发新的提示或新卡
3. 这些提示来自同步动作、异步结果，还是 tick / recovery
4. 当前实际落在哪种消息载体上
5. 从 `#267` 视角看，它应该归 `owner-flow`、`turn`，还是 `global runtime`

本文里的几个关键词：

- `owner-flow`
  - 当前业务流自己的主承载
  - 如果这条业务还没结束，默认应该优先 patch 回这里
- `turn-owned`
  - 当前 turn 的共享过程卡、request 卡、final card 之类
  - 不一定属于某个 slash/menu 业务流，但属于当前执行链
- `global runtime`
  - transport degraded、daemon shutdown、VS Code 恢复提示这类全局运行时提醒
  - 不应伪装成某条业务流自己的步骤结果

## 2. 当前系统里会“新发东西”的主要载体

当前飞书侧真正会新增消息或新增卡片主承载的载体，主要有这些：

| 载体 | 典型 UIEvent | 当前用途 |
| --- | --- | --- |
| 直接命令卡 | `command.catalog` | `/menu`、参数卡、`/cron`、`/debug`、`/upgrade` 的直接目录/状态卡 |
| 选择/业务卡 | `target.picker`、`path.picker`、`thread.history`、`request.prompt` | owner card、picker 子步骤、request 卡 |
| 系统提示卡 | `notice` | 即时错误、异步完成、恢复失败、旧卡拒绝、后台结果 |
| 共享过程卡 | `exec_command.progress` | `exec_command`、`web_search`、`mcp_tool_call`、`dynamic_tool_call`、`compact` |
| turn 结果卡 | final reply / overflow cards / image output | 最终回复、溢出回复、图片结果 |
| 输入状态提示 | `pending.input.state` | 队列/运行/typing 状态，通常不单独变成主卡，但会影响原消息周边状态 |

其中 `#267` 直接要处理的，不是所有载体，而是：

- 哪些 `notice` / 新卡本来应该被当前业务 owner-flow 吃掉
- 哪些应该继续留在 `turn-owned`
- 哪些属于 `global runtime`，本来就不该回写进 owner card

## 3. 跨业务流的统一触发源

先不按业务流，而是先按“触发源种类”看，当前大致有 5 类。

### 3.1 同步 UI 导航与编辑态动作

典型路径：

- `/menu` 分组切换
- target picker mode/source/workspace/session 切换
- path picker 浏览
- history 页码/详情切换
- request 卡题目填写与确认态切换

典型代码：

- `internal/core/orchestrator/service_feishu_ui_controller.go`
- `internal/core/orchestrator/service_target_picker.go`
- `internal/core/orchestrator/service_path_picker.go`
- `internal/core/orchestrator/service_thread_history_view.go`
- `internal/core/orchestrator/service_request.go`

当前特征：

- 大多数已经是同卡 patch 或 inline replace
- 它们本身不是 `#267` 的主问题

默认归属：

- `owner-flow`

### 3.2 业务确认后的异步完成 / 失败 / 超时

典型路径：

- target picker confirm 后等待 thread switch / new-thread ready
- 本地目录接入等待 headless prepare
- Git 导入等待 daemon command 完成
- sendfile confirm 后等待上传/发送完成
- Cron / debug / upgrade 发起后等待后台 goroutine 回来

典型代码：

- `internal/core/orchestrator/service_target_picker_owner_card.go`
- `internal/core/orchestrator/service_snapshot_runtime.go`
- `internal/core/orchestrator/service_surface.go`
- `internal/app/daemon/app_send_file.go`
- `internal/app/daemon/app_cron_commands.go`
- `internal/app/daemon/app_upgrade.go`
- `internal/app/daemon/app_upgrade_execute.go`
- `internal/app/daemon/app_upgrade_dev.go`

当前问题：

- 同类事件在不同业务里落到不同载体
- 有的会 patch 当前 owner card
- 有的会 append-only notice
- 有的会先发 started notice，再异步补另一张结果卡

默认归属：

- 如果当前业务流仍在等待这个结果，默认应判为 `owner-flow`
- 只有业务本来就不是单卡承载，或者明确后台化，才允许继续独立发消息

### 3.3 queue / dispatch / remote turn 运行态变化

典型路径：

- queue item 进入 queued / dispatching / running / failed / completed
- command dispatch failure / command rejected
- remote turn failure
- compact 完成
- request dispatch pending / request feedback queued

典型代码：

- `internal/core/orchestrator/service_queue.go`
- `internal/core/orchestrator/service_snapshot_runtime.go`
- `internal/core/orchestrator/service_compact_notice.go`
- `internal/core/orchestrator/service_request.go`

当前特征：

- 这类事件更偏 `turn-owned`
- 它们不一定属于某个 slash/menu launcher，但明确属于当前执行链
- 如果未来某条业务流本身要 owner-card 化 running 态，就需要重新决定它和共享过程卡之间谁是主承载

默认归属：

- `turn-owned`

### 3.4 tick / recovery / runtime 守护逻辑旁路提示

典型路径：

- pending headless timeout
- VS Code detached surface open prompt
- VS Code auto resume failure
- relay transport degraded
- daemon shutdown
- gateway apply failed 后的补发 notice

典型代码：

- `internal/core/orchestrator/service_surface.go`
- `internal/app/daemon/app_surface_resume_state.go`
- `internal/core/orchestrator/service_snapshot_runtime.go`
- `internal/app/daemon/app_ingress.go`
- `internal/app/daemon/app_ui.go`
- `internal/app/daemon/app_shutdown.go`

当前特征：

- 它们不是某条业务流自己在正常推进
- 很多就是用户什么都没点，但 runtime 发现状态变了，于是自己补发一张 notice
- 这就是目前最容易把“业务提示”和“全局提醒”混在一起的地方

默认归属：

- `global runtime`

唯一例外：

- 如果 tick 检查到的是“当前 owner-flow 正在等待的那个 pending 结果已经超时/失败”，可以把失败信号回写 owner card
- target picker 当前已经有一部分这样的特判

### 3.5 线程重放与延迟补发

典型路径：

- surface 当下不在 turn 上，但 thread later re-attach
- compact / final / failure 被暂存为 replay
- 重进 thread 时再补发 final text / notice / compact progress

典型代码：

- `internal/core/orchestrator/service_replay.go`
- `internal/core/orchestrator/service_queue.go`

当前特征：

- 这是“并非当场发卡，但会在之后突然补发”的另一条旁路
- 它和 tick/recovery 很像，都是“触发点不在当前业务页上”

默认归属：

- 以 `turn-owned` 为主
- 不应被误当成当前业务 owner-flow 的局部步骤

## 4. 12 组业务流审计

### 4.1 `/menu`

当前主承载：

- `command.catalog`

关键状态变化：

- 打开菜单首页
- 切换分组
- 从菜单进入具体命令
- 点击旧菜单卡 / 旧导航卡

可能触发的新提示 / 新卡：

- 同卡菜单 patch
- 命令提交锚点卡
- 目标业务自己的新卡
- 旧卡拒绝 notice

当前问题：

- `/menu` 自己不是问题，问题在于 handoff 后没有统一 owner
- 同样都是从菜单点进去，有的插提交锚点，有的直接落业务卡，有的结果继续往下 append

`#267` 归属判断：

- `/menu` 本身是 launcher，不该变成 owner-flow
- 但“菜单到业务”的 handoff 需要成为单独规则

### 4.2 `/help`

当前主承载：

- 静态说明卡

关键状态变化：

- 打开帮助卡

可能触发的新提示 / 新卡：

- 基本没有后续 async 旁路

`#267` 归属判断：

- 不是持续业务流
- 不值得纳入主治理面

### 4.3 `/list` `/use` `/useall`

当前主承载：

- target picker owner card
- `#266` 之后，短路径与 Git 长路径的主异步结果都已能回收到同一张 owner card

关键状态变化：

- 打开 picker
- mode/source/workspace/session 切换
- 打开 path picker 子步骤
- confirm `已有工作区 -> 会话`
- confirm `已有工作区 -> 新会话`
- confirm `添加工作区 -> 本地目录`
- confirm `添加工作区 -> Git URL`
- pending headless 成功 / 失败 / 超时
- thread selection changed

可能触发的新提示 / 新卡：

- target picker 同卡 patch
- path picker 子步骤卡
- owner card processing / terminal patch
- Git flow stale notice
- block / busy / expired / unauthorized notice
- pending headless timeout notice

关键证据：

- `internal/core/orchestrator/service_target_picker_owner_card.go`
- `internal/core/orchestrator/service_target_picker_add_workspace.go`
- `internal/core/orchestrator/service_target_picker_git_import.go`
- `internal/core/orchestrator/service_surface.go`
- `internal/core/orchestrator/service_snapshot_runtime.go`

当前已做对的一部分：

- 短路径已开始把 notice 吃回 owner card
- `maybeFinalizePendingTargetPicker(...)` 已明确在吸收：
  - `UIEventNotice`
  - `UIEventThreadSelectionChange`
  - pending headless timeout / command reject / attach failure 等异步结果
- `#266` 已把 Git confirm 后的 started / clone failure / post-clone attach failure / success / cancel 收回 owner card

当前还没做对的部分：

- 剩下的 target picker 外发提示，更多是：
  - direct invalid / expired / blocker 反馈
  - flow stale 降级提示
  - restore / transport / gateway failure 这类外围 runtime 提示
- 也就是说，target picker 已不再是 `#267` 第一批里“最破碎”的 owner-flow；更适合作为 residual mop-up 面，而不是第一批 first adopter

`#267` 归属判断：

- thread switch / new-thread ready / local attach 成败 / prepare timeout
  - 默认属于 `owner-flow`
- Git clone / attach / prepare 期间的 started / progress / failure
  - 若继续沿 `#266` owner-card 方向，也应归 `owner-flow`
- transport degraded / gateway failure / detached recovery
  - 属于 `global runtime`

### 4.4 `/history`

当前主承载：

- history owner card

关键状态变化：

- 打开 loading 卡
- async query 返回成功
- async query 返回失败
- 页码切换
- 列表/详情切换
- 当前 thread 失效
- 旧卡 / 非 owner 点击

可能触发的新提示 / 新卡：

- 同卡 loading -> resolved patch
- 同卡 loading -> error patch
- expired / unauthorized notice

关键证据：

- `internal/core/orchestrator/service_thread_history_view.go`

当前特征：

- 这是当前最干净的一组
- 同卡 loading / resolved / error 已经基本成立

`#267` 归属判断：

- 历史查询成功/失败默认属于 `owner-flow`
- expired / unauthorized 则属于 UI freshness 防护，可以继续保留为独立 notice

### 4.5 `/sendfile`

当前主承载：

- path picker
- confirm 后主承载断裂

关键状态变化：

- 打开 file picker
- 浏览目录 / 选中文件
- confirm 发送
- cancel
- upload / send success
- upload / send failure

可能触发的新提示 / 新卡：

- path picker 卡
- `send_file_cancelled` notice
- `send_file_sent` notice
- `send_file_failed` / `upload_failed` / `not_found` notice

关键证据：

- `internal/core/orchestrator/service_send_file.go`
- `internal/app/daemon/app_send_file.go`

当前问题：

- picker 负责选文件
- 但确认之后真正的发送结果落到独立 notice
- 这使得“选择文件 -> 发送完成”不是一条稳定单卡流

`#267` 归属判断：

- 发送成功/失败/取消，默认都属于 `owner-flow`
- 这条流未来如果 owner-card 化，不应再由 notice 承担主结果

### 4.6 `/new`

当前主承载：

- 没有独立 owner card
- 主要靠 thread selection change + notice

关键状态变化：

- 进入 `new_thread_ready`
- 重复 `/new`
- 阻塞于 request capture / pending request / busy state
- 首条普通文本真正创建新 thread

可能触发的新提示 / 新卡：

- `new_thread_ready` notice
- `already_new_thread_ready` notice
- `new_thread_ready_reset` notice
- 各类 blocker notice
- 首条普通文本之后，转入 turn-owned 共享过程卡 / final card

关键证据：

- `internal/core/orchestrator/service_surface_actions.go`
- `docs/implemented/new-thread-command-design.md`

当前判断：

- `/new` 更像路由态切换，不是典型长业务流
- 但它确实会发出“已准备新会话”的提示，而且这类提示会改变用户对当前线程运行态的感知

`#267` 归属判断：

- 当前仍可视为短流，先不强行 owner-card 化
- 但它发出的 notice 应避免和“已切换到现有 thread”语义混淆

### 4.7 `/follow` `/detach`

当前主承载：

- 即时 notice / 状态切换

关键状态变化：

- follow 成功 / 失败
- detach 成功 / busy / 旧卡点击

可能触发的新提示 / 新卡：

- 结果 notice
- 旧卡失效 notice

当前判断：

- 更像 surface routing / ownership 切换
- 不是 `#267` 的主战场

### 4.8 `/status`

当前主承载：

- snapshot 卡

关键状态变化：

- 主动查看状态

可能触发的新提示 / 新卡：

- 快照卡自身

当前判断：

- 只读状态，不是“旁路提示把 owner card 顶走”的主要来源

### 4.9 `/stop` `/compact` `/steerall`

当前主承载：

- `/stop`
  - 即时动作 + notice
- `/compact`
  - 共享过程卡 / compact 完成 progress
- `/steerall`
  - 提交锚点卡 + 之后继续 append 结果

关键状态变化：

- `/stop` 清队列 / 停止运行态
- `/compact` 开始整理 / 完成整理 / surface 不在时 replay compact
- `/steerall` 提交 / accepted / rejected / restore

可能触发的新提示 / 新卡：

- stop 相关 notice
- compact 的共享过程卡
- compact completion replay
- steer 提交锚点卡
- `steer_failed` notice

关键证据：

- `internal/core/orchestrator/service_compact_notice.go`
- `internal/core/orchestrator/service_snapshot_runtime.go`
- `internal/app/daemon/app_ingress.go`

当前判断：

- `/compact` 与 turn-owned 过程卡绑定更强
- `/steerall` 则更像“命令提交锚点”的遗留模型

`#267` 归属判断：

- `/compact` 更应归 `turn-owned`
- `/steerall` 是否需要 owner 化，要看是否继续保留锚点卡模型

### 4.10 参数卡 6 项

范围：

- `/mode`
- `/autowhip`
- `/reasoning`
- `/access`
- `/model`
- `/verbose`

当前主承载：

- 打开时是 `command.catalog`
- 应用后大多退化成独立 notice

关键状态变化：

- 打开配置卡
- 点选立即切换
- attachment required
- busy / usage error

可能触发的新提示 / 新卡：

- 配置卡
- “已更新设置” notice
- “当前已是该值” notice
- attachment-required catalog
- busy / usage error notice

当前问题：

- 它们是最典型的“打开像卡片流，提交却退回 notice”

`#267` 归属判断：

- 这类短流如果未来做统一收口，优先判为 `owner-flow`
- 但优先级低于 target picker / sendfile / Git 长路径

### 4.11 `/cron`

当前主承载：

- menu / status / list / edit 都是直接目录卡
- mutating command 目前走 started notice + async result notice/catalog

关键状态变化：

- 打开 Cron 菜单
- 查看状态 / 列表 / 配置
- repair / reload / run started
- repair / reload / run completed / failed

可能触发的新提示 / 新卡：

- Cron command catalog
- `cron_*_started` notice
- `cron_command_failed` notice
- 成功后的 catalog 或 ready notice

关键证据：

- `internal/app/daemon/app_cron_ui.go`
- `internal/app/daemon/app_cron_commands.go`

当前判断：

- 这是明确的“后台操作 + 后补结果”
- 当前更像管理面，而不是业务 owner-flow

`#267` 归属判断：

- 先不建议纳入第一批 owner-card 治理
- 但 started / result 至少应有稳定的同一条操作链语义

### 4.12 `/debug` `/upgrade`

当前主承载：

- status / track 是目录卡或 notice
- mutating / async 路径大量依赖 started notice + goroutine / restart 后补 notice

关键状态变化：

- `/debug admin` 准备外链
- admin 外链生成成功 / 失败
- `/upgrade latest` / `dev` 检查开始
- 候选可用 / 已最新 / 检查失败
- 升级准备开始
- 重启后结果扫描完成

可能触发的新提示 / 新卡：

- track / status catalog
- `debug_admin_prepare_started` notice
- `debug_admin_link_ready` / failed notice
- `upgrade_check_started` / `upgrade_dev_check_started` notice
- `upgrade_*_candidate_pending` / latest / failed notice
- `upgrade_prepare_started` notice
- 重启后 `debug_upgrade_result` notice

关键证据：

- `internal/app/daemon/app_upgrade.go`
- `internal/app/daemon/app_upgrade_execute.go`
- `internal/app/daemon/app_upgrade_dev.go`

当前问题：

- 这是另一条很强的“结果不在当前交互点上返回”的业务
- 尤其升级结果依赖重启后定期扫描状态，再反向补发 notice

`#267` 归属判断：

- 这组现在更适合保留为独立管理链路
- 不建议直接塞进当前 owner-card 治理第一阶段

## 5. 非 slash/menu 发起，但会打断 owner 焦点的 turn 派生流

虽然这轮主表按 slash/menu 分组，但真正会“把用户注意力顶走”的，还必须额外记住 4 条 turn 派生链。

### 5.1 request / approval 卡

触发源：

- agent 通过 request prompt 要求确认、补充输入或授予权限

关键证据：

- `internal/core/orchestrator/service_request.go`

当前判断：

- 这组应继续归 `turn-owned`
- 但如果未来有业务 owner card 正在 running，需要明确 request 卡和业务 owner card 谁是当前主问题

### 5.2 共享过程卡

触发源：

- `exec_command`
- `web_search`
- `mcp_tool_call`
- `dynamic_tool_call`
- `compact`

关键证据：

- `internal/core/orchestrator/service_exec_command_progress.go`
- `internal/core/orchestrator/service_compact_notice.go`

当前判断：

- 这组本质上是 turn runtime 的可视化
- 不应被某个 launcher 业务随意吞掉
- 但当某条业务本身明确承接 running 态时，需要做“谁是主承载”的仲裁

### 5.3 final reply / overflow / image output

触发源：

- turn 完成
- 图片输出完成

关键证据：

- `internal/core/orchestrator/service_queue.go`
- `internal/adapter/feishu/projector.go`

当前判断：

- 默认继续归 `turn-owned`
- 如果业务 owner card 自己也想展示最终结果，需要先定义两者边界，而不是让两套结果都抢主卡

### 5.4 thread replay

触发源：

- surface 不在 turn 上时暂存 final / notice / compact
- 后续 re-attach thread 再补发

关键证据：

- `internal/core/orchestrator/service_replay.go`

当前判断：

- 这是延迟发卡，不是当前业务步骤本身
- 默认归 `turn-owned`

## 6. 对 `#267` 的结论

### 6.1 这张单不应再只写“消息分层规则”

如果只写：

- 哪些提示回 owner card
- 哪些提示独立发

还是不够，因为真正决定行为的是“触发源”。

更准确的治理对象应该是：

1. `owner-flow` 自己的异步完成 / 失败 / 超时
2. `turn-owned` 的 request / progress / final
3. `global runtime` 的恢复 / 降级 / shutdown / gateway failure

### 6.2 第一批最值得进 `#267` 的，不是 12 组全改

更现实的第一批是：

1. `/sendfile`
2. target picker 的 residual owner-flow 提示
3. 其它仍已明确 owner-card 化、但结果还靠 notice 承载的短流

原因：

- `/sendfile` 现在仍是“path picker 负责选文件，confirm 后结果全靠独立 notice”，主承载断裂最明显
- target picker 在 `#264` + `#266` 后主异步结果已经大体收口，更适合作为顺手清 residual 的第二优先级，而不是第一优先级
- 这样可以先用一条新的业务流验证 `#267` 的规则，而不是反复在已经基本修顺的 target picker 上做小修小补

### 6.3 `turn-owned` 与 `global runtime` 要单独留出车道

不建议把下面这些东西硬塞回当前业务 owner card：

- request / approval 卡
- 共享过程卡
- final reply / overflow
- VS Code resume prompt
- transport degraded
- daemon shutdown
- gateway apply failed

否则会得到另一种混乱：

- 当前业务 owner card 里突然出现并不属于它的全局异常

### 6.4 这件事适合分阶段，不适合一把写穿

推荐拆成 3 段：

1. `phase A`
   - 只治理已有或明确要 owner-card 化的业务流旁路提示回收
   - 第一优先级改成 `/sendfile`
   - target picker 只处理剩余 residual 提示，不再把 Git 长路径当成第一主战场
2. `phase B`
   - 处理 owner-flow 与 turn-owned 过程卡之间的主承载仲裁
   - 重点是 running owner-flow 与 request / progress / final 的边界
3. `phase C`
   - 清理 global runtime 提示车道
   - 重点是 transport / resume / shutdown / gateway failure 的独立展示策略

## 7. 建议回写到 issue 的最小执行点

`#267` 下一步至少应明确 3 件事：

1. 第一批只做哪些 `owner-flow`
   - 当前建议：先做 `/sendfile`
   - target picker 只补 residual 提示，不再作为第一批主验证面
2. 对 `turn-owned` 的保留边界是什么
   - request / progress / final / replay 先不吞回业务 owner card
3. 对 `global runtime` 的独立展示边界是什么
   - transport degraded / shutdown / resume / gateway failure 继续独立

只有这 3 件事先拍稳，后面实现才不会在一个 patch 里混进三套完全不同的语义。
