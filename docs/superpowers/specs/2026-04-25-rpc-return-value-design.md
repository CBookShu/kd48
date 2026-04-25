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
│       └── errors.go             # code → message 映射（服务端用）
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

### api/proto/common/v1/errors.go
```go
package commonv1

var ErrorMessages = map[int32]string{
    0:    "成功",
    1:    "请求参数错误",
    2:    "内部错误",
    3:    "服务不可用",
    100:  "用户不存在",
    101:  "未认证",
    200:  "今日已签到",
    201:  "签到期未开启",
    202:  "签到期已过期",
    300:  "物品不存在",
    301:  "物品不足",
}

func ErrorMessage(code int32) string {
    if msg, ok := ErrorMessages[code]; ok {
        return msg
    }
    return "未知错误"
}
```

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
)

// 将 gRPC 错误转为 ApiResponse
func toApiResponse(err error) *commonv1.ApiResponse {
    if err == nil {
        return &commonv1.ApiResponse{Code: 0}
    }
    st, ok := status.FromError(err)
    if !ok {
        return &commonv1.ApiResponse{
            Code:    commonv1.ErrorCode_INTERNAL_ERROR,
            Message: err.Error(),
        }
    }
    code := errorCodeFromGRPC(st.Code())
    return &commonv1.ApiResponse{
        Code:    code,
        Message: commonv1.ErrorMessage(code),
    }
}

// gRPC Code -> ErrorCode 映射
func errorCodeFromGRPC(code codes.Code) int32 {
    switch code {
    case codes.OK:
        return 0
    case codes.InvalidArgument:
        return 1
    case codes.Internal:
        return 2
    case codes.Unavailable:
        return 3
    case codes.Unauthenticated:
        return 101
    case codes.NotFound:
        return 100
    default:
        return 2
    }
}
```

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

**客户端行为：**
- 通过同步 proto 获取 ErrorCode 枚举（知道有哪些错误）
- 收到响应后，直接使用 `resp.Message` 显示给用户
- `resp.Code` 可用于业务逻辑判断（如跳转、弹窗等）
- **不需要** errors.go，不需要查表

**客户端示例：**
```go
if resp.Code != 0 {
    return resp.Message  // 直接用服务端返回的 message
}
```

---

## 需要修改的文件

### 新建公共包
- `api/proto/common/v1/api_response.proto` - 公共 ApiResponse + ErrorCode 定义
- `api/proto/common/v1/errors.go` - code → message 映射（服务端用）

### Proto 文件（删除 lobby 包的 ApiResponse）
- `api/proto/lobby/v1/common.proto` - 删除 ApiResponse + ErrorCode 定义
- `api/proto/lobby/v1/checkin.proto` - 改为直接返回 CheckinData / CheckinStatusData
- 其他 proto 文件如果有类似问题一并清理

### 生成的 Go 代码
- 重新生成 `api/proto/common/v1/` 包
- 更新所有 import 引用

### 网关层
- `gateway/cmd/gateway/ingress.go` - 使用 commonv1.ApiResponse，添加 gRPC status → ApiResponse 转换逻辑，填充 message

### 服务端
- `services/lobby/cmd/lobby/checkin_server.go` - 改用 status.Errorf
- `services/user/...` - 同样改为 status.Errorf（如果还没改）

### 客户端 (CLI)
- `cmd/cli/internal/commands/handler.go` - 更新 import 为 commonv1，移除对 Success 字段的检查
