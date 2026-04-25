# 会话描述统一方案

> Type: `implemented`
> Updated: `2026-04-25`
> Summary: 已落地 canonical thread DisplayName contract，统一非 `/status` 展示，并把 `/status` 收口为唯一受控例外。

## 背景

同一个 thread 过去会在 `/use`、切换提示、强踢确认、`/history`、`/status`、admin runtime surface 等入口显示成不同东西：有的用 rename/name，有的用首条 user，有的用最近 user，有的又会退回 assistant / preview / raw id。

这次实现的目标不是单点改文案，而是把“thread 到底显示什么”收口成一个统一 contract，并让各层不再各自 fallback。

## 最终 contract

### 1. Canonical `DisplayName`

实际 thread 的主显示名统一为：

`<workspace 最后一节> · <text>`

其中 `<text>` 优先级固定为：

1. 用户显式 rename 后的 `Name`
2. 最近一条用户消息前缀
3. 第一条用户消息前缀
4. 当以上都缺失时，固定退到 `未命名会话`

以下内容不再参与 thread 主显示名决策：

- `LastAssistantMessage`
- `Preview`
- `ThreadID`
- short id

### 2. 非 `/status` 场景

凡是告诉用户“当前是哪一个 thread / 正在选哪一个 thread / 要恢复哪一个 thread”的场景，都只显示 canonical `DisplayName`。

这包括：

- `/use` thread 选择
- VS Code thread 下拉
- attach / switch / follow notice
- kick prompt
- `/history` summary
- request prompt
- compact owner card
- admin runtime surface 的 thread 主标题

这些场景允许保留与 thread 身份无关的状态 meta，例如占用状态、相对时间、实例状态；但不再拼会话内容型副标题。

### 3. `/status` 例外

`/status` 是唯一允许额外显示 thread 上下文的受控例外。当前只展示：

- 当前会话：canonical `DisplayName`
- 会话起点：`FirstUserMessage`
- 最近用户：`LastUserMessage`

`/status` 不再显示 assistant/preview 作为 thread 身份补充。

### 4. 伪状态

以下不是实际 thread，继续保留独立文案，不强行套用 canonical 命名：

- `未绑定会话`
- `跟随当前 VS Code（等待中）`
- `新建会话（等待首条消息）`

## Contract 收口点

### Producer / helper

- `threadTitle / displayThreadTitle / threadSelectionButtonLabel` 现在统一走 canonical `DisplayName`
- preview/assistant 已移出主显示名链路
- persisted sqlite thread 会把旧库里的 `firstUserMessage` 映射到 `FirstUserMessage`，不再伪装成 `Name`

### Carrier

以下 carrier 已去掉对旧命名源的放权：

- `control.ThreadSelectionChanged`
- `control.FeishuThreadSelectionEntry`
- `control.AttachmentSummary`

其中：

- selection carrier 不再暴露 first/last/assistant/preview 让下游自己选名字
- snapshot attachment 只保留 `/status` 需要的 `FirstUserMessage / LastUserMessage`

### Consumer

以下消费面已切到新 contract：

- selection projector
- kick prompt
- `/history` summary
- request prompt
- compact owner card
- `/status`
- admin runtime surface

## Legacy 兼容

### 1. 旧线程 / 持久化线程

旧线程如果没有 rename/name，也没有 first/last user message，不再回退到 preview，而是显示：

`<workspace> · 未命名会话`

这样可以避免 assistant/preview 在不同入口再次拥有命名权。

### 2. Resume / headless restore

resume 存储仍保留旧标题清洗逻辑，用来去掉历史遗留的 workspace prefix 与 short-id suffix；但如果拿不到可复用的原始标题，不再退回 `threadID` 作为 `ResumeThreadTitle`。

恢复后的最终用户可见标题仍重新走当前 canonical contract。

## 验证面

当前回归覆盖至少包括：

- rename / 最近 user / 首条 user / 全缺失兜底
- 非 `/status` 场景不再使用 preview/assistant/id 命名
- VS Code `/use` 下拉
- kick prompt / compact owner card
- `/history`
- `/status`
- admin runtime surface
- resume / headless restore 兼容
