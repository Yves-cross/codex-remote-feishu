# Go 重写架构设计

## 1. 目标

这次 Go 重写的目标不是把 Node/Rust 版本逐文件翻译，而是基于已冻结的产品和协议设计，重建一个：

- 进程数更少
- 模块边界更清晰
- 测试更容易自动化
- 运行时行为更可解释

的实现。

旧实现源码快照已归档到：

`/data/dl/fschannel-archive-20260403-234311`

后续实现可以参考旧代码，但不得反向把旧的耦合方式带回来。

## 2. 进程架构

Go 版冻结为三个二进制：

1. `relayd`
   - 统一承载原 `server + bot`
   - 管理 wrapper 连接、产品状态、renderer、Feishu 网关
2. `relay-wrapper`
   - 被 VS Code / Codex 扩展调用
   - 负责 Codex app-server 原生协议适配
3. `relay-install`
   - 负责安装、配置、编辑器集成、部署

这意味着：

- 逻辑上仍保留 `wrapper / server / bot` 三层边界
- 实现上把 `server + bot` 合并进同一 Go 进程
- 减少跨语言和跨进程同步成本

## 3. 目录结构

冻结目录布局如下：

```text
cmd/
  relayd/
  relay-wrapper/
  relay-install/

internal/
  app/
    daemon/
    wrapper/
    install/
  core/
    agentproto/
    control/
    render/
    state/
    orchestrator/
    renderer/
  adapter/
    codex/
    relayws/
    feishu/
    editor/
    process/
  config/
  logging/

testkit/
  mockcodex/
  mockfeishu/
  harness/
```

## 4. 依赖规则

必须满足下面的依赖方向：

```text
cmd -> internal/app -> internal/adapter + internal/core
internal/adapter -> internal/core
internal/core -> internal/core
testkit -> internal/core + internal/adapter
```

明确禁止：

- `internal/core` 反向依赖 `internal/adapter`
- `cmd/*` 直接编排具体业务逻辑
- `adapter/feishu` 直接操作 orchestrator 内部状态
- `adapter/codex` 直接夹带飞书产品语义

## 5. 模块职责

## 5.1 `internal/core/agentproto`

定义统一 canonical protocol：

- wrapper -> daemon 的 event / command model
- JSON 编解码
- versioning / envelope

这里不包含业务状态机。

## 5.2 `internal/core/control`

定义用户层控制语义：

- surface action
- snapshot
- selection prompt
- pending input state
- notice

这里不包含 Feishu SDK 细节。

## 5.3 `internal/core/render`

定义 renderer 输出模型：

- committed block
- block kind
- render exposure

目标是让 bot 网关只消费 render block，不自己猜文本边界。

## 5.4 `internal/core/state`

定义领域状态：

- `InstanceRecord`
- `ThreadRecord`
- `SurfaceConsoleRecord`
- `QueueItemRecord`
- `StagedImageRecord`
- `PendingRequestRecord`

状态对象不直接操作网络。

## 5.5 `internal/core/orchestrator`

这是系统的产品语义中心，负责：

- attach / detach
- routeMode / dispatchMode
- queue / staged image / reaction cancel
- local-priority arbitration
- request lifecycle
- prompt 路由冻结

这里是整个系统最关键的状态机模块。

## 5.6 `internal/core/renderer`

负责 assistant 文本切分：

- item 强边界
- fenced code block
- 文件列表 block
- 引导语 / 正文 / 尾注

输出 append-only committed block。

## 5.7 `internal/adapter/codex`

Codex app-server 适配层，负责：

- 观测原生 `thread/*`, `turn/*`, `item/*`
- 识别本地 `turn/start` / `turn/steer`
- 翻译为 canonical event
- 把 canonical command 翻译回原生命令

这里不做 attach、queue、render。

## 5.8 `internal/adapter/relayws`

负责 wrapper 和 daemon 之间的 WebSocket 传输：

- wrapper client
- daemon side hub
- batch envelope IO

这里不做业务决策。

## 5.9 `internal/adapter/feishu`

负责飞书平台适配：

- 长连接事件接收
- 发文本 / 发卡片 / reaction
- 下载图片
- 菜单事件、消息事件、按钮回调转成 `SurfaceAction`

这里不保存产品状态，只调用 orchestrator。

## 5.10 `internal/adapter/editor`

负责编辑器集成：

- VS Code / Cursor settings 写入
- managed shim 安装
- 路径探测

## 5.11 `internal/adapter/process`

负责部署与运行：

- `systemd-user`
- repo pidfile
- 进程状态探测

## 5.12 `internal/app/daemon`

负责把下面这些拼起来：

- config
- relay websocket hub
- orchestrator
- renderer
- feishu gateway
- optional local HTTP status API

## 5.13 `internal/app/wrapper`

负责：

- 启动真实 Codex 可执行
- 建立 stdio proxy
- 连接 relayd
- 把 codex adapter 和 relayws client 组装起来

## 5.14 `internal/app/install`

负责：

- bootstrap
- configure-wrapper
- configure-services
- integrate-editor
- deploy/start/stop/status/uninstall

## 6. 关键运行时流

## 6.1 远端 prompt

```text
Feishu event
  -> adapter/feishu
  -> core/control SurfaceAction
  -> core/orchestrator enqueue/freeze route
  -> agentproto command prompt.send
  -> adapter/relayws
  -> relay-wrapper
  -> adapter/codex
  -> Codex app-server
  -> canonical event
  -> core/orchestrator / core/renderer
  -> render block / pending state
  -> adapter/feishu send
```

## 6.2 本地 VS Code 交互

```text
VS Code
  -> relay-wrapper
  -> adapter/codex observe turn/start or turn/steer
  -> core/agentproto local.interaction.observed
  -> relayd orchestrator paused_for_local
  -> Feishu notice
```

## 6.3 本地 turn 完成后恢复

```text
turn.completed(initiator=local_ui)
  -> orchestrator enters handoff_wait
  -> if local interaction reappears: stay paused_for_local
  -> else resume remote queue
```

## 7. 数据存储策略

第一版冻结为内存状态 + 配置文件。

原因：

- 当前需求主要是在线 relay 和明确状态机
- 不需要先引入数据库才能把协议和行为做正确

但状态设计要允许后续替换 store。

因此：

- `core/state` 不依赖具体持久化
- `orchestrator` 通过 store interface 操作状态
- 第一版提供 `memory` 实现

## 8. 配置策略

沿用之前已冻结的双配置模型：

- wrapper config
- services config

但改成 Go 实现读取与写入。

二进制默认行为：

- `relay-wrapper` 读取 wrapper config
- `relayd` 读取 services config
- `relay-install` 负责写两份配置并记录 install state

## 9. 实施顺序

重写顺序冻结为：

1. `core/agentproto`, `core/control`, `core/render`, `core/state`
2. `core/orchestrator`, `core/renderer`
3. `adapter/codex`, `adapter/relayws`
4. `app/daemon`, `app/wrapper`
5. `adapter/feishu`
6. `app/install`, `adapter/editor`, `adapter/process`

这个顺序的原则是：

- 先冻结纯领域逻辑
- 再补协议翻译
- 最后接入外部平台

## 10. 代码约束

实现时必须遵守：

1. 领域状态机优先写纯函数/可测试方法。
2. adapter 不得偷偷维护第二份业务状态。
3. renderer 只能消费 canonical item，不看 Feishu 平台细节。
4. mock Codex 必须维护真实 thread/turn 状态，不能做“收到文本就随便 echo”。
5. e2e 测试必须通过公开接口驱动，不能越过 adapter 直接改状态。
