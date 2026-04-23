# Web Preview Renderer Architecture Redesign

> Type: `draft`
> Updated: `2026-04-23`
> Summary: 提出 web preview 渲染链路的可扩展重构方案：用 renderer registry + planning pipeline 替代单函数分支耦合，并统一 source-like 预览为“默认带行号高亮，loc 仅做定位增强”。

## 1. 背景

当前 web preview 的核心渲染策略集中在：

- `internal/adapter/feishu/preview/web_preview_render.go`
  - `buildWebPreviewPage(...)`

这段逻辑同时承担了：

- renderer kind 分发
- 大文件 diff/summary 策略
- Markdown prose/source 切换
- `loc` 解析后的行定位行为
- 安全提示拼接
- 高亮与非高亮回退

虽然底层已经有多个渲染函数（例如 line-addressed / highlighted variants），但策略层仍然在一个大分支里混合决策，导致新增类型或调整行为时容易产生横向连带影响。

## 2. 当前主要问题

### 2.1 抽象层级不清

“选哪种预览策略”和“如何生成 HTML”在同一层完成，缺少中间规划层（plan）。

### 2.2 行定位行为分叉

目前 `loc` 是否存在会触发两条路径：

- 有 `loc`：line-addressed
- 无 `loc`：non-line-addressed

这使 source-like 文件在同一类型下出现两套 DOM 结构和样式行为，不利于一致性与测试覆盖。

### 2.3 规则散落且重复

`text/html_source/svg_source/markdown` 各分支都写了一遍“是否高亮 + 回退 + 提示文案”，后续很难做一次改动全局一致。

### 2.4 扩展成本高

新增一种 preview 类型（例如 CSV 表格、JSON tree、结构化 log）会被迫在现有大分支里追加条件，很快继续膨胀。

## 3. 设计目标

1. 把“策略决策”和“具体渲染”解耦。
2. source-like 预览统一为默认带行号和语法高亮（可回退纯文本行号视图）。
3. `loc` 统一为可选增强参数：负责跳转/目标行列高亮，不再决定是否进入另一套 renderer 路径。
4. 保留当前需求能力：大文件 diff-first、summary fallback、HTML/SVG 安全语义、图片/PDF inline 预览。
5. 后续新增 renderer 类型时，不需要改动中心大 switch。

## 4. 总体方案

### 4.1 引入两段式渲染架构

第一段：`RenderPlanner`（决策层）

- 输入：artifact + previous artifact + request context（包含 `loc`）
- 输出：`RenderPlan`
  - `Mode`（source / prose / diff / summary / media / message）
  - `Decorations`（line numbers, syntax highlight, target location）
  - `NoticeParts`
  - `FallbackChain`

第二段：`Renderer`（执行层）

- 按 `Mode` + `RendererKind` 执行具体 HTML 生成
- 失败时按 `FallbackChain` 自动降级

### 4.2 Renderer Registry

在 `preview` 包内建立显式注册表：

- `SourceRenderer`（text/html_source/svg_source/markdown-source）
- `MarkdownProseRenderer`
- `DiffRenderer`
- `SummaryRenderer`
- `ImageRenderer`
- `PDFRenderer`
- `MessageRenderer`

核心调度从“按类型写大 switch”改成“plan 决定 renderer key -> registry 分发”。

### 4.3 统一 Source-Like 展示契约

`source-like` 包括：`text`, `html_source`, `svg_source`, `markdown` 的源码视图。

统一契约：

- 默认输出 `source-block--numbered` 结构（始终有行号锚点）
- 默认尝试语法高亮；失败回退纯文本逐行渲染
- 无 `loc` 时不做目标行/列标记
- 有 `loc` 时追加 target 行/列高亮和 notice

即：`loc` 只影响“定位增强”，不影响“是否带行号”。

### 4.4 Markdown 的双视图策略（保留扩展点）

为兼容“可读 prose”与“源码定位”两种诉求，规划层显式支持：

- `ViewPreference=source`（默认）
- `ViewPreference=prose`（后续可由 query 参数或入口策略控制）

本次推荐默认使用 `source`，保证行为一致性和定位稳定性；`prose` 作为可选模式保留在架构里，而不是写死分支。

### 4.5 安全提示与提示文案标准化

把当前 scattered notice 拼接改成 `NoticeComposer`：

- 通用定位提示
- 类型安全提示（HTML/SVG source-only）
- 大文件策略提示（diff-first / summary-only）

统一按固定顺序拼接，避免不同分支文案不一致。

## 5. 关键接口草案

```go
type WebPreviewRenderContext struct {
	Location PreviewLocation
}

type WebPreviewRenderPlan struct {
	RendererKey  string
	Mode         string
	Decorations  WebPreviewDecorations
	NoticeParts  []string
	FallbackKeys []string
}

type WebPreviewRenderer interface {
	Key() string
	Render(WebPreviewRenderInput) (WebPreviewRenderOutput, error)
}
```

注：接口名仅为草案，目标是固定“plan 与 render 分离”的结构，而不是绑定具体命名。

## 6. 迁移策略

### 阶段 A：结构重构，不改对外行为

1. 抽出 planner + registry + notice composer。
2. 保持现有默认行为（包括 markdown prose 分支）。
3. 用回归测试锁定“重构前后行为一致”。

### 阶段 B：统一 source-like 行号契约

1. `text/html_source/svg_source` 默认切到 numbered source renderer。
2. `loc` 仅作为 target 高亮增强。
3. 增补无 `loc` 场景的行号锚点测试。

### 阶段 C：Markdown 默认视图切换（可独立开关）

1. 默认改为 source 视图。
2. 若需要保留 prose，提供明确模式入口，而不是由 `loc` 隐式切换。
3. 同步更新文档与交互说明。

## 7. 测试与验收

至少覆盖：

1. source-like 文件在有/无 `loc` 时都带行号锚点。
2. 有 `loc` 时 target line/column 高亮生效；无 `loc` 时无 target 标记。
3. 高亮失败回退纯文本 numbered source，不丢行号。
4. HTML/SVG 始终 source-only 且保留安全提示。
5. 大文件 diff-first / summary fallback 在新架构下语义不变。
6. registry 中每个 renderer key 都可被独立单测调用。

## 8. 风险与取舍

### 8.1 视觉变化风险

统一带行号会改变部分用户习惯（尤其当前 markdown prose 默认阅读场景）。

应对：

- 分阶段切换
- 在 issue 中明确“行为变化清单”
- 为 markdown 预留 `prose` 可选模式

### 8.2 重构范围风险

渲染链路改造涉及多个文件与测试，若一次性落地全部策略变更，回归成本高。

应对：

- 先做结构拆分，再做行为切换
- 每阶段独立可回滚

## 9. 与当前需求的对应关系

本方案直接回应当前需求：

- 预览不再区分“带行号路径”和“不带行号路径”
- 统一为带行号 + 高亮的 source-like 展示
- 指定行列仅增加跳转和目标高亮能力

同时把这次调整沉淀为可扩展架构，避免后续继续在单点分支里“补丁式叠加”。
