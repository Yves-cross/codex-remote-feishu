# Relay Protocol Spec

## 1. 文档定位

这份文档描述的是**当前仓库已经实现的协议和内部模型**。

当前真实存在的三层边界是：

1. `Codex app-server native protocol`
   - VS Code / Codex 扩展和 `relay-wrapper` 之间
   - JSONL / JSON-RPC request-response + notification
2. `relay.agent.v1`
   - `relay-wrapper` 和 `relayd` 之间
   - WebSocket + JSON envelope
3. `control.Action` / `control.UIEvent`
   - `relayd` 进程内部的控制与渲染模型
   - 当前**不是**公开网络协议

需要特别说明：

- 当前仓库没有实现公开的 `relay.control.v1` HTTP API
- 当前仓库也没有实现公开的 `relay.render.v1` 拉流 API
- 飞书控制和投影是在 `relayd` 进程内完成的

## 2. Native 协议边界

`relay-wrapper` 针对 Codex app-server 观测并翻译下面这些原生信号：

- `thread/start`
- `thread/resume`
- `thread/name/set`
- `thread/list`
- `thread/read`
- `thread/started`
- `thread/name/updated`
- `turn/start`
- `turn/steer`
- `turn/interrupt`
- `turn/started`
- `turn/completed`
- `item/started`
- `item/completed`
- `item/*/delta`

当前实现中的关键规则：

- wrapper 只负责协议翻译和显式标注
- wrapper 可以 suppress 自己主动注入的原生命令响应
  - 例如远端 `prompt.send` 触发的内部 `thread/start`、`thread/resume`、`turn/start`
- wrapper 不允许因为 helper/internal traffic 而吞掉真实 runtime lifecycle event
  - helper 生命周期必须继续翻译成 canonical event，并显式带上 `trafficClass`

## 3. `relay.agent.v1`

### 3.1 协议名

当前固定为：

```json
{
  "protocol": "relay.agent.v1"
}
```

### 3.2 Envelope 类型

当前实现的 envelope 类型定义在 [wire.go](../internal/core/agentproto/wire.go)：

- `hello`
- `welcome`
- `event_batch`
- `command`
- `command_ack`
- `error`
- `ping`
- `pong`

当前实际主链路使用的是：

- `hello`
- `welcome`
- `event_batch`
- `command`
- `command_ack`
- `error`

### 3.3 `hello`

wrapper 建立连接后首先发送：

```json
{
  "type": "hello",
  "hello": {
    "protocol": "relay.agent.v1",
    "instance": {
      "instanceId": "inst-67f7045577c78c7a",
      "displayName": "dl",
      "workspaceRoot": "/workspace/demo",
      "workspaceKey": "/workspace/demo",
      "shortName": "demo"
    },
    "capabilities": {
      "threadsRefresh": true
    }
  }
}
```

### 3.4 `welcome`

`relayd` 接收 `hello` 后返回：

```json
{
  "type": "welcome",
  "welcome": {
    "protocol": "relay.agent.v1"
  }
}
```

### 3.5 `event_batch`

wrapper 向 `relayd` 上送 canonical event 时使用：

```json
{
  "type": "event_batch",
  "eventBatch": {
    "instanceId": "inst-67f7045577c78c7a",
    "events": [
      {
        "kind": "turn.started",
        "threadId": "thread-1",
        "turnId": "turn-1",
        "status": "running"
      }
    ]
  }
}
```

### 3.6 `command`

`relayd` 向 wrapper 下发 canonical command 时使用：

```json
{
  "type": "command",
  "command": {
    "commandId": "cmd-1",
    "kind": "threads.refresh"
  }
}
```

### 3.7 `command_ack`

wrapper 收到 `command` 后总是回传 accept/reject：

```json
{
  "type": "command_ack",
  "commandAck": {
    "instanceId": "inst-67f7045577c78c7a",
    "commandId": "cmd-1",
    "accepted": true
  }
}
```

## 4. Canonical Command

当前只实现四个公共 command：

- `prompt.send`
- `turn.interrupt`
- `request.respond`
- `threads.refresh`

这四个 command 的真实定义在 [types.go](../internal/core/agentproto/types.go)。

### 4.1 `prompt.send`

关键字段：

- `origin`
  - `surface`
  - `userId`
  - `chatId`
  - `messageId`
- `target`
  - `threadId`
  - `cwd`
  - `createThreadIfMissing`
- `prompt.inputs[]`
  - `text`
  - `local_image`
  - `remote_image`
- `overrides`
  - `model`
  - `reasoningEffort`

### 4.2 `turn.interrupt`

关键字段：

- `target.threadId`
- `target.turnId`

### 4.3 `request.respond`

用于将 approval / structured response 回写给 native request id。

### 4.4 `threads.refresh`

触发 wrapper 走 `thread/list + thread/read`，再返回标准化的 `threads.snapshot`。

## 5. Canonical Event

当前事件类型：

- `threads.snapshot`
- `thread.discovered`
- `thread.focused`
- `config.observed`
- `local.interaction.observed`
- `turn.started`
- `turn.completed`
- `item.started`
- `item.delta`
- `item.completed`
- `request.started`
- `request.resolved`

### 5.1 关键字段

#### `initiator`

当前使用：

- `remote_surface`
- `local_ui`
- `internal_helper`
- `unknown`

#### `trafficClass`

当前使用：

- `primary`
- `internal_helper`

这两个字段共同决定：

- turn 是否应进入远端 queue 状态机
- item 是否应进入 Feishu 主渲染面
- 本地交互是否应触发 `paused_for_local`

### 5.2 Helper/Internal traffic 规则

当前冻结规则：

- `ephemeral`
- `persistExtendedHistory`
- `outputSchema`

只能影响：

- wrapper 的模板复用
- canonical event 上的 `trafficClass=internal_helper`
- canonical event 上的 `initiator=internal_helper`

不能直接导致 wrapper 吞掉下面这些生命周期事件：

- `thread.discovered`
- `turn.started`
- `item.*`
- `turn.completed`

## 6. 当前实现中的内部控制与渲染模型

虽然当前没有公开的 `relay.control.v1` / `relay.render.v1` 网络协议，但这两层语义已经稳定存在于进程内模型中。

### 6.1 Inbound control

飞书入口最终被归一到 [control.Action](../internal/core/control/types.go)：

- `surface.menu.list_instances`
- `surface.menu.status`
- `surface.menu.stop`
- `surface.command.model`
- `surface.command.reasoning`
- `surface.message.text`
- `surface.message.image`
- `surface.message.reaction.created`
- `surface.button.attach_instance`
- `surface.button.show_threads`
- `surface.button.use_thread`
- `surface.button.follow_local`
- `surface.button.detach`

### 6.2 Outbound UI events

`orchestrator` 输出 [control.UIEvent](../internal/core/control/types.go)，再由 Feishu projector 映射成文本、卡片和 reaction：

- `snapshot.updated`
- `selection.prompt`
- `pending.input.state`
- `notice`
- `thread.selection.changed`
- `block.committed`
- `agent.command`

## 7. 当前不暴露的能力

下面这些在当前仓库里**没有作为公共协议暴露**：

- 公开的 attach/detach/use-thread HTTP API
- 公开的 render event 拉流 API
- 远端 `turn.steer`
- block update / replace
- native frame debug/replay export

如果后续真的需要对外开放控制面，应该在现有 `control.Action` / `control.UIEvent` 基础上重新设计，而不是继续使用旧文档里的 `/v1/users/:userId/...` 形式。
