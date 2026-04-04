# Docker 部署

这里的 Docker 资产只负责部署 `relayd`。

原因很直接：

- `relay-wrapper` 必须和 VS Code / Codex 运行在同一台机器上
- `relayd` 适合作为常驻后台服务单独部署

快速开始：

```bash
cp deploy/docker/.env.example deploy/docker/.env
docker compose -f deploy/docker/compose.yml --env-file deploy/docker/.env up -d --build
```

默认端口：

- `9500`: wrapper -> relayd websocket
- `9501`: 状态 API

容器起来以后，仍然需要在宿主机执行 `setup.sh` 或 `setup.ps1`，把 VS Code 接到本地映射出来的 relay 地址。
