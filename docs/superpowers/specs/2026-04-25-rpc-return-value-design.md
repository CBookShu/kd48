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

## 公共 ApiResponse 定义

### 文件结构
```
api/proto/
├── common/
│   └── v1/
│       ├── api_response.proto    # 公共 ApiResponse + ErrorCode
│       └── errors.go             # (可选) 错误消息参考
├── user/
│   └── v1/
│       └── user.proto
└── lobby/
    └── v1/
        ├── checkin.proto
        └── ...
```

### api/proto/common/v1/api_response.proto
```protobuf
syntax = "proto3";

package common.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/common/v1;commonv1";

import "google/protobuf/any.proto";

// ApiResponse 通用响应外壳（所有服务共用）
message ApiResponse {
  int32 code = 1;
  string message = 2;       // 错误描述，服务端填充
  google.protobuf.Any data = 3;
  map<string, string> meta = 4;  // 扩展属性
}

// ErrorCode 错误码定义（所有服务共用，全局同步一份 proto）
enum ErrorCode {
  SUCCESS = 0;

  // 通用错误 1-99
  INVALID_REQUEST = 1;
  INTERNAL_ERROR = 2;
  SERVICE_UNAVAILABLE = 3;

  // 用户相关 100-199
  USER_NOT_FOUND = 100;
  USER_NOT_AUTHENTICATED = 101;

  // 签到相关 200-299
  CHECKIN_ALREADY_TODAY = 200;
  CHECKIN_PERIOD_NOT_ACTIVE = 201;
  CHECKIN_PERIOD_EXPIRED = 202;

  // 物品相关 300-399
  ITEM_NOT_FOUND = 300;
  ITEM_INSUFFICIENT = 301;
}
```

### api/proto/common/v1/errors.go (可选)

errors.go 是可选的错误消息参考，用于：
- 服务端返回错误时构造 message
- 文档参考

```go
package commonv1

// 错误消息参考表（可选，用于构造错误或文档）
var ErrorMessages = map[int32]string{
    // 系统错误 (1xxx)
    1000: "请求参数错误",
    1001: "内部错误",
    1002: "服务不可用",
    1003: "未认证",

    // 用户相关 (2xxx)
    2000: "用户不存在",
    2001: "用户未认证",

    // 签到相关 (3xxx)
    3000: "今日已签到",
    3001: "签到期未开启",
    3002: "签到期已过期",

    // 物品相关 (4xxx)
    4000: "物品不存在",
    4001: "物品不足",
}
```

**注意**：网关不依赖此文件，直接透传服务端返回的 code 和 message。

### 响应格式

**成功：**
```json
{
  "code": 0,
  "message": "",
  "data": {"continuous_days": 5, "rewards": {...}}
}
```

**失败：**
```json
{
  "code": 200,
  "message": "今日已签到",
  "data": null
}
```

### 网关转换代码示例
```go
import (
    commonv1 "github.com/CBookShu/kd48/api/proto/common/v1"
    "google.golang.org/grpc/status"
)

// 将 gRPC 错误转为 ApiResponse（直接透传，不做映射）
func toApiResponse(err error) *commonv1.ApiResponse {
    if err == nil {
        return &commonv1.ApiResponse{Code: 0}
    }
    st, ok := status.FromError(err)
    if !ok {
        return &commonv1.ApiResponse{
            Code:    int32(commonv1.ErrorCode_INTERNAL_ERROR),
            Message: err.Error(),
        }
    }
    // 直接透传 gRPC 的 code 和 message
    return &commonv1.ApiResponse{
        Code:    int32(st.Code()),
        Message: st.Message(),
    }
}
```

---

## 错误码分类（类型 + 值）

### 设计规则

**高2位 = 类型**（十位百位）
**低2位 = 值**（个位十位）

```
1xxx  系统错误
2xxx  用户相关
3xxx  签到相关
4xxx  物品相关
...
```

### 具体定义

| 错误码 | 含义 |
|--------|------|
| 0 | 成功 |
| **1xxx 系统错误** | |
| 1000 | 请求参数错误 |
| 1001 | 内部错误 |
| 1002 | 服务不可用 |
| 1003 | 未认证（系统级） |
| **2xxx 用户相关** | |
| 2000 | 用户不存在 |
| 2001 | 用户未认证 |
| **3xxx 签到相关** | |
| 3000 | 今日已签到 |
| 3001 | 签到期未开启 |
| 3002 | 签到期已过期 |
| **4xxx 物品相关** | |
| 4000 | 物品不存在 |
| 4001 | 物品不足 |

### 网关透传规则

```go
func toApiResponse(err error) *commonv1.ApiResponse {
    st, _ := status.FromError(err)
    // 直接透传 code 和 message
    return &commonv1.ApiResponse{
        Code:    int32(st.Code()),
        Message: st.Message(),
    }
}
```

- 网关不映射、不转换
- 直接透传 code 和 message
- 客户端看到的就是服务端返回的

### 约定

- 业务错误使用 `codes.Code(3000)` 格式返回
- 高位表示错误类型，低位表示具体错误
- 类型内值可自行扩展（3003, 3004...）
- gRPC 标准码 (0-16) 保留系统使用，业务不冲突

---

## 服务端间调用 vs 客户端调用

| 调用路径 | 错误处理 |
|----------|----------|
| 客户端 ↔ 网关 | 用 ApiResponse (JSON over WebSocket) |
| 网关 ↔ 服务 | 用 gRPC 原生 status.Errorf |
| 服务 ↔ 服务 | 用 gRPC 原生 status.Errorf |

- 网关负责"翻译"：gRPC status ↔ ApiResponse
- 服务间调用不需要 ApiResponse

---

## 客户端处理

### 1. 消息解析流程

**CLI 客户端示例：**
```go
// gateway.go - 解析响应
type WsResponse struct {
    Method string      `json:"method"`
    Code   int32       `json:"code"`
    Msg    string      `json:"msg"`      // 错误描述（服务端填充）
    Data   interface{} `json:"data"`     // 业务数据
}

// handler.go - 使用响应
func (h *Handler) handleResponse(resp *client.WsResponse) string {
    if resp.Code != 0 {
        // 失败：直接用 Msg 显示给用户
        return fmt.Sprintf("[错误] %s", resp.Msg)
    }
    // 成功：处理 Data
    return "操作成功"
}
```

### 2. Code vs Message 用途

| 字段 | 用途 |
|------|------|
| `Code` | 业务逻辑判断（如：code==101 跳转登录页） |
| `Msg` | 直接显示给用户（服务端已填充好） |

### 3. 客户端不需要的错误处理代码

- ❌ 不需要 `errors.go`
- ❌ 不需要 `ErrorMessages` 映射表
- ❌ 不需要查表翻译 message

---

## 服务间 RPC 调用

### 调用关系

```
Gateway (ws) → User Service (gRPC)
             → Lobby Service (gRPC)
             
Lobby Service → User Service (gRPC)  // 服务间调用
```

### 服务间调用使用 gRPC 原生错误

**调用方（Client）：**
```go
// 使用 gRPC 标准客户端
resp, err := userClient.GetUser(ctx, &userv1.GetUserRequest{UserId: id})
if err != nil {
    // 直接处理 gRPC 错误，不需要包装
    st, _ := status.FromError(err)
    switch st.Code() {
    case codes.NotFound:
        return nil, status.Errorf(codes.InvalidArgument, "user not found")
    default:
        return nil, err  // 透传
    }
}
```

**被调用方（Server）：**
```go
// 直接返回 gRPC 原生错误
func (s *UserService) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.User, error) {
    user, err := s.store.GetUser(ctx, req.UserId)
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return nil, status.Errorf(codes.NotFound, "user not found")
        }
        return nil, status.Errorf(codes.Internal, "internal error")
    }
    return user, nil
}
```

### 服务间错误传播

**原则：** 错误在服务间直接透传，不做额外包装。

- Gateway → Service：Gateway 负责转换为 ApiResponse
- Service → Service：直接用 gRPC status 传递

---

## 需要修改的文件

### 新建公共包
- `api/proto/common/v1/api_response.proto` - 公共 ApiResponse + ErrorCode 定义
- `api/proto/common/v1/errors.go` - (可选) 错误消息参考，用于文档或辅助

### Proto 文件（删除 lobby 包的 ApiResponse）
- `api/proto/lobby/v1/common.proto` - 删除 ApiResponse + ErrorCode 定义
- `api/proto/lobby/v1/checkin.proto` - 改为直接返回 CheckinData / CheckinStatusData
- 其他 proto 文件如果有类似问题一并清理

### 生成的 Go 代码
- 重新生成 `api/proto/common/v1/` 包
- 更新所有 import 引用

### 网关层
- `gateway/cmd/gateway/ingress.go` - 使用 commonv1.ApiResponse，透传 gRPC status 的 code 和 message

### 服务端
- `services/lobby/cmd/lobby/checkin_server.go` - 改用 status.Errorf(codes.Code(200), "message")
- `services/user/...` - 同样改为 status.Errorf（如果还没改）

### 客户端 (CLI)
- `cmd/cli/internal/commands/handler.go` - 更新 import 为 commonv1，移除对 Success 字段的检查
