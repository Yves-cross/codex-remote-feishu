# Turn Diff Frozen Preview Design

> Type: `inprogress`
> Updated: `2026-04-24`
> Summary: 收敛只读 frozen turn diff viewer 的产品边界、页面交互、已确认 mock 细节和 snapshot 承载方案。

## 1. 背景

当前仓库已经把上游 `turn/diff/updated` 接成 authoritative `TurnDiffSnapshot`，并在 turn 结束时挂到 final block 上；但现有前台只消费了文件修改摘要，没有提供一条真正可读的 turn 级 diff 查看页。

最近这轮产品讨论已经把 `#307` 的方向收得比较清楚：

- 这条能力只服务于“看这一轮最终改了什么”
- 页面语义必须是 `turn-scoped`、`frozen`、`read-only`
- 它和未来 live workspace diff reviewer 完全切开
- 它不是 raw unified diff 文本页，而是一个“按文件切换、带上下文折叠”的只读 diff viewer

因此，这张单的重点不再是“如何把一段 patch 文本塞进现有 preview 页面”，而是：

- 如何把 authoritative turn diff 冻结成一个可信 snapshot
- 如何在不触碰 live workspace 的前提下，把这个 snapshot 渲染成可读的文件级 diff viewer
- 如何在风格上和现有 preview 页面保持一致，但在正文 renderer 上走专门路线

当前已确认的用户可见 mock 见：

- `docs/draft/turn-diff-frozen-preview-mock.html`

## 2. 产品定位

一句话定义：

> `#307` 是一个只读、冻结、按文件切换、带上下文折叠的 turn diff viewer；它不是 raw diff 文本页，也不是 live workspace reviewer。

更具体地说：

- source of truth 是 turn 结束时的 authoritative `TurnDiffSnapshot`
- 打开后看到的是“当时那一轮的冻结结果”，不是当前工作区此刻的真实状态
- 页面内不提供任何修改仓库状态的动作
- 页面不承载 `accept`、`reject`、`revert`、`stage`、`unstage` 等 reviewer 语义

## 3. 明确边界

### 3.1 与 `#144` / `#215` 的边界

`#144` / `#215` 的目标是 live workspace diff reviewer：

- 关注当前 Git 状态
- 后续可能支持 hunk 或 file 级动作
- 页面语义是“现在工作区是什么样”

`#307` 的目标不是这条路线：

- 不读 live workspace
- 不关心当前 Git 状态是否已经变化
- 不给任何动作入口
- 只展示“那一轮结束时的冻结 patch 视图”

这两条能力在产品、数据源和页面语义上都必须显式拆开，不互相借词，不互相预埋按钮。

### 3.2 非目标

- 不实现 `accept / reject / revert / stage / unstage`
- 不给 frozen viewer 增加“跳去 live reviewer”的入口
- 不把当前文件实时读盘结果混入这张页面
- 不把 raw unified diff 直接当成最终用户界面
- 不把这张页做成一份预烘焙、不可演进的静态 HTML 成品

## 4. 用户体验目标

### 4.1 用户要看到什么

用户不是只想看 patch 文本，而是想看：

- 这轮涉及了哪些文件
- 某个文件里到底改到了哪里
- 每个 hunk 周围的上下文是什么
- hunk 之间那些未修改内容是否可以先折叠、需要时再展开

因此页面默认应提供：

- 文件级切换
- hunk 级阅读
- 上下文保留
- 中间未修改区折叠
- 文件内阅读位置记忆

### 4.2 用户不要看到什么

页面里不应出现：

- `accept`
- `reject`
- `revert`
- `stage`
- `unstage`
- `reviewer`
- `apply`

唯一允许的“动作”应该是纯阅读动作，例如：

- 切换文件
- 展开/收起未修改区

phase-1 不要求为了“功能完整”额外堆出下载、解释、跳转等辅助控件；只要基础阅读动作成立即可。

## 5. 入口与消息形态

这条能力的主入口应跟随 final turn summary。

推荐形态：

- 在 turn 结束后的 diff summary 区域中提供一个 `查看` 链接
- 视觉位置可落在 summary 第一行右侧
- 点击后进入 turn diff viewer

这个链接不是“打开当前工作区 diff”，而是“查看本轮 diff 快照”。

这层 frozen 语义由入口位置、上下文和页面类型本身承担，不要求在正常阅读态再额外堆解释文案。

## 6. 页面形态

### 6.1 外层风格

虽然底层 artifact / renderer 已经不同于现有通用 preview，但外层风格应与当前 preview 页面保持基本一致。

需要保持一致的部分包括：

- 页面留白、边距、基础配色和排版节奏
- notice / expired / unavailable 等系统态样式

需要明确不同的部分是：

- 正文区域不是现有通用 file preview renderer
- 正文区域是一个专门的 turn diff viewer
- 不复用当前 artifact lineage diff-first 那条正文语义

也就是说，建议复用“preview shell 风格”，但不复用“preview 正文 renderer 语义”。

同时，这张页面不应堆解释性文案来“教用户怎么看”。

默认只保留最小必要信息：

- 文件选择
- 文件名
- `+/-` 统计
- 正文 diff 阅读区

除非进入异常态，否则不应额外出现：

- 教学式 banner
- “首次进入会怎样”的提示文案
- 图例说明
- “这是什么页面”的长说明
- 为了补充设计意图而加入的说明文字

系统说明文案应只保留给真正需要解释的状态，例如：

- 过期
- 数据缺失
- 无法渲染

### 6.2 竖屏

竖屏模式下：

- 顶部收敛成一条轻量 bar
- bar 内只保留 preview 图标和文件选择下拉
- 下拉内容是本次 diff 涉及的文件列表
- 页面正文一次只展示当前选中的一个文件
- 文件切换后，正文切到对应文件的 diff 阅读区

### 6.3 横屏

横屏模式下：

- 文件选择区转为左侧列表
- 右侧是当前文件的 diff 阅读区
- 正常阅读态不保留顶部 frame / 标题区
- 左右布局优先保证桌面横屏阅读效率

### 6.4 文件切换与阅读位置记忆

页面内需要维护一个非持久化的阅读状态：

- 为每个文件记住当前页面会话中的上次滚动位置
- 在几个文件之间来回切换时，恢复到该文件上次阅读位置
- 该状态只保存在当前页面内存中，不需要持久化到服务端

首次进入文件时：

- 若该文件之前没有本页会话内的滚动记录，则默认滚到第一个 hunk

### 6.5 Hunk 展示

每个文件应按 hunk 组织显示。

默认行为：

- 每个 hunk 周围展示有限的未修改上下文
- phase-1 默认上下文行数建议取 `8` 行
- 这个数字不作为 blocker，后续可以微调
- 若首个 hunk 离文件开头很近，应直接从文件开头开始显示
- 若首个 hunk 前仍有较长未修改内容，应在顶部显示可展开的折叠块，而不是直接从中间行号起跳

### 6.6 未修改区折叠

文件中的长段未修改内容不应默认全展开。

推荐行为：

- hunk 之间的长段未修改区默认折叠
- 折叠区显示成一个可点击的 gap bar
- 用户可一键展开该段未修改内容
- 展开后允许再次收起
- 展开后的 gap 应与上下代码块无缝衔接，不再保留额外分隔条
- 当下方 hunk 已与展开 gap 连成连续阅读流时，其 hunk 标题条应隐藏
- 展开/收起应尽量保持当前阅读位置，不应把页面重新跳回顶部

这条交互应尽量靠近 GitHub 的阅读习惯，但不需要复刻 reviewer 语义。

## 7. 数据语义与承载方式

### 7.1 冻结生成时机

这份 snapshot 的生成时机应明确固定为：

- `turn 刚结束时`

更具体地说：

- authoritative `TurnDiffSnapshot` 已经收齐
- turn 已进入最终终态
- final turn diff summary 即将投影或刚开始投影

此时应一次性把 viewer 所需的数据冻结下来，并产出 immutable snapshot artifact。

不应把生成时机延后到：

- 用户第一次点击 `查看` 时
- 后续某次懒加载打开页面时
- 或页面服务端首次访问时

因为只要延后生成，就会引入“工作区后来又变了”的风险，破坏 frozen 语义。

### 7.2 不采用 live 读磁盘

不推荐在打开页面时再去读当前工作区硬盘文件内容。

原因：

- turn 结束后，工作区可能已被后续 turn 继续修改
- 用户可能手动改过文件、切过分支、rebase 过
- 如果页面在打开时再读 live 文件，看到的上下文就会和那一轮的 diff snapshot 脱节

这会直接破坏这张页面最重要的产品语义：

- authoritative
- frozen
- read-only

因此，页面绝不能依赖“打开时读当前磁盘”来拼上下文。

### 7.3 不采用预烘焙静态 HTML

也不推荐在 turn 结束时一次性生成最终 HTML 页面并永久保存。

原因：

- 页面本身包含交互：文件切换、gap 展开/收起、阅读位置记忆
- 后续如果样式、折叠交互、移动端布局要调整，预烘焙页面会把旧页面永久锁死
- 这种能力本质上更适合“冻结数据 + 专用 renderer”，而不是“冻结最终 HTML”

因此推荐冻结的是“数据快照”，不是最终展示 HTML。

### 7.4 推荐方案：immutable snapshot payload + dedicated renderer

推荐承载方式：

- turn 刚结束时生成一个 immutable snapshot artifact
- artifact 内不仅保存 raw unified diff
- 还保存渲染这张页面所需的冻结文件快照和结构化元数据
- 页面打开时由专用 renderer 根据该 snapshot 生成最终 HTML

这样可以同时满足：

- 数据语义冻结
- 页面表现可迭代
- 外链服务端只需要服务一个 immutable artifact
- 不再依赖 live workspace

## 8. Snapshot 模型建议

推荐的 artifact 不是“只有一段 raw diff”的极简文本，而是面向 viewer 的冻结数据包。

示意结构：

```json
{
  "threadId": "thread-1",
  "turnId": "turn-1",
  "generatedAt": "2026-04-24T12:00:00Z",
  "rawUnifiedDiff": "diff --git ...",
  "files": [
    {
      "fileKey": "0",
      "oldPath": "a/internal/x.go",
      "newPath": "b/internal/x.go",
      "changeKind": "modify",
      "rawPatch": "@@ ...",
      "beforeText": "... optional ...",
      "afterText": "... optional ...",
      "binary": false,
      "parseStatus": "ok"
    }
  ]
}
```

其中：

- `rawUnifiedDiff` 仍保留 authoritative 原文，便于下载和调试
- `files[]` 承载文件级阅读所需的冻结数据
- 文本文件优先保留可用的冻结全文快照
- 删除文件时优先保留 `beforeText`
- 新增或修改文件时优先保留 `afterText`
- 若能稳定得到两侧文本，则可同时保留 `beforeText` 和 `afterText`

核心原则是：

- viewer 所需的上下文数据必须在 turn 结束时冻结到 artifact 内
- viewer 打开时不再向 live workspace 取数

## 9. 渲染策略

### 9.1 phase-1 目标

phase-1 的 renderer 需要做到：

- 按 diff 文件顺序列出文件
- 支持切换文件
- 按 hunk 展示 patch
- 展示有限上下文
- 折叠大段未修改内容
- 支持 gap 展开/收起
- 支持页面内阅读位置记忆

### 9.2 数据不足时的降级

若某个文件无法可靠生成完整 viewer 数据，则不要 silent fail。

推荐降级顺序：

1. 文件级 raw patch 视图
2. 整体 raw unified diff 视图

特殊场景包括：

- binary header
- rename/copy header 无法完整冻结文本
- patch parse 失败
- 文件快照缺失

这些情况仍可保留：

- 文件列表项
- 基础元信息
- raw patch 或 raw diff 下载

但不应伪装成完整文件级 viewer。

## 10. 与现有 preview 基座的关系

这条能力与现有 preview 的关系应当是：

- 复用已有 grant / path / TTL / external-access 基座
- 复用 preview shell 的页面风格
- 新增专门的 turn diff artifact 和 renderer

不应理解成：

- 沿用当前 file preview artifact 语义
- 复用当前 previous/current 文本 diff-first renderer
- 或把 turn diff viewer 强塞回普通文件预览路径

更合适的理解是：

- 这是 preview delivery 基座上的一个新只读 viewer 类型

## 11. 前端状态建议

页面内需要维护的最小前端状态包括：

- 当前选中的文件
- 每个文件当前是否已访问
- 每个文件上次滚动位置
- 每个 gap 是否已展开

状态范围：

- 只在当前页面会话内有效
- 不写服务端
- 不要求刷新后恢复

## 12. phase-1 默认值

以下默认值已经足够进入设计实现，不需要继续卡住：

- 默认上下文行数：`8`
- 文件顺序：沿 raw diff 中出现顺序
- 首次打开页面：选中第一个有 hunk 的文件
- 首次打开文件：滚到第一个 hunk
- 再次切回已访问文件：恢复该文件上次滚动位置
- 竖屏：顶部轻量 selector bar
- 横屏：左侧文件列表，顶部 frame 默认隐藏
- gap 展开后与上下代码块无缝拼接

这些默认值后续可以微调，但不改变整体设计方向。

## 13. 完成标准

当 `#307` 进入实现完成态时，至少应满足：

- final turn diff summary 可挂出 `查看` 链接
- 链接打开的是只读 frozen turn diff viewer
- 页面不出现任何 reviewer / accept / reject 语义
- 正常阅读态不出现额外解释性文案
- 页面能按文件切换
- 页面能按 hunk 阅读，并保留有限上下文
- 大段未修改内容默认折叠，可展开/收起
- gap 展开后与上下代码块无缝衔接
- 横竖屏都能完成基础阅读
- 页面内能记住每个文件的阅读位置
- 打开页面时不依赖 live workspace 读盘
- 外层风格与现有 preview 页面保持基本一致

## 14. 后续不在本单讨论的内容

以下内容即使未来存在，也不属于 `#307` 的范围：

- live workspace diff viewer
- hunk accept / reject
- file accept / reject
- frozen viewer 跳转到 live reviewer
- PR / commit / branch 级 reviewer 产品

如果未来要做这些能力，应继续留在 `#144` / `#215` 那条产品线上，而不是回流到 `#307`。
