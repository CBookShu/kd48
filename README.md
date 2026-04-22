# kd48

基于 Go 的云原生微服务架构硬核基座，采用显式契约与编译期安全的设计理念。

## 项目概述

kd48 是一个采用 **Monorepo** 模式管理的微服务项目，核心理念是"**拒绝运行时魔法**"——不使用运行时 ORM、不使用运行时 DI 框架，全面拥抱代码生成体系（Protobuf、sqlc），在编译期拦截低级错误，保证系统在高并发下的行为绝对可控、可预测。

项目深度使用ai code，结合体验各种类型的模型，实践 ai 参与长项目的可控性。

### 核心特性

- **网关服务 (Gateway)**：自研轻量级 API Gateway，支持 HTTP/WebSocket 协议转换到内部 gRPC
- **用户服务 (User Service)**：处理用户注册、登录、Session 管理
- **房间服务 (Lobby Service)**：房间管理与匹配
- **统一鉴权**：基于 Redis 的 Session 机制，支持强踢下线
- **分布式追踪**：完整 OpenTelemetry 集成
- **多数据源路由**：支持按路由键选择 MySQL/Redis 连接池

## 技术栈

| 类别 | 技术选型 |
|------|----------|
| 语言 | Go 1.26 |
| 网关框架 | Fiber + gorilla/websocket |
| RPC 框架 | gRPC |
| 数据库 | MySQL 8.0 + sqlc |
| 缓存/会话 | Redis |
| 注册中心 | Etcd |
| 追踪 | OpenTelemetry + Jaeger/Tempo |
| 日志 | log/slog + Zap + lumberjack |
| 迁移工具 | golang-migrate |

## 项目结构

```
kd48/
├── api/proto/          # 集中式 Proto 模块 (IDL 契约)
│   ├── user/           # 用户服务 Proto 定义
│   ├── lobby/          # 房间服务 Proto 定义
│   ├── gateway/        # 网关相关 Proto 定义
│   └── dsroute/        # 数据源路由 Proto 定义
├── gateway/            # API 网关服务
│   ├── cmd/            # 入口
│   └── internal/       # 内部实现
├── services/
│   ├── user/           # 用户微服务
│   │   ├── cmd/        # 入口
│   │   ├── migrations/ # 数据库迁移
│   │   └── sqlc.yaml   # sqlc 配置
│   └── lobby/          # 房间微服务
├── pkg/                # 公共工具包
│   ├── conf/           # 配置管理
│   ├── dsroute/        # 数据源路由
│   ├── logzap/         # Zap 日志封装
│   ├── otelkit/        # OpenTelemetry 工具
│   ├── rediskit/       # Redis 工具
│   └── registry/       # 服务注册
├── docs/               # 文档
├── config.yaml         # 示例配置文件
└── docker-compose.yml  # 本地开发中间件
```

## 快速开始

### 1. 启动中间件

```bash
docker compose up -d
```

| 服务 | 端口 |
|------|------|
| Etcd | 2379 |
| MySQL | 3306 |
| Redis | 6379 |

> MySQL 默认库名 `kd48`，root 密码 `root`

### 2. 配置文件

```bash
mkdir -p logs
cp config.yaml gateway/config.yaml
cp config.yaml services/user/config.yaml
```

### 3. 数据库迁移

```bash
cd services/user
migrate -path ./migrations -database "mysql://root:root@tcp(localhost:3306)/kd48?parseTime=true&loc=Local" up
```

### 4. 启动服务

```bash
# 启动 User 服务
go run ./services/user/cmd/user

# 启动 Gateway
go run ./gateway/cmd/gateway
```

### 5. 连接测试

WebSocket 入口：`ws://localhost:8080/ws`

## 开发命令

```bash
# 运行所有测试
go test ./...

# 构建所有模块
go build ./...

# 运行单个测试包
go test ./gateway/internal/ws/... -v

# 构建二进制
go build -o gateway.bin ./gateway/cmd/gateway
go build -o user.bin ./services/user/cmd/user
```

## 架构设计

### 流量链路

```
客户端 HTTP/WS → Gateway → gRPC → 微服务
                    ↓
              Redis Session
                    ↓
              Etcd 服务发现
```

### 协议转换

- **短连接**：HTTP/HTTPS → gRPC 一元调用
- **长连接**：WebSocket → gRPC 双向流

### 鉴权机制

采用 **Redis Session** 而非 JWT：
- 登录成功后签发 32 字节安全随机 Token
- 支持主动强踢下线（删除 Redis Key）
- 网关层统一校验，未认证请求返回 401

## 配置说明

```yaml
# config.yaml 示例
server:
  name: "kd48"
  env: "dev"

gateway:
  port: 8080
  heartbeat:
    server_timeout: 90s    # 无活动超时
    check_interval: 5s     # 扫描间隔

user_service:
  port: 9000

lobby_service:
  port: 9001

redis:
  addr: "localhost:6379"

etcd:
  endpoints:
    - "localhost:2379"

mysql:
  dsn: "root:root@tcp(localhost:3306)/kd48?parseTime=true&loc=Local"

session:
  expire_hours: 168  # 7天
```

## 文档索引

- [技术规格说明书](./spec.md) - 详细架构设计
- [AI 执行规范](./AGENTS.md) - 开发流程与约定
- [路线图与设计](./docs/README.md) - 长期规划

## 开发流程

本项目使用 [superpowers](https://github.com/obra/superpowers) 技能体系：

```
brainstorming → writing-plans → TDD开发 → verification → code-review → 完成
```

## License

MIT
