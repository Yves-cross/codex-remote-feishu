# Fschannel

Go 版 Codex Relay 重写基线。

旧的 Node.js / Rust 实现源码快照已归档到：

`/data/dl/fschannel-archive-20260403-234311`

当前仓库只保留 Go 重写后的代码、设计文档和测试基座。

## 当前结构

```text
cmd/
  relayd/
  relay-wrapper/
  relay-install/

internal/
  app/
  adapter/
  core/
  config/

testkit/
  harness/
  mockcodex/
  mockfeishu/

docs/
```

## 设计文档

- [docs/go-rewrite-architecture.md](./docs/go-rewrite-architecture.md)
- [docs/go-test-strategy.md](./docs/go-test-strategy.md)
- [docs/relay-protocol-spec.md](./docs/relay-protocol-spec.md)
- [docs/feishu-product-design.md](./docs/feishu-product-design.md)
- [docs/install-deploy-design.md](./docs/install-deploy-design.md)
- [docs/implementation-change-inventory.md](./docs/implementation-change-inventory.md)

## 已实现内容

- canonical protocol 核心类型与 `wrapper <-> relayd` websocket envelope
- surface / thread / queue / staged image 领域状态
- orchestrator 状态机
- assistant text renderer
- Codex native -> canonical translator
- `thread/loaded/list + thread/read` 初始同步
- relayd 运行时
  - wrapper 注册 / 断线下线
  - agent command 路由
  - HTTP `healthz` / `v1/status`
  - Feishu 网关接入与操作投影
- relay-wrapper 运行时
  - 真实 Codex 子进程代理
  - 本地 VS Code 流量观测
  - 远端 command 注入
  - 内部 JSON-RPC response 抑制
- 安装器配置写入与 VS Code settings 集成
- `install.sh` bootstrap/start/stop/status
- 真实状态机式 `mockcodex`
- `mockfeishu`
- e2e harness + relayws / daemon / wrapper 集成测试

## 二进制

- `relayd`
  - 提供 wrapper websocket 接入、产品状态编排、Feishu 转发和状态 API
- `relay-wrapper`
  - 作为 Codex 可执行包装器运行，代理 VS Code <-> Codex，并接入 relayd
- `relay-install`
  - 生成 wrapper/services 配置并修改 VS Code settings

## 开发

运行测试：

```bash
go test ./...
```

格式化：

```bash
gofmt -w $(find cmd internal testkit -name '*.go' | sort)
```

构建：

```bash
go build ./cmd/relayd
go build ./cmd/relay-wrapper
go build ./cmd/relay-install
```

安装与启动：

```bash
./install.sh bootstrap
./install.sh start
./install.sh status
```

## 当前测试覆盖重点

- attach 后默认 pin 当前 focused thread
- queue item 入队时冻结 thread/cwd
- 本地 `turn/start` / `turn/steer` 触发 `paused_for_local`
- handoff 恢复远端队列
- staged image 跟随首条文本绑定
- stop 清理飞书侧 queue / staged image
- renderer 的代码块 / 文件列表分块
- Codex translator 的 initiator 判定
- installer 配置和 editor settings 写入
