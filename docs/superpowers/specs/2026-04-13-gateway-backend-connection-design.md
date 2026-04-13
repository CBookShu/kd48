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
| Gateway | **逻辑服务类型**（见 §8）+ 实例发现前缀 | 按类型维护 `ClientConn`；无状态类型下 resolver 自动 Watch **实例** |
| 业务服务 | Etcd（及彼此） | **实例** 自注册；**类型定义** 由运维写入（可与路由同前缀或分子树） |

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

- **路由表** 来源：演进目标为 **Etcd 托管 + Watch 热更新**（见 **§11**）；过渡期可为 **代码注册** 或 **本地 YAML**，与 Etcd 中 **结构对齐** 以便迁移。
- **与类型的关系**：路由项引用 **服务类型** `service_type`（`ServiceTypeSpec.type_key`）；由类型解析 **gRPC 发现 target** 及 **routing_mode**（见 §8～§9）。
- **鉴权**：`GatewayRouteSpec.public == true` 的条目 **免鉴权**；否则须已认证——用于替代 `handler` 内 **后缀 `/Login`、`/Register`** 硬编码（实现 Watch 后落地）。

### 4.1 路由 SSOT：`GatewayRouteSpec`（Proto + Etcd JSON）

- **Proto**：`api/proto/gateway/v1/gateway_route.proto` 中的 **`GatewayRouteSpec`**。
- **Etcd 键**：`kd48/meta/gateway-routes/{route_id}`；**值**：`GatewayRouteSpec` 的 **protojson** UTF-8 JSON。
- **网关 v0.1 加载（建议）**：`schema_version == 1`；`route_id` 与 key 一致；`ws_method` 唯一（冲突则拒收或后写覆盖策略由实现定）；`service_type` 须能关联到已加载的 **`ServiceTypeSpec`** 且为 **STATELESS_LB**。

**Etcd 值示例（protojson）**：

```json
{
  "schemaVersion": 1,
  "routeId": "user-login",
  "wsMethod": "/user.v1.UserService/Login",
  "serviceType": "user",
  "ingressRoute": "/user.v1.UserService/Login",
  "public": true,
  "displayName": "Login"
}
```

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
- **热更新复杂度**：Watch 触发频繁时需 **防抖/合并**；`ClientConn` 替换需 **优雅下线**（见 §11）。

---

## 8. 逻辑服务类型与运行实例

### 8.1 区分两类概念

| 概念 | 含义 | 变更频率 | 维护方 |
|------|------|----------|--------|
| **服务类型（逻辑）** | 对外可引用的「一类后端」：发现前缀、路由模式、协议约定等 **元数据** | 低；**增删改类型属运维/发布动作** | 运维（暂定 **Etcd** 为真理源之一） |
| **服务实例（进程）** | 具体 `host:port`（或等价 endpoint），随扩缩容上下线 | 高 | **进程启动时向 Etcd 注册**（与现有 `registry.RegisterService` 一致方向） |

### 8.2 与网关的关系

- 网关启动或 Watch 更新时：先认识 **有哪些服务类型** 及各自 **发现前缀**，再为每个需要的类型建立（或更新）**`grpc.ClientConn`**。
- **实例** 的增减由 **gRPC Resolver** 对前缀的 Watch 消化；网关 **不** 在配置里枚举每台机器。

### 8.3 类型定义的 SSOT：Proto + Etcd 存 JSON（已定）

- **结构约束** 以 **`api/proto/gateway/v1/service_type.proto`** 中的 `ServiceTypeSpec` 及子 message 为 **单一描述源**（与业务 proto 分离，仍属 `gateway.v1`）。
- **Etcd 值** 仍为 **UTF-8 JSON 文本**：写入/读取时使用 **`protojson`**（`Marshal` / `Unmarshal`），与手工 `etcdctl put`、运维脚本、网关代码 **同一套字段语义**。
- **JSON 字段名**：遵循 proto 的 **JSON 映射**（默认 camelCase 等），运维侧以生成文档或 `buf curl` / 示例为准。
- **网关 v0.1 加载策略（已定）**：仅接受 `routing_mode == SERVICE_ROUTING_MODE_STATELESS_LB` 且 `schema_version == 1`；`discovery.grpc_etcd_target` 必填；**有状态枚举值预留于 proto 注释**，实现未就绪前 **拒收或跳过**（与讨论选项 **A** 一致）。

**Etcd 值示例（protojson，字段名以生成结果为准）**：

```json
{
  "schemaVersion": 1,
  "typeKey": "user",
  "displayName": "User Service",
  "routingMode": "SERVICE_ROUTING_MODE_STATELESS_LB",
  "discovery": {
    "grpcEtcdTarget": "etcd:///kd48/user-service",
    "loadBalancing": "round_robin"
  },
  "ingress": {
    "useGatewayIngress": true
  }
}
```

---

## 9. `routing_mode`：无状态与有状态抵达

### 9.1 无状态（`stateless_lb`，默认与「前缀发现」匹配）

- **语义**：同类实例 **可互换**；`grpc.Dial("etcd:///kd48/<prefix>", …, round_robin)` 合理。
- **连接形态**：一个 `ClientConn` 下，resolver 解析出 **多地址**，形成 **多条 subchannel（多条 TCP）**；应用侧仍按 **一个逻辑连接** 使用，由 LB 选路。符合「无状态服务各节点单独连、但对网关抽象为一类」的说法。

### 9.2 有状态（不可仅依赖「统一前缀 + 轮询」）

- **语义**：请求必须抵达 **特定实例**（或 **由分片键决定的固定集合**），不能假设任意节点等价。
- **抵达方式（实现阶段择一或组合）**，示例：
  - **显式 target**：先由大厅/匹配等算出 `host:port` 或实例 ID，网关 **对该 target 建连或复用连接池**；
  - **分片发现前缀**：Etcd 上按 shard/room 等区分子前缀，resolver + **一致性哈希** 等；
  - **有状态入口 + 内部转发**：网关只连少量路由器，由其后转到正确房间进程（多一跳）。

### 9.3 类型元数据与 Proto 枚举

- **`routing_mode`**：以 **`ServiceRoutingMode`** 枚举为准（见 `service_type.proto`）；JSON 中形如 `"routingMode": "SERVICE_ROUTING_MODE_STATELESS_LB"`（protojson 默认枚举名为字符串）。
- **仅 `STATELESS_LB`**（v0.1 唯一启用）：**Etcd 实例注册前缀** 由 `discovery.grpc_etcd_target` 给出（如 `etcd:///kd48/user-service`）。
- **有状态**（预留枚举值）：须另扩 `ServiceTypeSpec` 或子 message，并定义 **抵达规则**；在网关实现落地前 **不按生产加载**（选项 **A**）。

---

## 10. Etcd 键空间草案（示意，非最终实现）

以下仅为 **命名空间约定**，便于网关与运维工具对齐；实际前缀可与现有 `kd48/` 统一。

| 用途 | 示意前缀 / Key 模式 | 值内容（示意） |
|------|---------------------|----------------|
| **服务类型定义** | `kd48/meta/service-types/{type_key}` | **JSON**：`ServiceTypeSpec` 的 **protojson** 形式（见 `gateway/v1/service_type.proto`） |
| **WS 路由** | `kd48/meta/gateway-routes/{route_id}` | **JSON**：`GatewayRouteSpec` 的 protojson（见 `gateway/v1/gateway_route.proto`） |
| **实例注册**（现有） | `kd48/user-service/…`（与 `etcd:///kd48/user-service` 解析规则一致） | 地址与租约 |

**说明**：**类型与路由** 与 **实例注册** 分 subtree，避免与 resolver 扫描实例的逻辑混淆。

---

## 11. 网关 Watch 与热更新（尽量不依赖重启）

### 11.1 原则

- **正常运维**（改服务类型、改路由、实例上下线）应 **尽量不重启网关进程**；**二进制升级** 另当别论。
- **实例上下线**：无状态场景下多数已由 **gRPC Resolver Watch** 覆盖，无需网关单独写 Watch（除非做运维观测）。

### 11.2 建议 Watch 的数据

| 数据 | 行为 |
|------|------|
| **服务类型**（`…/service-types/`） | 合并变更 → **新建/关闭** 对应 `ClientConn`；关闭时 **停止接新 RPC、等待在途、Close** |
| **WS 路由**（`…/gateway-routes/`） | **原子替换** 内存路由表；`WsRouter` 须 **并发安全**（`RWMutex` 或 `atomic.Value` 挂整张 map） |

### 11.3 实现注意

- **抖动**：路由或类型频繁变更时 **防抖/批处理**，避免连接风暴。
- **失败策略**：Watch 断连时 **重连 + 全量拉取**（fallback），避免静默空表。

### 11.4 冷启动：Bootstrap 顺序（规范）

以下顺序 **须遵守**，保证「先有连接池、再有路由」，避免路由引用不存在的 `service_type`。

1. **建立 Etcd 客户端**（超时、TLS、认证等与现有 `pkg/registry` 或等价配置对齐）。
2. **全量加载服务类型**：对前缀 `kd48/meta/service-types/` 做 **Range（带 revision）**，解析每条值为 **`ServiceTypeSpec`**（protojson）。
3. **校验类型（v0.1）**：对每条执行 §8.3 规则；**失败单条**：记录错误并 **跳过该 key**（或 **整体失败** 策略二选一，须在实现中固定一种并文档化；**推荐开发环境跳过、生产环境可选严格失败**）。
4. **构建连接池**：对每个 **有效** `type_key`：`grpc.Dial(spec.discovery.grpc_etcd_target, …)`，得到 **`ClientConn`**，存入 **`pool[type_key]`**；并为每类创建 **`GatewayIngressClient`**（或等价接口）。
5. **全量加载 WS 路由**：对前缀 `kd48/meta/gateway-routes/` Range（带 revision）。
6. **校验路由**：`schema_version`；`route_id` 与 key 后缀一致；`ws_method` **非空**；`ingress_route` **非空**；**`service_type` 必须 ∈ `pool` 且对应类型已校验为 `STATELESS_LB`**；否则跳过或严格失败（与步骤 3 策略一致）。
7. **构建运行时路由视图**：内存结构至少包含：`ws_method` → `{ ingress_client 或 conn + WrapIngress 所需参数, ingress_route, public }`。
8. **注入 `WsRouter` 与 Handler**：注册 **`WsHandlerFunc`**；若 Handler 仍用后缀白名单，**过渡期保留**；最终实现应改为 **按 `ws_method` 查表得 `public`**（见 **§11.9**）。
9. **启动 Watch**：在持有 **bootstrap revision** 之后，分别对 **服务类型前缀**、**路由前缀** 建立 **Watch**（可合并为单个 watcher + 按 key 分流，实现任选）。

### 11.5 全量与增量（Revision）

- **Bootstrap** 的 Range 必须记录 **返回的 `revision`**（或各前缀各自 revision，若分开 Range）。
- **Watch** 从 **`revision + 1`** 起订阅；事件类型 **PUT/DELETE** 均需处理。
- **断连或 `compact revision` 过期**：**禁止** 假设本地快照仍完整；必须 **退化为全量 Range + 重建内存视图 + 重建/复用 ClientConn**（见 **§11.8**），再重新 Watch。

### 11.6 服务类型变更（热更新）

| 事件 | 行为（规范） |
|------|----------------|
| **新增类型** | 校验 → `Dial` → 加入 `pool`；**不自动** 启用路由，直至路由 Watch 到引用该类型的条目 |
| **修改类型** | 视为 **替换**：对 **新 spec** 校验 → **新建** `ClientConn`（或 target 未变则复用）→ **切换引用** → 旧 conn **进入 draining**（§11.8）后 `Close` |
| **删除类型** | **须先** 无路由引用（实现可拒绝删除或网关侧将受影响路由标为 **503**）；通过后 **draining** 关闭 conn，从 `pool` 移除 |

**同一 `type_key` 并发写**：以 **Etcd 修订号较大者** 或 **后到达事件** 为准（实现须 **幂等**）。

### 11.7 WS 路由变更（热更新）

- **按 key 粒度** 更新内存表：`route_id` 即 Etcd key 后缀；**DELETE** 则移除对应 `ws_method` 映射。
- **`ws_method` 冲突**（两条路由指向同一 `ws_method`）：Bootstrap 与每次增量合并后 **须检测**；**推荐**：拒绝后写者（日志 + 指标）或 **显式定义「按 `route_id` 字典序后者获胜」**——**实现必须选一种并写死**。
- **原子暴露**：对前端路由表采用 **`atomic.Value` 持有不可变 snapshot** 或 **`sync.RWMutex` + 深拷贝替换**，保证 **读路径（每条 WS 消息）无锁或读锁**，避免长时间阻塞。

### 11.8 `ClientConn` 生命周期（draining）

| 状态 | 含义 |
|------|------|
| **active** | 可接新 RPC |
| **draining** | **不接新 RPC**；已在途 RPC **允许完成**（设 **上限等待时间**，超时后 `Close` 并记录） |
| **closed** | 已关闭，不得再使用 |

**类型 target 变更或类型删除** 时必须经 **draining**，避免泄漏连接与 **在途请求** 误打到旧集群。

### 11.9 鉴权与 `GatewayRouteSpec.public` 的衔接（规范）

- **目标**：淘汰 Handler 内 **`strings.HasSuffix(..., "/Login")`** 一类硬编码。
- **读路径**：根据 **`req.Method`（WS 信封）** 在 **当前路由 snapshot** 中查找；若 **无路由**：保持现有 **404/unknown** 行为。
- **`public == true`**：允许未认证连接调用该 `ws_method`。
- **`public == false`**：与现网一致，未认证则 **拒绝**（错误码与是否断连保持现有 §3.3 策略）。
- **路由表中未找到** 且未认证：按 **unknown method** 或 **unauthorized** 的优先级须在实现中 **固定**（推荐：**先鉴权**——未登录且非 public 列表则 unauthorized，与现逻辑兼容需单独对账）。

**过渡期**：允许 **「Etcd 路由未启用」** 时回退 **硬编码 public 方法表**（与配置开关绑定）。

### 11.10 降级与健康

| 场景 | 建议策略 |
|------|----------|
| **启动时 Etcd 不可达** | **默认 fail-fast**（进程退出，由编排重启）；可选 **本地快照 YAML** 仅用于 **灾备**，须在配置中 **显式开启** |
| **运行中 Etcd 断连** | **保留最后一致 snapshot** 继续服务；**健康检查** 置 **unready**；恢复后 **全量 resync** |
| **部分 key 解析失败** | **指标 + 日志**；是否 **整表拒绝** 与 §11.4 步骤 3 策略一致 |

### 11.11 可观测性（规范）

- **计数器**：类型加载失败数、路由加载失败数、Watch 重连次数、draining 超时次数。
- **日志**：每次 **全量 resync**、每次 **连接池替换**（含 `type_key`、old/new target 摘要）。
- **追踪**：可选将 **`route_id` / `type_key`** 写入 OTel attribute（实现阶段）。

---

## 12. 对 `spec.md` 的修订建议（合并前审阅）

1. **§4.1**：写明网关 **仅依赖** `gateway.v1`（或最终包名）等 **传输层 proto**，业务 proto 仍在 `api/proto` SSOT，但 **网关模块不 require 业务子包**。
2. **§3.3**：补充 **Ingress + 路由表** 与 WS 信封的对应关系；鉴权白名单 **配置化** 方向。
3. **§5.2**：增加「对外暴露给网关的 Ingress 实现」作为各服务的 **接入规范** 之一。
4. **§3 / 注册发现**：区分 **逻辑服务类型（运维维护）** 与 **实例自注册**；补充 **有状态** 不得仅依赖无差别前缀轮询。
5. **运维与网关**：网关对 **类型与路由** 的 **Etcd Watch 热更新** 方向（与本文 §8～§11 一致）。
6. **服务类型 Schema**：在 `api/proto/gateway/v1/service_type.proto` 定义 **`ServiceTypeSpec`**；Etcd 存 **protojson** 化 JSON。
7. **WS 路由 Schema**：在 `api/proto/gateway/v1/gateway_route.proto` 定义 **`GatewayRouteSpec`**；含 **`public`** 与 **`service_type`** 引用。
8. **网关动态配置**：Bootstrap 顺序、Watch/revision、**draining**、`public` 与 Handler 衔接、降级与健康（与本文 **§11.4～§11.11** 一致）。

---

## 13. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-04-13 | 初版落盘：目标 A；拓扑甲（服务可直连）；载荷甲 JSON UTF-8；默认稳定 Ingress + 内部分发；FDS/反射作备选。 |
| 2026-04-13 | 已按实现计划落地：`gateway.v1 GatewayIngress`、User 内分发 Login/Register、网关 `WrapIngress` + `WsHandlerResult`，网关 `main` 不再 import `user/v1` 客户端。 |
| 2026-04-13 | 增补：逻辑服务类型 vs 实例；`routing_mode`（无状态 LB / 有状态抵达）；Etcd 键空间草案；网关 Watch 热更新与不依赖重启原则；§4 与 §2 表格对齐。 |
| 2026-04-13 | 服务类型：以 `service_type.proto`（`ServiceTypeSpec`）为 SSOT，Etcd 存 protojson JSON；v0.1 仅启用 `STATELESS_LB`（选项 A）。 |
| 2026-04-13 | WS 路由：`gateway_route.proto` 中 `GatewayRouteSpec`；Etcd protojson；§4.1 与 `public`/类型引用约定。 |
| 2026-04-13 | §11 细化：冷启动 bootstrap、revision 与全量重同步、类型/路由热更、`ClientConn` draining、`public` 与鉴权衔接、降级与健康、可观测性。 |
