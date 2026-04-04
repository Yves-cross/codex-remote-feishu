# Codex Remote Feishu

`codex-remote-feishu` 用来把 VS Code 里的 Codex 会话接到飞书。

它的工作方式是：

- 用 `relay-wrapper` 包装真实 `codex` 可执行
- 把 Codex app-server 事件转成统一事件流发给 `relayd`
- 由 `relayd` 负责实例管理、thread 路由、消息队列、飞书投影和状态接口

当前目标场景是：

- Linux 主机
- VS Code Remote 连接到这台 Linux 主机
- 在飞书里接管并继续使用 Codex 会话

## 功能

- 列出当前在线的 Codex 实例，并在飞书里 attach
- 查看和切换当前实例的 thread
- 从飞书继续向当前 thread 发消息，或在未绑定时自动新建 thread
- 支持图片暂存，和下一条文本一起提交给 Codex
- 支持 `/stop` 中断当前 turn
- 支持查看和临时覆盖模型、推理强度
- 区分系统提示、过程消息和最终回复

## 组件

- `relayd`
  - 常驻服务
  - 管理实例、surface、queue、Feishu 网关和状态 API
- `relay-wrapper`
  - 替代 VS Code 实际调用的 `codex`
  - 代理 VS Code 和真实 `codex.real`
- `relay-install`
  - 写配置
  - 集成 VS Code / VS Code Remote

## 快速开始

### 1. 准备

- 安装 Go
- 在远端 Linux 主机上安装并可运行 `codex`
- 用 VS Code Remote 连接到这台机器，并安装 OpenAI/Codex 扩展
- 准备飞书机器人的 `App ID` 和 `App Secret`

### 2. 引导安装

```bash
FEISHU_APP_ID=cli_xxx \
FEISHU_APP_SECRET=xxx \
./install.sh bootstrap
```

`bootstrap` 会：

- 构建 `codex-remote-relayd`、`codex-remote-wrapper`、`codex-remote-install`
- 自动探测 VS Code Remote 扩展的 `codex` 入口
- 能 patch bundle 时优先使用 `managed_shim`
- 否则退回修改 `chatgpt.cliExecutable`

### 3. 启动服务

```bash
./install.sh start
./install.sh status
```

默认会生成这些文件：

- `~/.config/codex-remote/wrapper.env`
- `~/.config/codex-remote/services.env`
- `~/.local/share/codex-remote/install-state.json`
- `~/.local/share/codex-remote/logs/codex-remote-relayd.log`

### 4. 重新打开 VS Code Remote

完成 `bootstrap` 后，重新打开远端窗口或重启相关扩展，让 Codex 改为通过 `relay-wrapper` 启动。

### 5. 在飞书里接管

先确保远端 VS Code 里已经打开 Codex，并存在可见实例，然后在飞书里使用：

- `/list`：列出在线实例
- 回复数字：attach 到对应实例
- `/threads` 或 `/use`：列出当前实例可见 thread
- 回复数字：切换输入目标 thread

## 飞书命令

- `/list`：列出在线实例
- `/status`：查看当前接管状态、输入目标、模型配置
- `/threads` 或 `/use`：列出当前实例的 thread
- `/follow`：切回“跟随当前 VS Code”
- `/detach`：断开当前实例接管
- `/stop`：中断当前正在执行的 turn，并清空尚未发出的飞书队列
- `/model`：查看或设置飞书侧临时模型覆盖
- `/reasoning`：查看或设置飞书侧临时推理强度覆盖

机器人菜单当前支持：

- `list`
- `status`
- `stop`
- `threads`

## 飞书侧需要开通什么

至少需要让机器人能接收：

- 文本消息
- 图片消息
- reaction 创建事件
- 机器人菜单事件

如果要在单聊里收消息，飞书应用还需要开通对应的 P2P 消息权限。

## 状态与排障

查看服务状态：

```bash
./install.sh status
./install.sh logs
curl --noproxy '*' -sf http://127.0.0.1:9501/v1/status
```

常见排障顺序：

1. 确认 `relayd` 已启动
2. 检查 `services.env` 里的飞书凭证和端口
3. 检查 `wrapper.env` 里的 `RELAY_SERVER_URL` 和 `CODEX_REAL_BINARY`
4. 确认 VS Code Remote 扩展入口已经被 shim 或 settings 接管
5. 打开 Codex 界面并确认 wrapper instance 已连上
6. 查看 `codex-remote-relayd.log`

## 文档

- [架构说明](./docs/architecture.md)
- [协议说明](./docs/relay-protocol-spec.md)
- [飞书产品行为](./docs/feishu-product-design.md)
- [安装与部署](./docs/install-deploy-design.md)
- [测试策略](./docs/go-test-strategy.md)
- [app-server 重构背景](./docs/app-server-redesign.md)

## 持续集成与发版

- Push / PR 会自动跑 GitHub Actions CI
- 在 GitHub Actions 里手动触发 `Release` workflow 可以发版
- `Release` 会自动计算下一个语义化版本号，运行测试，构建多平台产物，并创建 GitHub Release

## 开发

开发者说明见：

- [DEVELOPER.md](./DEVELOPER.md)
