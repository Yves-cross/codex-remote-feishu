# Feishu System Markdown Boundary Design

> Type: `draft`
> Updated: `2026-04-18`
> Summary: 基于 `#269` 的新证据，重新界定剩余 Feishu system-card 的 markdown 边界，并给出后续 picker 与 notice 两条执行线的拆分建议。

## 背景

`#269` 的 `P0/P1` 已经收掉了最危险的“真实动态内容直接嵌入 markdown”路径：

- `thread history`
- `request prompt`
- `selection / switch`
- `snapshot`

原先剩余的 `P2 helper / 系统提示收口` 本来被当成一个较轻的后置单元，但最新代码核对后发现，这个判断已经失效。

剩余的 system-card 并不是单纯“系统固定文案 + 局部 inline helper”：

- `target/path picker` 的一部分状态文本，已经在 orchestrator 上游把结构和动态值预先拼成 markdown 字符串
- Git 导入消息不仅有固定 copy，还会混入 repo URL、目标目录、stderr tail、失败原因
- `send file` 的 terminal status 也会把文件名和大小直接拼进 markdown 字符串
- `debug_error` 一类 notice 会混入 runtime/debug 动态值，而 `projectNoticeBody(...)` 当前只负责局部 inline-tag 改写

所以，继续沿用旧 `P2` 的直觉去 sink 侧补 helper，只会让 string-markdown 合同继续扩大。

## 新证据

### 1. picker 状态文本已经是“上游预拼 markdown”

代表代码：

- `internal/core/orchestrator/service_target_picker_git_import.go`
- `internal/core/orchestrator/service_send_file.go`

典型问题：

- `targetPickerGitImportSuccessText(...)`
- `targetPickerGitImportStatusText(...)`
- `targetPickerGitImportCloneFailureText(...)`
- `sendFileStartedStatusText(...)`

这些函数不是把“结构”和“动态值”分开传给 adapter，而是直接在 orchestrator 里拼出整段 markdown。

动态值包括：

- repo URL
- workspace path
- stderr 输出
- 失败原因
- 文件名
- 文件大小

这意味着 adapter 看到的已经只是一个裸 `string`，再也无法区分：

- 哪部分是系统拥有的结构
- 哪部分是动态值
- 哪部分只适合 `plain_text`

### 2. picker message/hint 链路不再只是“固定系统 copy”

代表代码：

- `internal/core/orchestrator/service_target_picker_add_workspace.go`
- `internal/core/orchestrator/service_target_picker.go`
- `internal/adapter/feishu/projector_target_picker.go`

`FeishuTargetPickerMessage.Text` 当前既可能是固定说明，也可能带：

- 目标目录
- Git 可用性提示
- 导入失败原因

但 `targetPickerMessageMarkdown(...)` 仍把它当成一整段 markdown 文本，仅通过 `renderSystemInlineTags(...)` 处理反引号包裹片段。

### 3. notice 不再只是“简单系统提示”

代表代码：

- `internal/core/orchestrator/problem_notice.go`
- `internal/adapter/feishu/projector.go`

`control.Notice` 这条线里同时混有两类东西：

1. 纯系统 copy notice
2. 携带 runtime/debug 动态值的 notice

`debug_error` 明显属于第二类，但现在它仍然落在 `Notice.Text string` 这条总线里。  
`projectNoticeBody(...)` 最多只能把反引号包起来的 token 变成 `<text_tag>`，并不能恢复更清晰的结构边界。

### 4. `renderSystemInlineTags(...)` 不是通用 sanitizer

`renderSystemInlineTags(...)` 当前真实能力只有：

- 保留 fenced code
- 把单行内的反引号片段改写成 `<text_tag>`

它做不到：

- 判断整段 markdown 是否仍然安全
- 判断一段文字里哪些部分其实应该是 `plain_text`
- 把“已经上游预拼好的结构 + 动态值”重新拆开

所以它的合理定位只能是：

- adapter-owned markdown 的局部格式 helper

而不是：

- 剩余 system-card 的通用收口器

## 为什么旧的 P2 不再是稳定执行闭包

旧的 `P2 helper / 系统提示收口` 混在了一起的，其实是三个不同问题：

1. `picker/path picker` 的上游预拼 markdown 合同
2. `notice` 的动态 debug/runtime 合同
3. `renderSystemInlineTags(...)` 的 helper 命名和职责边界

它们虽然都叫“system-card”，但执行时有两个关键差异：

- 风险来源不同
  - picker 更偏“结构化卡片里混入动态值”
  - notice 更偏“纯 copy 与动态 runtime/debug 内容共用一个 string 总线”
- 验证面不同
  - picker 需要看 target/path picker/card projector 全链路
  - notice 需要看 notice 生产逻辑和 projector 投影契约

这已经足够说明：旧 `P2` 不该继续作为一个直接编码单元推进。

## 约束

### 1. 不引入 DTO 风格的大搬运

当前新方向不鼓励为了“看起来更整齐”引入一层泛化 DTO。

因此后续边界设计应优先：

- 直接增强现有的 `control.*View` / `control.Notice`
- 使用少量、目的明确、贴近业务语义的字段
- 避免为了这一个问题引入跨全仓的通用内容 AST

### 2. 继续保留 adapter 作为 Feishu-specific 渲染 owner

`<text_tag>` / `<font>` / 飞书 markdown 细节仍应由 adapter 决定。  
不应该把 Feishu-specific 标签格式反向扩散到 orchestrator。

### 3. 不提前扩大到全仓类型系统

当前要解决的是剩余 system-card 的最小边界，不是重新设计整个内容系统。  
能在现有控制层视图上补小而明确的边界，就不要先做“全局内容类型化”。

### 4. `command catalog` 暂不回并

它目前仍然更像系统拥有的 markdown 文案合同问题。  
在开始混入外部动态文本之前，不应为了“统一”而重新并回本轮执行面。

## 方案比较

### 方案 A：继续在 sink 侧补 helper

做法：

- 继续保留上游预拼 markdown
- 在 projector 侧新增更多 neutralize / inline-tag / danger wrapper 逻辑

优点：

- 改动表面小
- 不需要扩 control 结构

缺点：

- adapter 看不到结构和动态值的边界
- 只能按症状继续补字符串规则
- 很容易把剩余系统卡的 string 合同继续放大

结论：

- 不推荐

### 方案 B：只对高风险 system-card 引入最小结构边界

做法：

- 固定系统 copy 仍允许保留 string markdown
- 但只要一条链路会把动态值混进结构化卡片，就停止上游预拼 markdown
- 在现有 `control.*View` / `control.Notice` 上补少量目的明确的字段，让 adapter 重新拥有结构

优点：

- 能直接收掉本轮剩余高风险面
- 不要求全仓统一升级
- 可以按 picker / notice 分线落地

缺点：

- 需要对现有 view / notice 结构做增量调整
- 会引入一部分兼容期双路径

结论：

- 推荐

### 方案 C：现在就上更强的全局内容类型系统

做法：

- 直接建立更显式的 `PlainText` / `Markdown` / `UnsafeExternalText` 等类型边界
- 把更多 projector / orchestrator 链路一起纳入

优点：

- 理论一致性最好

缺点：

- 当前范围会迅速做大
- 容易把正在收口的 tracker 重新拉回“大重构”
- 与“避免 DTO 化”的当前偏好不一致

结论：

- 本轮不推荐

## 推荐边界

### 1. 保留 string-markdown 的条件

只有同时满足以下条件，才继续允许某条 system-card 链路保留 raw markdown string：

- 结构完全由本地固定 copy 拥有
- 没有混入 runtime/debug/path/repo/output/file name 这类动态值
- 即使包含反引号格式，`renderSystemInlineTags(...)` 也只是局部增强，而不是安全兜底

不满足这些条件时，不应继续把 string-markdown 当边界。

### 2. picker / path picker 的推荐落点

对这类卡片，推荐把“结构”收回到 adapter：

- orchestrator 不再预拼完整 `StatusText` / `Message.Text` markdown
- orchestrator 改为传递更贴近业务的结构信息
- adapter 再决定哪些部分用 markdown，哪些部分用 `plain_text` 或 neutral tag

为了避免 DTO 化，建议直接增强现有 view：

- `FeishuTargetPickerView`
- `FeishuPathPickerView`

优先加少量、强语义字段，例如：

- 状态标题
- 若干 label/value section
- 最近输出列表
- footer / next-step 说明

而不是引入一个抽象的“通用内容节点树”。

### 3. notice 的推荐落点

把 notice 分成两类：

1. 纯系统 copy notice
   - 继续允许保留简单 string body
2. 动态 runtime/debug notice
   - 不再只靠 `Notice.Text string`
   - 由生产侧显式传出结构化 debug 信息，或在 `control.Notice` 上补专用字段

这样 `projectNoticeBody(...)` 就不再需要同时承担：

- 固定 copy 投影
- 动态 debug 信息弱解析

### 4. `renderSystemInlineTags(...)` 的推荐定位

后续应把它降格并显式命名为：

- 仅服务 adapter-owned markdown 的局部 inline-code helper

它可以继续存在，但不应再被当成“剩余 system-card 的通用安全层”。

## 推荐的后续执行拆分

### 执行单元 A：picker / path picker status boundary

目标：

- 收掉 target picker、path picker、git import、send file 里剩余的预拼 markdown 合同

包含：

- `internal/core/orchestrator/service_target_picker*.go`
- `internal/core/orchestrator/service_send_file.go`
- `internal/adapter/feishu/projector_target_picker.go`
- `internal/adapter/feishu/projector_path_picker.go`

验收重点：

- repo/path/output/file name 不再作为裸字符串混进 markdown 结构
- adapter 对结构重新拥有明确控制权

### 执行单元 B：notice boundary

目标：

- 把纯系统 copy notice 与动态 debug/runtime notice 分流

包含：

- `internal/core/orchestrator/problem_notice.go`
- `internal/adapter/feishu/projector.go`
- 其余会生成动态 notice body 的位置

验收重点：

- `debug_error` 不再共享同一条裸 `Notice.Text` string 合同
- 纯 copy notice 不因为这轮改造被过度复杂化

### 执行单元 C：helper contract cleanup

目标：

- 把 `renderSystemInlineTags(...)` 的职责名实对齐
- 补一组明确声明其边界的测试

这个单元应在 A/B 至少完成一条之后再做，这样 helper 的定位就不会继续漂。

## 推荐顺序

1. 先做执行单元 A
2. 再做执行单元 B
3. 最后做执行单元 C

原因：

- picker/status 这条线当前最明显地存在“上游预拼 markdown + 动态值混用”
- notice 虽然也有风险，但先把 picker/status 的结构边界立住，后面的 notice 方案会更清晰
- helper cleanup 必须最后做，否则容易再次变成“先写一个通用 helper 再说”

## 结论

`#269` 后续不应再直接进入旧的 `P2 helper / 系统提示收口`。

更稳的推进方式是：

1. 先承认旧 `P2` 已失去稳定闭包
2. 把“最小 system-markdown 边界设计”提升为单独入口
3. 再把后续执行拆成：
   - picker / path picker status boundary
   - notice boundary
   - helper contract cleanup

这样既能继续控制范围，也不会为了收掉剩余 system-card 风险而提前走向一轮过大的类型重构。
