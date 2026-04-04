# 安装与部署设计

## 1. 文档定位

这份文档描述的是**当前实现中的安装、配置和本地部署方式**。

当前目标很明确：

- 让 `relay-wrapper` 有稳定配置来源
- 让 `relayd` 有稳定服务配置
- 支持 VS Code Remote 下的真实 Codex 接管
- 提供一组足够简单的本地命令来构建、启动和排障

## 2. 当前配置布局

下面这些路径描述的是**默认安装布局**。

如果通过 `relay-install -base-dir` 或 `install.sh` 的 `BASE_DIR` 改了基目录，则会落到 `<base-dir>/.config` / `<base-dir>/.local/...` 下。

### 2.1 Wrapper 配置

路径：

```text
~/.config/codex-remote/wrapper.env
```

当前写入字段：

- `RELAY_SERVER_URL`
- `CODEX_REAL_BINARY`
- `CODEX_REMOTE_WRAPPER_NAME_MODE`
- `CODEX_REMOTE_WRAPPER_INTEGRATION_MODE`

### 2.2 Services 配置

路径：

```text
~/.config/codex-remote/services.env
```

当前写入字段：

- `RELAY_PORT`
- `RELAY_API_PORT`
- `FEISHU_APP_ID`
- `FEISHU_APP_SECRET`
- `FEISHU_USE_SYSTEM_PROXY`

需要注意：

- `FEISHU_USE_SYSTEM_PROXY` 已经被 `relayd` 读取
- 但当前 `relay-install` CLI 和 `install.sh bootstrap` 还没有单独暴露对应参数
- 也就是说，如需打开它，当前做法仍然是手动修改 `services.env`

### 2.3 安装状态

路径：

```text
~/.local/share/codex-remote/install-state.json
```

当前记录：

- wrapper config path
- services config path
- state path
- integration mode
- vscode settings path
- bundle entrypoint

### 2.4 运行时文件

- `relayd` pid: `~/.local/state/codex-remote/codex-remote-relayd.pid`
- `relayd` log: `~/.local/share/codex-remote/logs/codex-remote-relayd.log`

## 3. 编辑器集成模式

当前只支持两种集成模式。

### 3.1 `editor_settings`

通过 [settings.go](../internal/adapter/editor/settings.go) 修改：

- `chatgpt.cliExecutable`

目标是让编辑器直接调用 `relay-wrapper`。

### 3.2 `managed_shim`

通过 [shim.go](../internal/adapter/editor/shim.go) 改写 VS Code Remote 扩展 bundle 内的 `codex` 入口脚本。

脚本会：

- 保留原始 `codex.real`
- 设置 `CODEX_REAL_BINARY`
- 最终 `exec relay-wrapper`

这是当前在 VS Code Remote 场景下更可靠的模式。

## 4. `relay-install`

`relay-install` 是安装器的 Go 入口，负责：

- 写配置文件
- 保留已有 Feishu 凭证
- 按模式 patch 编辑器入口
- 写 `install-state.json`

当前命令行参数见 [main.go](../cmd/relay-install/main.go)。

## 5. `install.sh`

`install.sh` 是当前仓库里的本地运维脚本，支持：

- `bootstrap`
- `start`
- `stop`
- `status`
- `logs`
- `build`

### 5.1 `bootstrap`

行为：

1. 构建三个二进制
2. 自动探测 VS Code Remote 扩展 bundle 入口
3. 自动决定集成模式
   - 找到 bundle 入口则优先 `managed_shim`
   - 否则退回 `editor_settings`
4. 调用 `relay-install`

### 5.2 `start`

行为：

- 构建二进制
- 后台启动 `relayd`
- 设置 `CODEX_REMOTE_SERVICES_CONFIG`
- 使用 pidfile 跟踪进程

### 5.3 `stop`

行为：

- 按 pidfile 停止 `relayd`

### 5.4 `status`

输出：

- `relayd` 是否运行
- wrapper/services config 路径
- log 路径

### 5.5 `logs`

直接 `tail -f` 当前 log 文件。

## 6. 当前部署模型

当前稳定模型是：

- `relayd` 常驻运行在这台 Linux 机器上
- `relay-wrapper` 不常驻
  - 它由 VS Code / Codex 扩展在需要时拉起
- Feishu gateway 集成在 `relayd` 内

因此联调时要分清两种状态：

1. `relayd` 是否在线
2. 当前是否有 wrapper instance 连接进来

这两件事缺一不可。

## 7. 凭证与保留策略

安装器当前遵循的规则：

- 如果本次 bootstrap 没有显式传新的 `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
- 则保留已有 `services.env` 里的值

也就是说，“未传入”代表“保留旧值”，不是“清空配置”。

## 8. 当前已知边界

- 还没有 `uninstall`
- 还没有 editor integration rollback
- `install.sh` 仍然是仓库内脚本，不是完整的系统服务管理器
- Windows 本机不是当前主目标平台；当前主要围绕 Linux + VS Code Remote 设计

## 9. 实际排障建议

安装/部署问题优先检查：

1. `wrapper.env`
2. `services.env`
3. `install-state.json`
4. `codex-remote-relayd.pid`
5. `codex-remote-relayd.log`
6. VS Code Remote 扩展 bundle 里的 `codex` 入口是否已被 shim 接管
