# 网关与后端连接及消息协议设计

> **状态**：已评审落盘（brainstorming → 设计）。  
> **关联**：[产品与技术面路线图](./2026-04-13-kd48-roadmap.md)（网关与业务 IDL 解耦、多服务扩展）；根目录 [`spec.md`](../../../spec.md) §3、§4。  
> **日期**：2026-04-13  

---

## 1. 目标与非目标

### 1.1 目标

1. **降低编译期耦合**：业务服务调整 **业务 proto / 生成代码** 时，**网关模块原则上无需同步修改与重编译**（不 import 各业务 `v1` 生成包）。
2. **统一玩家入口**：客户端 **只连接 Gateway**；由网关 **转发** 至后端 gRPC 服务。
3. **通用消息协议**：网关与后端之间使用 **稳定、薄** 的契约；载荷对网关为 **不透明**，编码固定为 **JSON UTF-8 字节**（与当前 WebSocket `payload` 语义一致）。
4. **服务发现**：网关通过 **Etcd**（或现有等价机制）解析 `etcd:///kd48/<service-name>` 等 target，与当前 M0 一致方向。
5. **服务间调用**：允许 **业务微服务之间直连 gRPC**（不经网关）；网关仅承接 **来自客户端的流量**。

### 1.2 非目标（本设计不锁死）

- **FileDescriptorSet / gRPC Reflection** 是否采用：作为 **备选** 记录在 §6，**实现阶段** 与「稳定 ingress」方案比选；描述符 **嵌入 vs 挂载** 未定（路线图级 **丙** 策略）。
- **具体 proto 包名、服务名、字段命名**：§3 仅为 **示意**，实现时可微调，但须保持 **向后兼容或显式版本**。
- **流式 RPC / 双向流**：当前以 **一元调用** 为主叙事；若 WS 需对流式扩展，另起增量设计。

---

## 2. 拓扑与发现关系

```
客户端 ──WS/HTTP──► Gateway ──gRPC(稳定 Ingress)──► User / Room / …
                              │
                              │  etcd 解析 target
                              ▼
                        kd48/user-service 等

Room ──gRPC──► User   （示例：服务间直连，不经网关）
```

| 参与者 | 发现对象 | 说明 |
|--------|----------|------|
| 客户端 | Gateway | 不直连业务 gRPC |
| Gateway | Etcd 中的服务名 | 每类后端一条或多条 `grpc.ClientConn`；负载均衡沿用现有策略 |
| 业务服务 | Etcd（及彼此） | 注册与互调保持现有/路线图约定 |

---

## 3. 协议分层

### 3.1 客户端 ↔ 网关（保持现有方向）

- **WebSocket**：统一信封，例如 `method`（字符串）+ `payload`（JSON 对象序列化后的字符串，UTF-8）。
- **迁移建议**：`method` 可逐步规范为 **逻辑 route id**；过渡期允许与 **gRPC full method name** 同形（如 `/user.v1.UserService/Login`），由 **路由表** 映射到后端。

### 3.2 网关 ↔ 后端（新稳定契约）

**原则**：网关 **仅依赖** 一份 **极少变更** 的 proto（建议独立模块，例如 `api/gateway/v1` 或仓库约定路径），**不依赖** `user.v1` 等业务包。

**示意消息**（名称可调整，语义保持稳定）：

```protobuf
// 示意：实际包名、服务名以仓库为准
syntax = "proto3";

package gateway.v1;

message IngressRequest {
  // 供后端内部分发：如 "/user.v1.UserService/Login" 或逻辑 route_key
  string route = 1;
  // UTF-8 JSON 字节；网关不解析业务字段
  bytes json_payload = 2;
  // 可选：trace、染色标签、AB 桶等，与 metadata 二选一或并存，实现阶段定
  map<string, string> baggage = 3;
}

message IngressReply {
  // 业务成功时的 JSON 字节；错误仍优先用 gRPC Status
  bytes json_payload = 1;
}

service GatewayIngress {
  rpc Call(IngressRequest) returns (IngressReply);
}
```

**后端职责**：

- 在进程内将 `json_payload` 用 **protojson**（推荐，与字段命名规则一致）或 `encoding/json` 反序列化到 **当前** 业务 `Message`。
- 调用既有 **业务 handler**；响应再序列化为 JSON bytes 填入 `IngressReply`。
- **业务 proto 变更** 仅影响 **服务内** 代码，**不要求** 网关重编。

**网关职责**：

- 维护 **路由表**：`WS method / route_id` → `grpc target` + `route` 字符串（传入 `IngressRequest.route`）。
- 将 WS `payload` 转为 UTF-8 JSON bytes（若客户端已是 JSON 字符串可直接 `[]byte`）。
- `grpc.Call` 至对应 `ClientConn` 的 `GatewayIngress/Call`（或生成的等价方法）。

### 3.3 错误与状态码

- **对内**：继续遵循 [`spec.md`](../../../spec.md) **§4.3**（gRPC `Status`、业务码扩展）。
- **对外**：网关将 gRPC 错误映射为现有 WS 响应信封（`code` / `msg` / `data`）；**不** 要求后端在 JSON body 内重复一套错误模型（除非产品另有约定）。

---

## 4. 路由与配置

- **路由表** 来源：**代码注册**、**配置文件** 或 **二者结合**；实现阶段选型。
- **最小字段**：`client_method`（或 route_id）、`etcd_scheme`（如 `etcd:///kd48/user-service`）、`ingress_route`（写入 `IngressRequest.route`）。
- **鉴权白名单**（如 Login/Register）：建议从 **硬编码后缀** 演进为 **配置或路由元数据**，避免每加认证接口改网关核心逻辑。

---

## 5. 迁移路径（建议）

1. 在 **api** 中新增 **稳定** `gateway.v1` Ingress proto 并生成 Go（**网关只依赖此包 + grpc + 公共 pkg**）。
2. **User Service** 实现 `GatewayIngress`，在 `Call` 内根据 `route` **分发** 到现有 `Login` / `Register` 实现（可先包装现有 gRPC 方法，再内聚）。
3. **Gateway** 将 `WrapUnary(userClient.Login)` 改为 **通用 Ingress 调用** + 路由表；**删除** 对 `userv1` 客户端的 import。
4. 新增业务服务时：**仅** 扩展路由表 + 该服务实现 `GatewayIngress`（或共享基类），**无需** 修改网关 `go.mod` 业务依赖。

---

## 6. 备选方案（未采纳为默认，保留比选）

| 方案 | 说明 | 与默认关系 |
|------|------|------------|
| **dynamicpb + FileDescriptorSet** | 网关 `grpc.ClientConn.Invoke` + 动态消息；需 **FDS 与网关同步**（嵌入/挂载/流水线） | 你已关注同步成本；作 **备选** |
| **gRPC Server Reflection** | 开发/联调便利；生产是否开启由安全与运维策略决定 | **可选**，不替代默认 ingress |
| **域 BFF** | 网关只调冻结 BFF | 多组件；**远期**可按域引入 |

---

## 7. 风险与待定

- **Ingress 分发器** 在各服务的 **维护成本**：可通过 **小工具从业务 proto 生成注册表** 减轻（实现阶段）。
- **JSON 与 proto 字段** 命名差异（camelCase vs snake_case）：须在 **客户端—网关—服务** 间统一 **protojson 默认** 或文档约定。
- **大包体 / 性能**：JSON 相对 binary 开销；若未来热点路径有瓶颈，可在 **不改变网关稳定契约** 前提下，在 metadata 中协商 **压缩** 或 **内层编码**（另案）。

---

## 8. 对 `spec.md` 的修订建议（合并前审阅）

1. **§4.1**：写明网关 **仅依赖** `gateway.v1`（或最终包名）等 **传输层 proto**，业务 proto 仍在 `api/proto` SSOT，但 **网关模块不 require 业务子包**。
2. **§3.3**：补充 **Ingress + 路由表** 与 WS 信封的对应关系；鉴权白名单 **配置化** 方向。
3. **§5.2**：增加「对外暴露给网关的 Ingress 实现」作为各服务的 **接入规范** 之一。

---

## 9. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-04-13 | 初版落盘：目标 A；拓扑甲（服务可直连）；载荷甲 JSON UTF-8；默认稳定 Ingress + 内部分发；FDS/反射作备选。 |
| 2026-04-13 | 已按实现计划落地：`gateway.v1 GatewayIngress`、User 内分发 Login/Register、网关 `WrapIngress` + `WsHandlerResult`，网关 `main` 不再 import `user/v1` 客户端。 |
