# Gateway Ingress（网关与后端通用协议）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 [网关与后端连接设计](../specs/2026-04-13-gateway-backend-connection-design.md)：新增稳定 `GatewayIngress` gRPC、User 服务内部分发、网关通过 Ingress + 路由表转发 WS 请求，**移除网关对 `user/v1` 生成客户端的依赖**（网关 `go.mod` 仍依赖 `api/proto`，但仅使用 `gateway/v1`）。

**Architecture:** 客户端 WS 信封不变；网关将 `(method, payload)` 映射为 `IngressRequest{ route, json_payload }`，经 **同一 `grpc.ClientConn`** 调用 `GatewayIngress/Call`；User 服务在 `Call` 内按 `route` 做 **protojson** 解包并转调现有 `Login`/`Register`。为把 **Ingress 返回的 JSON bytes** 填入现有 WS `Data` 字段，将 `WsHandlerFunc` 扩展为返回 **`WsHandlerResult`（含 `proto.Message` 或原始 JSON bytes）**。

**Tech Stack:** Go 1.26、`protoc` + `protoc-gen-go` + `protoc-gen-go-grpc`（与现有 `gen_proto.sh` 一致）、gRPC、`google.golang.org/protobuf/encoding/protojson`、现有 Etcd resolver。

---

## 文件结构（创建 / 修改）

| 路径 | 职责 |
|------|------|
| `api/proto/gateway/v1/gateway.proto` | 稳定 Ingress 契约 |
| `api/proto/gateway/v1/*.pb.go`、`*_grpc.pb.go` | `protoc` 生成（勿手改） |
| `gen_proto.sh` | 增加 `gateway/v1/gateway.proto` |
| `services/user/cmd/user/ingress.go` | `GatewayIngress` 实现与 `route` 分发 |
| `services/user/cmd/user/main.go` | `RegisterGatewayIngressServer` |
| `services/user/cmd/user/ingress_test.go` | Ingress 分发与 mock 单测 |
| `gateway/internal/ws/router.go` | 定义 `WsHandlerResult`、`WsHandlerFunc` 新签名 |
| `gateway/internal/ws/wrapper.go` | `WrapUnary` 适配 `WsHandlerResult`；新增 `WrapIngress` |
| `gateway/internal/ws/handler.go` | 成功分支根据 `WsHandlerResult` 选择 protojson 或 `json.Unmarshal`；鉴权白名单 **本计划保持现状**（后缀 `/Login`、`/Register`） |
| `gateway/cmd/gateway/main.go` | 路由表：`method` → `ingressClient` + `route`；删除 `userv1` import |

---

### Task 1: 新增 `gateway.v1` Proto 并生成 Go 代码

**Files:**
- Create: `api/proto/gateway/v1/gateway.proto`
- Modify: `gen_proto.sh`
- Create（生成）: `api/proto/gateway/v1/gateway.pb.go`、`api/proto/gateway/v1/gateway_grpc.pb.go`

- [ ] **Step 1: 写入 `gateway.proto`**

```protobuf
syntax = "proto3";

package gateway.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/gateway/v1;gatewayv1";

message IngressRequest {
  string route = 1;
  bytes json_payload = 2;
  map<string, string> baggage = 3;
}

message IngressReply {
  bytes json_payload = 1;
}

service GatewayIngress {
  rpc Call(IngressRequest) returns (IngressReply);
}
```

- [ ] **Step 2: 修改 `gen_proto.sh`**

在 `protoc` 调用中 **追加** 一个 proto 文件参数（与 `user/v1/user.proto` 并列）：

```bash
protoc -I api/proto \
	--go_out=api/proto --go_opt=module=github.com/CBookShu/kd48/api/proto \
	--go-grpc_out=api/proto --go-grpc_opt=module=github.com/CBookShu/kd48/api/proto \
	user/v1/user.proto \
	gateway/v1/gateway.proto
```

- [ ] **Step 3: 执行生成**

Run:

```bash
cd /Users/cbookshu/dev/temp/kd48 && bash gen_proto.sh
```

Expected: 无错误；`api/proto/gateway/v1/` 下出现 `gateway.pb.go` 与 `gateway_grpc.pb.go`。

- [ ] **Step 4: 编译 api/proto 模块**

Run:

```bash
cd /Users/cbookshu/dev/temp/kd48/api/proto && go build ./...
```

Expected: `exit code 0`。

- [ ] **Step 5: Commit**

```bash
git add api/proto/gateway/v1 gen_proto.sh
git commit -m "feat(proto): add gateway.v1 GatewayIngress for generic JSON relay"
```

---

### Task 2: User 服务实现 `GatewayIngress` 与单测

**Files:**
- Create: `services/user/cmd/user/ingress.go`
- Create: `services/user/cmd/user/ingress_test.go`
- Modify: `services/user/cmd/user/main.go`

- [ ] **Step 1: 编写失败单测（未知 route）**

在 `ingress_test.go` 中：

```go
package main

import (
	"context"
	"net"
	"testing"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestIngress_Call_UnknownRoute(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	// 使用仅含零值依赖的 ingressServer，Call 内只校验 route
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(nil)) // nil inner：本测仅命中 unknown route，不调用 Login
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	cli := gatewayv1.NewGatewayIngressClient(conn)
	_, err = cli.Call(ctx, &gatewayv1.IngressRequest{
		Route:       "/unknown/Method",
		JsonPayload: []byte(`{}`),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}
```

**注意：** 此步需 `newIngressServer` 在 Step 3 才存在；先运行测试应 **编译失败**。若你严格 TDD，可先写 **仅编译通过的桩** `func newIngressServer(...) *ingressServer { return &ingressServer{} }` 与 `Call` 返回 `InvalidArgument`。

- [ ] **Step 2: 运行测试确认失败（若尚无实现）**

Run:

```bash
cd /Users/cbookshu/dev/temp/kd48/services/user/cmd/user && go test -run TestIngress_Call_UnknownRoute -v .
```

Expected: 编译失败 或（若有桩）FAIL / 非预期码 —— 与当前实现一致即可。

- [ ] **Step 3: 实现 `ingress.go`**

要点：

- `type ingressServer struct { gatewayv1.UnimplementedGatewayIngressServer; inner *userService }`（或显式字段持有 `*userService`）。
- `Register` 在 `main` 里：`gatewayv1.RegisterGatewayIngressServer(grpcServer, newIngressServer(userSvc))`。
- `Call` 逻辑：
  - `route == "/user.v1.UserService/Login"`：`protojson.Unmarshal` → `userv1.LoginRequest`，调用 `s.inner.Login`，`protojson.Marshal` `LoginReply` → `IngressReply.JsonPayload`。
  - `route == "/user.v1.UserService/Register"`：同上。
  - 其他：`return nil, status.Error(codes.InvalidArgument, "unknown ingress route")`。
- `protojson.UnmarshalOptions` / `MarshalOptions`：与网关侧一致使用 **默认字段名**（proto JSON 名）；若客户端 JSON 使用 camelCase，需与 `user.proto` 字段 json 名一致（当前 `username`/`password` 已为 proto 风格）。

```go
// ingress.go 核心片段示意（完整实现由执行者补全包声明与 import）
func (s *ingressServer) Call(ctx context.Context, req *gatewayv1.IngressRequest) (*gatewayv1.IngressReply, error) {
	switch req.GetRoute() {
	case "/user.v1.UserService/Login":
		var in userv1.LoginRequest
		if err := protojson.Unmarshal(req.GetJsonPayload(), &in); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid json: %v", err)
		}
		out, err := s.inner.Login(ctx, &in)
		if err != nil {
			return nil, err
		}
		b, err := protojson.Marshal(out)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
		}
		return &gatewayv1.IngressReply{JsonPayload: b}, nil
	// Register 分支对称
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown ingress route")
	}
}
```

- [ ] **Step 4: 在 `main.go` 注册 Ingress**

在 `grpc.NewServer` 与 `RegisterUserServiceServer` 旁增加：

```go
gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(userSvc))
```

其中 `userSvc` 为 `NewUserService(...)` 的返回值（需与 `RegisterUserServiceServer` 使用 **同一实例**，以便 Ingress 与直连 User API 行为一致）。

- [ ] **Step 5: 运行单测**

Run:

```bash
cd /Users/cbookshu/dev/temp/kd48/services/user/cmd/user && go test -run TestIngress -v .
```

Expected: PASS。

- [ ] **Step 6: 补充 Login 成功路径单测（mock `userService`）**

在 `ingress_test.go` 增加 mock 结构体，实现与 `Login` 相同签名，返回固定 `LoginReply`；`newIngressServer` 接受接口类型（建议定义 `loginRegisterAPI` interface，`*userService` 与 mock 均实现），避免循环类型问题。

Run:

```bash
go test -run TestIngress_Call_Login -v .
```

Expected: PASS；`JsonPayload` 反序列化后 `success == true`。

- [ ] **Step 7: Commit**

```bash
git add services/user/cmd/user/ingress.go services/user/cmd/user/ingress_test.go services/user/cmd/user/main.go
git commit -m "feat(user): implement GatewayIngress Call dispatch for Login/Register"
```

---

### Task 3: 网关 `WsHandlerResult` + `WrapIngress` + Handler 兼容

**Files:**
- Modify: `gateway/internal/ws/router.go`
- Modify: `gateway/internal/ws/wrapper.go`
- Modify: `gateway/internal/ws/handler.go`

- [ ] **Step 1: 在 `router.go` 定义结果类型并改签名**

```go
// WsHandlerResult 成功响应：二选一（互斥）
type WsHandlerResult struct {
	Message proto.Message // 非 nil 时走 protojson → Data
	JSON    []byte        // Message==nil 且 JSON!=nil 时直接 json.Unmarshal 到 Data
}

// WsHandlerFunc 网关业务处理器
type WsHandlerFunc func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error)
```

- [ ] **Step 2: 修改 `wrapper.go` 中 `WrapUnary`**

将返回改为：

```go
return func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error) {
	// ... 原 Unmarshal 与 fn 调用 ...
	if err != nil {
		return nil, err
	}
	return &WsHandlerResult{Message: resp}, nil
}
```

- [ ] **Step 3: 新增 `WrapIngress`**

```go
func WrapIngress(
	cli gatewayv1.GatewayIngressClient,
	route string,
) WsHandlerFunc {
	return func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error) {
		reply, err := cli.Call(ctx, &gatewayv1.IngressRequest{
			Route:       route,
			JsonPayload: payload,
		})
		if err != nil {
			return nil, err
		}
		return &WsHandlerResult{JSON: reply.GetJsonPayload()}, nil
	}
}
```

（`import` 增加 `gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"`。）

- [ ] **Step 4: 修改 `handler.go` 成功响应分支**

在 `resp != nil` 处替换为：

```go
var data interface{}
if resp != nil {
	if resp.Message != nil {
		marshaler := protojson.MarshalOptions{EmitUnpopulated: true}
		jsonBytes, err := marshaler.Marshal(resp.Message)
		if err != nil {
			h.sendResp(ctx, conn, req.Method, int32(codes.Internal), err.Error(), nil)
			continue
		}
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			h.sendResp(ctx, conn, req.Method, int32(codes.Internal), err.Error(), nil)
			continue
		}
	} else if len(resp.JSON) > 0 {
		if err := json.Unmarshal(resp.JSON, &data); err != nil {
			h.sendResp(ctx, conn, req.Method, int32(codes.Internal), err.Error(), nil)
			continue
		}
	}
}
```

并将 `handler(ctx, ...)` 的返回值类型改为 `*WsHandlerResult`（变量名仍可用 `resp`）。

- [ ] **Step 5: 编译网关**

Run:

```bash
cd /Users/cbookshu/dev/temp/kd48/gateway && go build -o /dev/null ./...
```

Expected: 在 `main.go` 仍用旧 `Register` 前应 **编译失败**；进入 Task 4 后恢复通过。

- [ ] **Step 6: Commit**

```bash
git add gateway/internal/ws/router.go gateway/internal/ws/wrapper.go gateway/internal/ws/handler.go
git commit -m "refactor(gateway): WsHandlerResult for proto or raw JSON from Ingress"
```

---

### Task 4: 网关 `main` 接入 Ingress 并移除 `userv1` 客户端

**Files:**
- Modify: `gateway/cmd/gateway/main.go`

- [ ] **Step 1: 替换 import**

删除：`userv1 "github.com/CBookShu/kd48/api/proto/user/v1"`  
增加：`gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"`

- [ ] **Step 2: 创建 Ingress 客户端并注册路由**

在 `conn` 建立后：

```go
ingressCli := gatewayv1.NewGatewayIngressClient(conn)
wsRouter := ws.NewWsRouter()
wsRouter.Register("/user.v1.UserService/Login", ws.WrapIngress(ingressCli, "/user.v1.UserService/Login"))
wsRouter.Register("/user.v1.UserService/Register", ws.WrapIngress(ingressCli, "/user.v1.UserService/Register"))
```

删除：`userClient := userv1.NewUserServiceClient(conn)` 及原 `WrapUnary` 两行。

- [ ] **Step 3: 编译与快速运行验证**

Run:

```bash
cd /Users/cbookshu/dev/temp/kd48/gateway && go build -o /tmp/kd48-gateway ./cmd/gateway
```

Expected: 成功。

手动（或已有脚本）：启动 etcd + mysql + redis + user + gateway，WS 发送：

```json
{"method":"/user.v1.UserService/Login","payload":"{\"username\":\"test\",\"password\":\"test\"}"}
```

Expected: 与改造前行为一致（业务错误码、成功时 `data` 为 JSON 对象）。

- [ ] **Step 4: Commit**

```bash
git add gateway/cmd/gateway/main.go
git commit -m "feat(gateway): route WS to user via GatewayIngress; drop user v1 client import"
```

---

### Task 5: 文档与 Spec 对齐（可选但建议）

**Files:**
- Modify: `docs/superpowers/specs/2026-04-13-gateway-backend-connection-design.md`（§9 变更记录）
- Modify: `spec.md`（按需合并设计 §8 建议）

- [ ] **Step 1: 在设计文档变更记录增加一行「已实现 M0 Ingress + User」**

- [ ] **Step 2: 若合并根 spec**：在 §4.1 / §3.3 增加 1～2 句指向 `gateway.v1` 与路由表（具体内容与设计 §8 一致）。

- [ ] **Step 3: Commit**

```bash
git add docs spec.md
git commit -m "docs: note GatewayIngress implementation status"
```

---

## Self-review（计划自检）

| 设计 / Spec 要求 | 对应 Task |
|------------------|-----------|
| 稳定 Ingress proto + JSON UTF-8 载荷 | Task 1、2、3 |
| 网关不依赖业务生成 Client | Task 4（移除 `userv1.NewUserServiceClient`） |
| Etcd 发现不变 | Task 4 仍使用原 `etcd:///kd48/user-service` |
| 错误走 gRPC Status | Task 2 `Login` 返回的 `error` 原样返回；Task 3 handler 已有 `status.FromError` |
| 服务间可直连 User gRPC | **未改** `RegisterUserServiceServer`，保留 |
| 鉴权白名单配置化 | **本计划未实现**（留后续）；仍用 handler 内后缀 |

**占位符扫描：** 无 TBD/TODO 步骤；未知 route 测试与 mock Login 为明确代码。

**类型一致：** `route` 字符串与 `main` 注册、`ingress.go` `switch`、`WrapIngress` 第二参数 **必须完全一致**（建议三处使用 `const` 或共享 `gateway/internal/ws` 包内常量 —— 可选小重构，YAGNI 可先复制字面量）。

---

## 执行交接

**计划已保存至** `docs/superpowers/plans/2026-04-13-gateway-ingress-implementation-plan.md`。

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每 Task 派生子代理，Task 间人工复核。  
2. **Inline Execution** — 本会话按 Task 顺序执行，每 Task 结束停顿检查。

**请选择：** 回复 `1` 或 `2`（或在 Cursor 中让执行代理按 checkbox 逐项勾选）。
