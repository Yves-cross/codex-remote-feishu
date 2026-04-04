# Developer Guide

## 项目定位

这个仓库当前维护的是公开可发布的 Go 实现，模块路径为：

```text
github.com/kxn/codex-remote-feishu
```

不要再把旧的 `fschannel`、旧的 Node/Rust 目录结构、或本机绝对路径带回主分支。

## 主要二进制

- `relayd`
  - 常驻服务
  - 负责 relay websocket、orchestrator、Feishu gateway、状态 API
- `relay-wrapper`
  - Codex 可执行包装器
  - 负责 native app-server 协议翻译和 relay websocket 上报
- `relay-install`
  - 写配置并接管编辑器入口

## 目录结构

```text
cmd/
  relayd/
  relay-wrapper/
  relay-install/

internal/
  adapter/
  app/
  config/
  core/
  logging/

testkit/
  harness/
  mockcodex/
  mockfeishu/

docs/
```

## 关键文档

- [架构说明](./docs/architecture.md)
- [协议说明](./docs/relay-protocol-spec.md)
- [飞书产品行为](./docs/feishu-product-design.md)
- [安装与部署](./docs/install-deploy-design.md)
- [测试策略](./docs/go-test-strategy.md)

如果改了下面这些内容，文档也要同步：

- wrapper 和 relayd 之间的 canonical protocol
- Feishu 交互行为
- 安装流程、配置路径、运行命令

## 常用命令

格式化：

```bash
gofmt -w $(find cmd internal testkit -name '*.go' | sort)
```

测试：

```bash
go test ./...
```

构建：

```bash
go build ./cmd/relayd
go build ./cmd/relay-wrapper
go build ./cmd/relay-install
```

本地运行：

```bash
./install.sh bootstrap
./install.sh start
./install.sh status
./install.sh logs
./install.sh stop
```

默认生成的可执行文件名：

- `bin/codex-remote-relayd`
- `bin/codex-remote-wrapper`
- `bin/codex-remote-install`

## 代理环境注意事项

很多联调问题不是代码本身，而是全局代理污染了本地回环链路。

本地测试、状态检查、`curl 127.0.0.1`、本地 websocket 调试前，先清理代理环境：

```bash
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY ALL_PROXY all_proxy
```

唯一例外：

- `relay-wrapper` 自己连本地 `relayd` 时不应走代理
- `relay-wrapper` 拉起真实 `codex.real` 时，应恢复捕获到的代理环境

原因是：

- 本地 relay 流量经常会被代理干扰
- 上游 `codex.real -> ChatGPT/OpenAI` 在有代理时往往更稳定

## 开发约束

- 对状态机或协议问题，不要只看单层代码，先看完整链路
- helper/internal traffic 只能靠协议相关的 request/response 标识做关联，不能靠时间或“看起来像同一 thread”
- wrapper 负责准确翻译和标注，不负责产品层可见性策略
- queue、attach、thread 选择、Feishu 展示都应由 orchestrator 决策
- mock 必须和真实协议一致，不能用静态脚本假装通过
- 公开文档里不要写本机绝对路径、个人目录、临时验证痕迹

## 发布前自检

至少执行：

1. `gofmt -w $(find cmd internal testkit -name '*.go' | sort)`
2. `go test ./...`
3. `bash scripts/check/no-local-paths.sh && bash scripts/check/no-legacy-names.sh`

第 3 步的目的是防止旧项目名、本机路径和私有环境痕迹重新进入仓库。

## GitHub Actions

仓库当前包含两个工作流：

- `CI`
  - 在 `master` / `main` 的 push 和 pull request 上运行
  - 检查公开文档是否泄漏本机路径
  - 检查旧项目名和旧前缀是否回流
  - 检查 `gofmt`
  - 运行 `go build` 和 `go test ./...`
- `Release`
  - 通过 GitHub Actions 的 `workflow_dispatch` 手动触发
  - 自动决定下一个版本号
  - 生成 release notes
  - 构建多平台产物并创建 GitHub Release

本地可预演的对应命令：

- `make check`
- `make release-artifacts VERSION=v0.1.0`
