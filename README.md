# kd48

Monorepo：网关（Fiber + WebSocket）、User 等 gRPC 服务、共享 `pkg` 与 `api/proto`。架构说明见根目录 [spec.md](./spec.md)。

## 本地开发：只起中间件（Docker）

仓库根目录已提供 `docker-compose.yml`，用于拉起 **Etcd、MySQL 8、Redis**：

```bash
docker compose up -d
```

默认端口（与根目录 `config.yaml` 一致）：

| 服务   | 端口 |
|--------|------|
| Etcd   | 2379 |
| MySQL  | 3306 |
| Redis  | 6379 |

Compose 里为 MySQL 创建了库名 `kd48`，root 密码为 `root`（与示例配置中的 DSN 一致）。若你使用 Apple Silicon 以外的机器，而 `compose` 中仍带有 `platform: linux/arm64`，在 amd64 环境下可能需去掉该行或改为适合本机的架构。

## 配置文件

根目录 [config.yaml](./config.yaml) 为示例。`gateway` 与 `services/user` 的 `main` 均从**各自工作目录**读取 `./config.yaml`，因此启动前请将同一份配置拷到对应目录（或在两处维护符号链接 / 拷贝）：

```bash
cp config.yaml gateway/config.yaml
cp config.yaml services/user/config.yaml
```

按需创建日志目录（与配置里 `log.file_path` 一致）：

```bash
mkdir -p logs
```

## 数据库迁移

在 MySQL 已就绪且库 `kd48` 可连之后，在 **user 服务目录**执行（需已安装 [golang-migrate](https://github.com/golang-migrate/migrate)）：

```bash
cd services/user
migrate -path ./migrations -database "mysql://root:root@tcp(localhost:3306)/kd48?parseTime=true&loc=Local" up
```

## 启动顺序

1. `docker compose up -d`
2. 执行上述 migrate（首次或 schema 变更后）
3. 启动 **User 服务**（向 Etcd 注册，默认端口见 `config.yaml` 中 `user_service.port`）
4. 启动 **Gateway**（默认 HTTP `:8080`）

使用本仓库 `go.work` 时，可在各模块目录下 `go run ./cmd/...`。

WebSocket 入口：`ws://localhost:8080/ws`（具体消息格式见网关 WS 处理与 `spec.md`）。
