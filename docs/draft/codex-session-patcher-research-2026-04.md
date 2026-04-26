# Codex Session Patcher Research 2026-04

> Type: `draft`
> Updated: `2026-04-26`
> Summary: 归档 `codex-session-patcher` 调研结论，拆清会话 patch、外部 LLM 改写、CTF 提示词注入三层能力，并给出本仓库的吸收建议。

## 1. 文档定位

本文用于沉淀这次对 `ryfineZ/codex-session-patcher` 的调研结论。

它回答 5 个问题：

1. 这个项目实际做了哪些事。
2. 它分别如何作用于 Codex CLI、Claude Code 和 OpenCode。
3. 没有外接 AI 模型时还能用到什么程度。
4. 它和本仓库当前想做的“当前 thread patch 事务”有什么本质差异。
5. 如果本仓库要吸收类似能力，应该吸收哪一层，不该照搬哪一层。

相关上下文：

- 外部仓库：`https://github.com/ryfineZ/codex-session-patcher`
- 现有 issue：`#189 [Feature] 当前 thread 本地补丁事务（latest_turn / entire_thread）`

## 2. 先给结论

结论可以先压缩成三句：

1. `codex-session-patcher` 不是单一“patch 工具”，而是把 `离线改会话`、`外部 LLM 改写`、`CTF 提示词注入` 三类能力揉在了一起。
2. 对本仓库真正有参考价值的是第 1 类里的“备份 -> patch -> 恢复继续”的产品概念，而不是它当前直接改本地文件的实现方式。
3. 本仓库如果要做类似能力，V1 应该优先做“当前 attached thread 的最新 assistant turn 事务化 patch”，不要一开始就把 AI 改写、整线程 patch、复杂 UI 一起打包。

## 3. 外部项目实际能力拆解

## 3.1 离线会话 patch

这是它最核心、也最容易被误认为“全部能力”的一层。

它会直接修改本机落盘的会话数据：

- Codex CLI
  - 扫描 `~/.codex/sessions/*.jsonl`
  - 定位 assistant 输出
  - 替换拒绝回复
  - 删除独立的 `reasoning` 行
  - 同步改关联的 `event_msg` 冗余副本
- Claude Code
  - 扫描 `~/.claude/projects/**/*.jsonl`
  - 定位 `assistant.message.content[]`
  - 替换 `text`
  - 移除嵌入式 `thinking` block
- OpenCode
  - 直接修改 `~/.local/share/opencode/opencode.db`
  - 更新 `part` 表里的 `text`
  - 删除 `reasoning` part

配套能力还包括：

- 会话扫描和搜索
- 预览和 diff
- 自动备份
- 备份还原
- 批量处理多个会话

这层是纯本地存储 patch，不接管运行中的 agent 生命周期，也不处理多 surface、多队列、多实例状态。

## 3.2 外部 LLM 辅助改写

这是它的第二层能力。

它支持配置 OpenAI 兼容接口：

- `ai_endpoint`
- `ai_key`
- `ai_model`

然后做两件事：

1. 根据拒绝前的上下文，生成更自然的 assistant 替换文本。
2. 把用户原始请求改写成更容易被接受的提示词。

这一层不是 patch 事务的必需条件，而是“让替换后的文本更像真对话”的增强层。

没有这层时，工具仍然能运行，只是会退化成固定模板替换。

## 3.3 CTF / 渗透测试提示词注入

这是第三层能力，也是它项目定位里最重的一层。

它不是在 patch 当前会话，而是在改新会话的默认上下文：

- Codex CLI
  - 写 `~/.codex/prompts/*.md`
  - 更新 `~/.codex/config.toml`
  - 支持 `codex -p ctf`
  - 也支持全局注入 `model_instructions_file`
- Claude Code
  - 创建 `~/.claude-ctf-workspace/.claude/CLAUDE.md`
  - 让用户从这个专用 workspace 启动 `claude`
- OpenCode
  - 创建 `~/.opencode-ctf-workspace/AGENTS.md`
  - 创建 `opencode.json`

这层的目标是“降低未来被拒绝的概率”，而不是“修复当前已拒绝的线程”。

## 3.4 Web UI

它的 Web UI 主要是把前三层能力可视化：

- 会话列表
- 过滤和搜索
- 预览和 diff
- patch / restore
- AI 改写
- 提示词模板编辑
- CTF 配置安装与状态检查

因此，这个项目的 UI 不是事务编排器，而是本地工具控制台。

## 4. 它分别如何作用于 Codex 和 Claude

## 4.1 对 Codex CLI

它对 Codex 的作用更像：

- 一个 `rollout JSONL patcher`
- 一个 `profile / global prompt` 注入器
- 一个可选的外部 AI prompt rewriter

它不做这些事：

- 不接管运行中的 child stop/start
- 不维护 `thread/resume` 协议恢复时序
- 不处理队列门禁
- 不处理 remote surface 的用户可见噪音

所以它和本仓库的 wrapper / daemon 编排层不是同一层系统。

## 4.2 对 Claude Code

它对 Claude Code 的作用更像：

- 修改本地 JSONL 会话
- 在专用 workspace 下写 `CLAUDE.md`
- 让用户从那个 workspace 新开 Claude 会话

它对 Claude 不是“运行时事务恢复”，而是“工作区上下文塑形 + 离线会话改写”。

如果把这套思路直接照搬到本仓库，会缺掉最关键的运行时收口：

- 当前 instance 是否忙
- patch 期间是否还允许进新输入
- patch 后如何恢复当前 thread
- 多 surface 如何看到一致状态

## 5. 没有外接 AI 模型时还能不能用

可以用，但只能用到第 1 层和第 3 层。

可用部分：

- 会话扫描
- refusal 检测
- 固定模板替换
- reasoning / thinking 清理
- 备份和恢复
- Codex / Claude / OpenCode 的 CTF prompt 注入

不可用部分：

- AI 生成替换文本
- AI 改写用户 prompt

实际影响：

- 替换后的文本会退化成固定模板，语境贴合度差很多。
- 未来新会话仍可通过 prompt 注入获得“更容易接受”的默认上下文。
- 但这不保证模型一定不再拒绝，只是少了一层动态改写辅助。

如果进一步假设“连 Codex / Claude 自己都没有可用模型后端”，那这个项目只剩本地文件修改意义，几乎没有完整使用价值。

## 6. 为什么它不能直接作为本仓库的实现方案

本仓库和它最大的差异不在存储格式，而在运行时结构。

`codex-session-patcher` 默认假设：

- 用户直接在本机上操作单个 CLI 会话
- patch 时不需要接管运行态 child
- patch 时没有外部 surface 持续发请求
- patch 后用户自己回去 resume 即可

本仓库真实要处理的是：

- wrapper child 生命周期
- daemon / relay / surface 三层状态同步
- `thread/resume` 恢复链路
- `pendingRemote / activeRemote / pendingSteers`
- queued / dispatching / running 门禁
- 对 Feishu 用户尽量无噪音

所以这里不能只做“改文件”。

真正需要的是：

1. 事务前检查
2. surface dispatch 暂停
3. child stop / restart 编排
4. 存储后端 patch
5. `thread/resume` 恢复
6. 失败回滚和统一用户提示

## 7. 本仓库应该吸收哪一层

推荐只吸收这三个产品思想：

1. patch 前自动备份
2. patch 后继续在同一个 thread 上工作
3. AI 辅助改写必须是可选增强层，不要和基础事务绑死

不推荐直接照搬的部分：

- 直接对当前本地会话文件做无编排热改
- 把“提示词注入”和“当前 thread patch”混成一个 feature
- 让 V1 同时支持 Codex / Claude / OpenCode 三条实现线

## 8. 对 `#189` 的启发

`#189` 的大方向没有错。

它想解决的是：

- 当前 thread 的受控 patch
- patch 完成后继续在同一 thread 上工作
- patch 期间尽量不让用户看到离线/恢复噪音

这比外部项目更适合本仓库，因为本仓库已经具备部分必要底座：

- child restart 后自动 `thread/resume`
- surface dispatch pause / resume
- persisted thread catalog

但 `#189` 目前仍然太宽，混合了：

- `latest_turn`
- `entire_thread`
- 后端自适应
- 恢复时序
- 用户体验

这也是为什么更适合先拆一个单独的 V1。

## 9. 推荐的吸收路径

推荐分三层推进：

### 9.1 V1

只做：

- 当前 attached thread
- 最新 assistant turn
- 事务化 patch
- 备份、回滚、恢复

不做：

- AI 改写
- entire_thread
- 任意 turn
- 可视化 diff 工作流

### 9.2 V2

在 V1 稳定后再加：

- 更自然的 patch 文本生成
- diff / rollback UI
- 更强的 patch 范围表达

### 9.3 V3

如果产品价值成立，再考虑：

- 新会话级的上下文塑形
- 与 Claude / 其他 backend 的统一抽象

## 10. 最终判断

这次调研后的最终判断是：

- `codex-session-patcher` 值得参考，但参考点是产品概念，不是具体实现。
- 本仓库当前更应该做“事务化当前 thread patch 底座”，而不是“本地多平台会话清理器”。
- V1 要先把“当前 thread 的上一轮怎么修、修完后用户看到什么”定义清楚，再进入编码。
