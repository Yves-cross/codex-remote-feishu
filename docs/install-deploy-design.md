# 安装与部署设计

## 1. 文档定位

这份文档定义的是安装器、配置布局、编辑器接入方式和 server/bot 部署方式。

它解决的不是飞书产品交互，而是下面这些问题：

- wrapper 的配置放哪里，如何和 server/bot 配置解耦
- 安装器如何根据当前平台自动决定“改 VS Code 配置”还是“接管 codex 入口”
- 如何同时支持交互式安装和非交互式部署
- 如何让开发环境和实际部署共用一套命令面

相关文档：

- [relay-protocol-spec.md](./relay-protocol-spec.md)
- [app-server-redesign.md](./app-server-redesign.md)
- [feishu-product-design.md](./feishu-product-design.md)

## 2. 当前问题

基于现有 [setup.sh](/data/dl/fschannel/setup.sh) 和运行方式，当前安装路径有几个结构性问题：

1. `.env` 同时承载了 wrapper 和 server/bot 配置，但 wrapper 往往是由 VS Code 拉起，未必继承同一份 shell 环境。
2. `setup.sh vscode` 只能“直接改 settings.json”，没有把“是否应改 settings，还是应接管 codex 命令入口”建成正式策略。
3. `setup.sh start` 直接在 repo 里起 `node server/dist/index.js` 和 `node bot/dist/index.js`，适合开发，不适合做稳定部署模型。
4. 当前没有非交互配置面，CI、开发自测和远程批量部署都不方便。
5. 当前没有安装状态记录，后续无法可靠回滚 editor integration。

## 3. 设计目标

安装器第一版目标冻结为：

1. 不增加新的长期运行语言栈。前期允许用脚本实现。
2. wrapper 配置独立于 server/bot 配置。
3. 安装器支持交互式和非交互式两种模式。
4. 编辑器接入支持自动判定策略，也允许显式覆盖。
5. 不直接覆盖系统真实 `codex` 二进制。
6. 同时支持：
   - repo 内开发模式
   - 当前用户级安装/部署模式

## 4. 冻结决策

### 4.1 wrapper 必须使用独立配置

后续实现中，wrapper 不再依赖 repo 根目录 `.env` 作为主配置来源。

原因：

- wrapper 是由 VS Code / Cursor 拉起的
- 工作目录可能是任意项目目录
- 继承环境可能不稳定

因此必须给 wrapper 一个稳定的独立配置文件。

### 4.2 “替换 codex 可执行”用受管 shim 实现，不直接覆盖真实二进制

用户诉求里的“替换 codex 可执行”，第一版实现为：

- 受管 shim / launcher
- 放在受控路径里
- 通过 PATH 优先级或 editor 显式路径接管调用

明确不做：

- `mv /usr/bin/codex /usr/bin/codex.real`
- 原地覆盖 npm 安装出来的 `codex`

这样做的原因：

- 回滚简单
- 不破坏用户现有安装
- 不依赖 root 权限

### 4.3 编辑器集成有两种正式策略

```ts
type WrapperIntegrationMode =
  | "editor_settings"
  | "managed_shim";
```

- `editor_settings`
  - 修改 VS Code / Cursor 的 `chatgpt.cliExecutable`
- `managed_shim`
  - 安装一个受管 `codex` shim，接管 editor 最终调用到的命令名

### 4.4 自动模式只在这两种策略之间做选择

```ts
type WrapperIntegrationDecision = {
  requestedMode: "auto" | "editor_settings" | "managed_shim";
  resolvedMode: "editor_settings" | "managed_shim";
  targetEditor: "vscode-remote" | "cursor-remote" | "vscode-local" | "cursor-local" | "none";
};
```

### 4.5 前期仍然允许脚本实现，但命令面要先冻结

第一版可以是 shell 脚本，但不能继续只做“几段 install + configure + start”的松散逻辑。

必须先冻结命令面和配置模型，这样后面即使重写成 Node/Rust CLI，也不会再改产品接口。

## 5. 配置布局

## 5.1 两套配置文件

配置明确分成两类：

### 5.1.1 wrapper 配置

专供 wrapper 读取：

```text
~/.config/codex-relay/wrapper.env
```

字段建议冻结为：

```dotenv
RELAY_SERVER_URL=ws://127.0.0.1:9500
CODEX_REAL_BINARY=/home/dl/.nvm/versions/node/v22.16.0/bin/codex
CODEX_RELAY_WRAPPER_NAME_MODE=workspace_basename
CODEX_RELAY_WRAPPER_INTEGRATION_MODE=editor_settings
```

### 5.1.2 services 配置

专供 server/bot 读取：

```text
~/.config/codex-relay/services.env
```

字段建议冻结为：

```dotenv
RELAY_PORT=9500
RELAY_API_PORT=9501
SESSION_GRACE_PERIOD=300
MESSAGE_BUFFER_SIZE=100

FEISHU_APP_ID=cli_xxx
FEISHU_APP_SECRET=xxx
RELAY_API_URL=http://127.0.0.1:9501
FEISHU_USE_SYSTEM_PROXY=false
```

## 5.2 安装状态文件

安装器必须维护状态文件：

```text
~/.local/share/codex-relay/install-state.json
```

至少记录：

```ts
type InstallState = {
  layoutMode: "repo" | "user";
  wrapperIntegration: {
    mode: "editor_settings" | "managed_shim";
    targetEditor: string | null;
    settingsFile: string | null;
    backupFile: string | null;
    shimPath: string | null;
  };
  paths: {
    wrapperConfig: string;
    servicesConfig: string;
    logDir: string;
    runDir: string;
  };
  deploy: {
    processManager: "repo-pidfile" | "systemd-user";
    serverUnit: string | null;
    botUnit: string | null;
  };
};
```

这一步是为了：

- 支持 status
- 支持 rollback / uninstall
- 避免重复安装时把 editor 改乱

## 5.3 配置优先级

### wrapper

后续 wrapper 配置读取顺序建议冻结为：

1. CLI args
2. `CODEX_RELAY_WRAPPER_CONFIG`
3. `~/.config/codex-relay/wrapper.env`
4. 兼容旧环境变量
   - `RELAY_SERVER_URL`
   - `CODEX_REAL_BINARY`
5. 内建默认值

### server / bot

后续 services 配置读取顺序建议冻结为：

1. 进程环境变量
2. `CODEX_RELAY_SERVICES_CONFIG`
3. `~/.config/codex-relay/services.env`
4. repo `.env` 仅作为开发兼容 fallback

## 6. 安装器命令面

建议新增单独安装器入口，例如：

```text
./install.sh
```

为了保持平滑迁移：

- 现有 `setup.sh` 保留一段时间
- 后续让它逐步代理到新安装器

第一版命令面建议冻结为：

```text
install.sh bootstrap
install.sh configure-wrapper
install.sh configure-services
install.sh integrate-editor
install.sh deploy
install.sh start
install.sh stop
install.sh status
install.sh uninstall
```

## 6.1 `bootstrap`

职责：

1. 检查依赖
2. 构建产物
3. 配置 wrapper
4. 配置 server/bot
5. 集成 editor
6. 部署并启动服务

交互式默认入口：

```bash
./install.sh bootstrap
```

非交互式：

```bash
./install.sh bootstrap \
  --non-interactive \
  --layout repo \
  --integration auto \
  --feishu-app-id "$FEISHU_APP_ID" \
  --feishu-app-secret "$FEISHU_APP_SECRET" \
  --relay-server-url ws://127.0.0.1:9500
```

## 6.2 `configure-wrapper`

职责：

- 发现真实 `codex`
- 生成 wrapper 独立配置
- 决定 editor integration 策略

## 6.3 `configure-services`

职责：

- 收集飞书配置
- 收集 relay 端口和地址
- 生成 services 配置

## 6.4 `integrate-editor`

职责：

- 按 auto / explicit mode 修改 editor settings 或安装 managed shim

## 6.5 `deploy`

职责：

- 根据 layout mode 写 launcher
- 准备日志目录、运行目录
- 安装 process manager 配置

## 7. 运行布局

## 7.1 `repo` 模式

面向开发。

特点：

- 直接使用当前仓库里的 `server/dist`、`bot/dist`、`wrapper/target/release`
- 不复制构建产物
- 日志、pid 可以继续放在 repo 内，或者放到 XDG 目录
- 适合本地开发、自测、CI

建议命令：

```bash
./install.sh bootstrap --layout repo --non-interactive --skip-editor-if-missing
```

## 7.2 `user` 模式

面向实际部署。

特点：

- 配置放 XDG 目录
- 可选复制或链接产物到受控目录
- 后续 status / uninstall 更稳定

建议目录：

```text
~/.config/codex-relay/
~/.local/share/codex-relay/
~/.local/state/codex-relay/
~/.local/bin/
```

## 8. 编辑器接入自动判定

## 8.1 候选目标

安装器先搜索当前机器上可写的 editor settings 目标。

Linux / remote Linux：

- `~/.vscode-server/data/Machine/settings.json`
- `~/.vscode-server-insiders/data/Machine/settings.json`
- `~/.cursor-server/data/Machine/settings.json`
- `~/.cursor-server-insiders/data/Machine/settings.json`
- `~/.config/Code/User/settings.json`
- `~/.config/Code - Insiders/User/settings.json`
- `~/.config/Cursor/User/settings.json`

macOS：

- `~/Library/Application Support/Code/User/settings.json`
- `~/Library/Application Support/Code - Insiders/User/settings.json`
- `~/Library/Application Support/Cursor/User/settings.json`

Windows 本机安装，属于后续增强项。第一版脚本若仍采用 shell，实现上不应把 Windows 本机作为主目标平台。

## 8.2 自动判定规则

`--integration auto` 时，决策顺序冻结为：

1. 若发现可写的 remote settings
   - 选择 `editor_settings`
   - 优先 remote settings
2. 否则若发现可写的 local settings
   - 选择 `editor_settings`
3. 否则
   - 选择 `managed_shim`

这条规则正好覆盖你现在最常见的场景：

- Windows 上开 VS Code
- Remote SSH 到 Linux 机器
- 安装器跑在 Linux 机器
- 此时应优先改 `~/.vscode-server/data/Machine/settings.json`

## 8.3 多个 settings 目标同时存在时

交互式模式：

- 列出所有候选
- 推荐项放第一位
- 用户可明确选择一个

非交互式模式：

- 只选一个目标
- 优先级：
  1. `vscode-remote`
  2. `cursor-remote`
  3. `vscode-local`
  4. `cursor-local`

这样避免一次性把多个 editor 都改了。

## 8.4 settings 模式下写入什么

推荐直接写：

```json
{
  "chatgpt.cliExecutable": "/absolute/path/to/codex-relay-wrapper-launcher"
}
```

这里建议写 launcher，而不是直接写 wrapper binary，原因是 launcher 更容易：

- 固定加载 wrapper 配置文件
- 增加兼容 env
- 后续做诊断和回滚

## 8.5 shim 模式怎么做

shim 模式下，不改 editor settings，而是在受控目录生成：

```text
~/.local/bin/codex
```

其行为是：

- 读取 wrapper 配置
- 启动真正的 wrapper launcher

补充约束：

- 不直接修改 npm 安装出来的真实 `codex`
- 安装器必须校验 `~/.local/bin` 是否在 PATH 前部
- 若 PATH 不满足条件：
  - 交互式给出明确提示
  - 非交互式直接失败，除非显式 `--allow-misordered-path`

## 9. 真实 `codex` 发现逻辑

因为 shim 模式下，wrapper 需要知道真实 codex 路径，所以安装器必须正式化发现策略。

顺序建议冻结为：

1. 显式 `--codex-real-binary`
2. 配置中已有值
3. PATH 中第一个**不属于受管 shim** 的 `codex`
4. 用户常见 Node 安装路径探测
   - `~/.nvm/versions/node/*/bin/codex`
   - `~/.asdf/installs/nodejs/*/bin/codex`

探测到后，至少做一次可执行校验。

## 10. 部署模型

## 10.1 第一版只支持两类 process manager

```ts
type ProcessManagerMode =
  | "repo-pidfile"
  | "systemd-user";
```

### `repo-pidfile`

用于开发和无 systemd 的环境。

特点：

- 直接起 `node server/dist/index.js`
- 直接起 `node bot/dist/index.js`
- 维护 pidfile 和日志

### `systemd-user`

用于长期运行的 Linux 用户环境。

特点：

- 生成 `systemd --user` unit
- 使用独立 env 文件
- 开机后用户会话可自动恢复

第一版建议：

- 默认 `repo-pidfile`
- 只有用户显式选择或检测到稳定 Linux 环境时才推荐 `systemd-user`

## 10.2 launcher 设计

建议最终由安装器生成两个 launcher：

### wrapper launcher

```text
~/.local/share/codex-relay/bin/codex-relay-wrapper-launcher
```

职责：

- 加载 wrapper 配置
- 设置需要的 env
- 调用真实 wrapper binary

### service launcher

```text
~/.local/share/codex-relay/bin/codex-relay-start
~/.local/share/codex-relay/bin/codex-relay-stop
```

职责：

- 加载 services 配置
- 启动 / 停止 server 和 bot

## 11. 交互式安装流程

建议交互顺序冻结为：

1. 选择运行布局
   - `repo`（推荐给开发）
   - `user`
2. 选择 editor integration
   - `auto`
   - `editor_settings`
   - `managed_shim`
3. 确认真实 `codex` 路径
4. 输入 `RELAY_SERVER_URL`
5. 输入飞书 `App ID`
6. 输入飞书 `App Secret`
7. 确认端口
8. 选择 process manager
9. 执行部署

这里建议把“wrapper 配置”和“services 配置”分两屏展示，避免用户误以为是一套配置。

## 12. 非交互式接口

第一版非交互参数建议冻结为：

```text
--non-interactive
--layout repo|user
--integration auto|editor_settings|managed_shim
--editor-target auto|vscode-remote|cursor-remote|vscode-local|cursor-local|none
--codex-real-binary <path>
--relay-server-url <ws-url>
--relay-port <port>
--relay-api-port <port>
--feishu-app-id <id>
--feishu-app-secret <secret>
--feishu-use-system-proxy true|false
--process-manager repo-pidfile|systemd-user
--skip-editor-if-missing
--skip-start
```

约束：

- 非交互模式下缺少必填参数时，直接失败
- 不允许 silently prompt

## 12.1 开发场景示例

```bash
./install.sh bootstrap \
  --non-interactive \
  --layout repo \
  --integration auto \
  --editor-target auto \
  --relay-server-url ws://127.0.0.1:9500 \
  --feishu-app-id "$FEISHU_APP_ID" \
  --feishu-app-secret "$FEISHU_APP_SECRET" \
  --process-manager repo-pidfile
```

## 12.2 仅配置 wrapper，不部署 bot/server

```bash
./install.sh configure-wrapper \
  --non-interactive \
  --integration auto \
  --editor-target auto \
  --codex-real-binary /home/dl/.nvm/versions/node/v22.16.0/bin/codex \
  --relay-server-url ws://127.0.0.1:9500
```

## 13. 安全和回滚

## 13.1 配置权限

下列文件必须限制权限：

- `wrapper.env`
- `services.env`
- `install-state.json`

目标权限：

- 文件 `0600`
- 目录 `0700`

## 13.2 editor settings 回滚

若使用 `editor_settings`：

- 安装器必须在首次修改前记录 backup
- uninstall 时恢复
- 若 settings 在中间被用户手改，优先做“仅移除受管键”而不是整文件覆盖

## 13.3 shim 回滚

若使用 `managed_shim`：

- uninstall 只删除受管 shim
- 不碰真实 codex

## 14. 边界情况

## 14.1 机器上同时存在 VS Code Remote 和本地 Code

处理：

- 交互式让用户选
- 非交互式只按优先级选一个

## 14.2 未安装 `jq`

不能把 `jq` 当必须依赖。

处理：

- 优先用 `jq`
- 其次用 Node 自己改 JSON
- 再次用 Python
- 再次失败则回退成“打印出需要手动添加的配置并返回失败”

## 14.3 bot/server 配置变更后重启

处理：

- `configure-services` 只改配置，不隐式重启
- `deploy --restart` 或 `start` 再接管重启

## 14.4 wrapper 配置变更后 editor 不生效

原因通常是：

- editor 还在缓存旧进程
- remote server 还没重连

处理：

- 安装器 status 里给出提示：
  - 重启 VS Code 窗口
  - Remote SSH 重新连接

## 15. 与现有 `setup.sh` 的关系

当前 [setup.sh](/data/dl/fschannel/setup.sh) 仍可继续作为临时脚本，但它应该逐步退化成兼容入口。

建议迁移顺序：

1. 先按本文把命令面和配置模型定下来。
2. 让新安装器先覆盖 `configure-wrapper` / `configure-services` / `integrate-editor`。
3. 再把 `setup.sh install|configure|start` 迁移成对新安装器的包装。
4. 最后再考虑是否保留 `setup.sh`。

## 16. 本轮实现建议

如果下一步开始编码，建议顺序如下：

1. 先给 wrapper 和 services 定义独立配置文件读取逻辑，但保留当前 `.env` 兼容。
2. 再实现安装器的 `configure-wrapper` 和 `integrate-editor`，把“auto 决策”真正做出来。
3. 再实现 `configure-services` 和 `deploy/start/stop/status`。
4. 最后再处理 `systemd-user` 和 uninstall/rollback。

第一阶段不需要一上来就做完整打包安装，只要把：

- 配置拆分
- editor 接入决策
- 非交互部署

这三件事先做稳，后面演进就比较自然了。
