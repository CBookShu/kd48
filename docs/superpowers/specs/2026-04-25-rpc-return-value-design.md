# RPC 返回值设计规范

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:writing-plans to create implementation plan after spec approval.

**Goal:** 业务 RPC 只返回业务数据，错误处理交给框架统一处理

**Architecture:** 业务 RPC 使用纯粹的 gRPC 原生类型，网关层负责将 gRPC status code 转换为标准 ApiResponse 响应

**Tech Stack:** gRPC, gorilla/websocket, proto3

---

## 设计原则

### 1. 业务 RPC 只返回业务数据

**Proto 定义：**
```protobuf
service CheckinService {
  rpc Checkin(CheckinRequest) returns (CheckinData);
  rpc GetStatus(GetStatusRequest) returns (CheckinStatusData);
}
```

- 不使用 `ApiResponse` 包装
- 返回类型直接是业务数据结构
- 服务开发者只关注业务逻辑

### 2. 错误处理由框架统一

**服务端代码：**
```go
// 使用 gRPC 原生错误
if err != nil {
    return nil, status.Errorf(codes.InvalidArgument, "already checked in today")
}

// 成功直接返回业务数据
return &CheckinData{ContinuousDays: 1}, nil
```

**网关转换层 (ingress)：**
- gRPC OK (code=0) → ApiResponse Code=0
- gRPC InvalidArgument (code=3) → ApiResponse Code=1
- gRPC Unauthenticated (code=16) → ApiResponse Code=101
- gRPC Internal (code=13) → ApiResponse Code=2
- ... 其他错误码映射

### 3. 扩展属性

使用 gRPC metadata 传递扩展信息：

**服务端：**
```go
md := metadata.Pairs("request-id", "abc123")
grpc.SendHeader(ctx, md)
```

**网关转换层：**
- 从 gRPC metadata 提取需要暴露给客户端的字段
- 放入 ApiResponse.meta

**客户端：**
- 解析 ApiResponse.meta 获取扩展信息

---

## 统一错误码映射

| gRPC Code | ErrorCode | 说明 |
|-----------|-----------|------|
| OK | 0 | 成功 |
| INVALID_ARGUMENT | 1 | 请求参数错误 |
| INTERNAL | 2 | 内部错误 |
| UNAVAILABLE | 3 | 服务不可用 |
| UNAUTHENTICATED | 101 | 未认证 (用户相关) |
| NOT_FOUND | 100/300 | 资源不存在 |

---

## 需要修改的文件

### Proto 文件
- `api/proto/lobby/v1/checkin.proto` - 改为直接返回 CheckinData / CheckinStatusData

### 网关层
- `gateway/cmd/gateway/ingress.go` - 添加 gRPC status → ApiResponse 转换逻辑
- 或创建独立的 error mapper

### 服务端
- `services/lobby/cmd/lobby/checkin_server.go` - 改用 status.Errorf

### 客户端 (CLI)
- `cmd/cli/internal/commands/handler.go` - 移除对 Success 字段的检查，只检查 Code
- `cmd/cli/internal/client/gateway.go` - 无变化（已经处理 ApiResponse）

---

## 示例：修改后的 Checkin RPC

**Proto:**
```protobuf
service CheckinService {
  rpc Checkin(CheckinRequest) returns (CheckinData);
  rpc GetStatus(GetStatusRequest) returns (CheckinStatusData);
}
```

**服务端:**
```go
func (s *CheckinService) Checkin(ctx context.Context, req *lobbyv1.CheckinRequest) (*lobbyv1.CheckinData, error) {
    if s.period == nil {
        return nil, status.Errorf(codes.Unavailable, "no active period")
    }
    // ... 业务逻辑
    return &lobbyv1.CheckinData{
        ContinuousDays: int32(status.ContinuousDays),
        Rewards:        rewards,
    }, nil
}
```

**网关转换:**
```go
// 将 gRPC 错误转为 ApiResponse
if st, ok := status.FromError(err); ok {
    return &lobbyv1.ApiResponse{
        Code:    errorCodeFromGRPC(st.Code()),
        Message: st.Message(),
    }, nil
}
```
