# 签到活动设计文档

> 创建日期: 2026-04-23
> 状态: 待实现

---

## 概述

用签到活动驱动基础设施搭建，验证全链路。签到功能扩展到 Lobby 服务，复用现有配置加载机制。

---

## 数据模型

### 配置表（五张表，CSV → MySQL）

#### 1. 签到期配置表 `checkin_period`

```csv
期ID,期名称,开始时间,结束时间,状态
period_id,period_name,start_time,end_time,status
int64,string,time,time,string
1,新春签到活动,2026-01-01 00:00:00,2026-01-31 23:59:59,closed
2,日常签到,2026-02-01 00:00:00,2026-12-31 23:59:59,active
```

#### 2. 每日签到奖励表 `checkin_daily_reward`

```csv
天数,奖励物品映射
day,rewards
int32,map<int32,int64>
1,1001:100|1002:10
2,1001:150|1002:15
7,1001:500|1003:1
```

#### 3. 连续签到奖励表 `checkin_continuous_reward`

```csv
连续天数,奖励物品映射
continuous_days,rewards
int32,map<int32,int64>
7,2001:1
30,2002:1
```

#### 4. 物品配置表 `item_config`

```csv
物品ID,物品名称,物品类型,描述,图标
item_id,item_name,item_type,description,icon
int32,string,string,string,string
1001,金币,currency,游戏金币,icon_coin
1002,钻石,currency,充值钻石,icon_diamond
2001,礼盒,item,普通礼盒,icon_gift_box
2002,传说礼盒,item,传说礼盒,icon_legendary
```

#### 5. 玩家物品表 `user_items`

**MySQL**：
```sql
CREATE TABLE user_items (
  user_id BIGINT NOT NULL,
  item_id INT NOT NULL,
  count BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, item_id)
);
```

**Redis**（Hash）：
```
Key: kd48:user_items:{user_id}
Type: Hash
  field: item_id
  value: count

示例：
HGETALL kd48:user_items:12345
  1001 => 10000
  1002 => 500
  2001 => 3
```

### 玩家签到数据

**MySQL**：
```sql
CREATE TABLE user_checkin (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  user_id BIGINT NOT NULL,
  period_id BIGINT NOT NULL,
  last_checkin_date DATE NOT NULL,
  continuous_days INT NOT NULL DEFAULT 0,
  claimed_days JSON,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_user (user_id)
);
```

**Redis**：
```
Key: kd48:checkin:{user_id}
Value: {
  "period_id": 2,
  "last_checkin_date": "2026-04-23",
  "continuous_days": 5,
  "claimed_days": [1,2,3,4,5]
}
```

---

## API 设计

### Proto 定义

```protobuf
// common.proto - 通用响应外壳
message ApiResponse {
  int32 code = 1;
  string message = 2;
  google.protobuf.Any data = 3;
  map<string, string> meta = 4;
}

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

// item.proto - 物品服务
service ItemService {
  rpc GetMyItems(GetMyItemsRequest) returns (ApiResponse);
}

message GetMyItemsRequest {}

message MyItemsData {
  map<int32, int64> items = 1;
}

// checkin.proto - 签到服务
service CheckinService {
  rpc Checkin(CheckinRequest) returns (ApiResponse);
  rpc GetStatus(GetStatusRequest) returns (ApiResponse);
}

message CheckinRequest {}

message CheckinData {
  int32 continuous_days = 1;
  map<int32, int64> rewards = 2;
}

message GetStatusRequest {}

message CheckinStatusData {
  int64 period_id = 1;
  string period_name = 2;
  repeated DailyRewardInfo daily_rewards = 3;
  repeated ContinuousRewardInfo continuous_rewards = 4;
  bool today_checked = 10;
  int32 continuous_days = 11;
  int32 total_days = 12;
  repeated int32 claimed_continuous = 13;
}

message DailyRewardInfo {
  int32 day = 1;
  map<int32, int64> rewards = 2;
}

message ContinuousRewardInfo {
  int32 continuous_days = 1;
  map<int32, int64> rewards = 2;
}
```

---

## 业务流程

### 签到流程

```
1. 获取当前 active 期配置
   - 从 ConfigStore 获取 checkin_period
   - 筛选 status=active 且在时间范围内的期

2. 检查玩家签到状态
   - 从 Redis 读取玩家签到数据
   - 比较 period_id：
     • 不匹配 → 新期开始，重置状态
     • 匹配 → 继续检查
   - 检查 last_checkin_date：
     • 今天 → 返回 "今日已签到"
     • 昨天 → 连续天数+1
     • 更早 → 连续天数重置为1

3. 发放奖励
   - 查询 checkin_daily_reward[day] → 发放
   - 检查 checkin_continuous_reward[continuous_days]
     • 存在且未领取 → 发放并标记已领取
   - 奖励写入 user_items（Redis HINCRBY + MySQL 同步）

4. 更新玩家签到状态
   - Redis 更新签到数据
   - MySQL 同步更新

5. 返回响应
   - ApiResponse { code: 0, data: CheckinData }
```

### 持久化方案

**当前方案：同步持久化**

```
签到时：
1. Redis 更新物品数量
2. MySQL 更新物品数量（事务）
3. Redis 更新签到状态
4. MySQL 更新签到状态（事务）
5. 返回成功
```

---

## 观测环境

### 组件

| 组件 | 端口 | 用途 |
|------|------|------|
| Jaeger | 16686 | 链路追踪 |
| Prometheus | 9090 | 指标采集 |
| Grafana | 3000 | 可视化仪表板 |

### 集成点

```
签到请求 → Gateway (OTel Span) → Lobby Service (OTel Span)
                                        ↓
                                  Jaeger Collector
                                        ↓
                                   Jaeger UI

Prometheus ← 拉取指标 ← Gateway/Lobby (OTel Metrics)
     ↓
Grafana ← 查询 Prometheus
```

---

## 文件结构

```
api/proto/lobby/v1/
├── lobby.proto              # 现有 Ping（保留）
├── checkin.proto            # 签到服务 Proto
├── item.proto               # 物品服务 Proto
└── common.proto             # ApiResponse 通用响应

services/lobby/
├── cmd/lobby/
│   ├── main.go
│   ├── server.go            # 现有 LobbyService
│   ├── checkin_server.go    # 新增：CheckinService
│   └── item_server.go       # 新增：ItemService
├── internal/
│   ├── config/              # 现有配置加载
│   ├── checkin/
│   │   ├── store.go         # 签到状态存储（Redis + MySQL）
│   │   └── reward.go        # 奖励发放逻辑
│   └── item/
│       └── store.go         # 物品存储（Redis + MySQL）
└── migrations/
    └── 001_create_checkin_tables.sql

tools/config-loader/testdata/
├── checkin_period.csv
├── checkin_daily_reward.csv
├── checkin_continuous_reward.csv
└── item_config.csv

docker/
├── docker-compose.yml       # 添加 Jaeger/Prometheus/Grafana
├── prometheus.yml
└── grafana/dashboards/

web/
├── index.html               # 入口/主页
├── login.html               # 登录页
├── register.html            # 注册页
├── checkin.html             # 签到页
├── items.html               # 道具背包页
├── css/
│   └── style.css
└── js/
    ├── api.js               # API 调用封装
    ├── ws.js                # WebSocket 连接
    └── checkin.js           # 签到逻辑
```

---

## Web 客户端

### 页面

| 页面 | 功能 |
|------|------|
| index.html | 入口/主页 |
| login.html | 登录 |
| register.html | 注册 |
| checkin.html | 签到状态展示、签到操作 |
| items.html | 道具背包展示 |

### 签到页布局

```
┌─────────────────────────────────────┐
│  签到状态                            │
│  连续签到: 5 天                       │
│  累计签到: 30 天                      │
├─────────────────────────────────────┤
│  [第1天] ✓  [第2天] ✓  ... [第7天] ○  │
├─────────────────────────────────────┤
│  [立即签到]                          │
├─────────────────────────────────────┤
│  签到成功！                          │
│  获得奖励: 物品1001 x 100, 物品1002 x 10 │
└─────────────────────────────────────┘
```

### 道具背包页布局

```
┌─────────────────────────────────────┐
│  我的道具                            │
├─────────────────────────────────────┤
│  物品ID: 1001    数量: 10,000        │
│  物品ID: 1002    数量: 500           │
│  物品ID: 2001    数量: 3             │
└─────────────────────────────────────┘
```

---

## TODO（未来待实现）

| # | 任务 | 说明 |
|---|------|------|
| 1 | 通用异步持久化方案 | 设计可靠的 Redis → MySQL 异步同步机制 |
| 2 | Cron Task 服务 | 独立的定时任务服务，解决分布式环境下的定时任务问题 |
| 3 | 移除 Ping RPC | 服务健康检查由 K8s/Docker 探针负责 |
