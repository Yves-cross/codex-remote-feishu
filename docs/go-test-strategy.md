# Go 重写测试策略

## 1. 目标

测试第一优先级是全自动化。

这意味着：

- 单元测试要覆盖核心状态机
- 协议翻译要有 contract test
- wrapper/daemon/bot 的关键链路要有 integration test
- 至少有一组端到端测试使用“行为正确的 mock Codex”和“行为正确的 mock Feishu”

## 2. 分层策略

## 2.1 纯单元测试

覆盖模块：

- `core/agentproto`
- `core/control`
- `core/render`
- `core/state`
- `core/orchestrator`
- `core/renderer`

重点断言：

- attach 后默认 pin 当前 thread
- queue item 入队时冻结 thread/cwd
- `local.interaction.observed` 触发 `paused_for_local`
- `turn.completed(local_ui)` 进入 `handoff_wait`
- handoff 超时后恢复 remote queue
- stop 只清飞书侧 queue / staged image
- staged image 跟随第一条文本绑定
- renderer 对 fenced code block / 文件列表分块正确

## 2.2 协议 contract test

覆盖模块：

- `adapter/codex`
- `adapter/relayws`

重点断言：

- 原生 `turn/start` -> canonical `local.interaction.observed(turn_start)`
- 原生 `turn/steer` -> canonical `local.interaction.observed(turn_steer)`
- 远端 `prompt.send` -> 正确生成 `thread/start` / `thread/resume` / `turn/start`
- `turn.started` 的 `initiator` 判定正确
- reconnect 时 `instanceId` 复用规则正确

## 2.3 适配层集成测试

覆盖模块：

- `adapter/feishu`
- `app/daemon`
- `app/wrapper`

重点断言：

- 飞书菜单 / 消息 / reaction / 图片事件被正确翻成 `SurfaceAction`
- notice / render block / pending state 被正确发回飞书接口
- wrapper 和 daemon 之间 WS 断线重连正确

## 2.4 端到端测试

使用：

- `testkit/mockcodex`
- `testkit/mockfeishu`
- `testkit/harness`

场景至少包括：

1. `/list -> attach -> 发文本 -> 收到完整文本 block`
2. `attach 后本地 VS Code 发起 turn/start，飞书进入 paused_for_local`
3. `本地 turn 完成，handoff_wait 后飞书队列恢复`
4. `发两张图片 + 一条文本，prompt.send(inputs[])` 顺序正确
5. `飞书 queue 中追加两条消息，只有 dispatching 项有 Typing`
6. `stop -> interrupt 当前 turn + discard 剩余 queue`
7. `切 thread 后新消息进入新 thread，旧 queue 不被改写`

## 3. Mock 设计约束

## 3.1 Mock Codex 必须是状态机，不是静态脚本

`mockcodex` 必须维护：

- loaded threads
- focused thread
- active turn
- interrupted / completed 状态
- thread cwd
- turn inputs

必须支持原生请求：

- `thread/start`
- `thread/resume`
- `thread/loaded/list`
- `thread/read`
- `turn/start`
- `turn/steer`
- `turn/interrupt`

必须发出的通知：

- `thread/started`
- `turn/started`
- `item/started`
- `item/agentMessage/delta`
- `item/completed`
- `turn/completed`

## 3.2 Mock Feishu 必须记录真实交互副作用

`mockfeishu` 必须记录：

- 收到的文本
- 收到的卡片
- 添加/移除 reaction
- 下载图片请求
- 菜单点击和按钮回调

测试要断言副作用顺序，而不是只断言“调用过一次”。

## 4. 通过标准

第一版完成标准冻结为：

1. `go test ./...` 全绿
2. 关键 e2e 场景全绿
3. 不依赖手工点按钮或人工观察日志判断正确性
4. mock 不使用与真实协议矛盾的偷懒行为

## 5. 开发顺序

实施时按 TDD 推进：

1. 先写纯领域测试
2. 再写 protocol contract test
3. 再写 integration/e2e harness
4. 最后补 installer 测试
