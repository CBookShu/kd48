# UserService MySQL 路由集成设计

## 批准记录（人类门闩）

- **状态**：已批准
- **批准范围**：全文
- **批准人 / 日期**：用户 / 2026-04-18
- **TDD**：强制
- **Subagent**：按任务拆分

---

## 1. 目标

让 UserService 的 MySQL 业务使用 Router 进行动态路由，与 Redis 路由保持一致。

---

## 2. 当前状态

| 组件 | 使用 Router 情况 |
|------|------------------|
| Redis | ✅ 已使用（`ResolveRedis`） |
| MySQL | ❌ 未使用（固定 `s.qc` 单一 Queries） |

**问题**: `s.qc` 是启动时创建的单一 `*sqlc.Queries`，绑定了 `default` 池，无法动态路由。

---

## 3. 设计方案

### 3.1 方案选择

采用**辅助方法封装**方案：
- 业务层自行构造 routingKey
- 通过 `getQueries()` 辅助方法获取 Queries
- `sqlc.New(db)` 动态创建，极轻量无 GC 压力

### 3.2 routingKey 格式

**格式**: `{category}:{table}:{key}`

| 业务 | category | table | key | routingKey 示例 |
|------|----------|-------|-----|-----------------|
| 用户登录/注册 | sys | user | username | `sys:user:alice` |
| Session | sys | session | - | `sys:session` |

**category 分类**:
- `sys` - 系统基础数据（用户、配置等）
- `game` - 游戏业务数据
- `activity` - 活动数据
- 可自定义扩展

---

## 4. 代码变更

### 4.1 main.go

```go
// 删除
queries := sqlc.New(mysqlPools["default"])

// 修改 NewUserService 调用（移除 queries 参数）
userSvc := NewUserService(router, time.Duration(c.Session.ExpireHours)*time.Hour)
```

### 4.2 server.go

```go
type userService struct {
    userv1.UnimplementedUserServiceServer
    router   *dsroute.Router  // 只保留 router，移除 qc
    tokenTTL time.Duration
}

func NewUserService(router *dsroute.Router, tokenTTL time.Duration) *userService {
    return &userService{
        router:   router,
        tokenTTL: tokenTTL,
    }
}

// 新增辅助方法
func (s *userService) getQueries(ctx context.Context, routingKey string) (*sqlc.Queries, error) {
    db, _, err := s.router.ResolveDB(ctx, routingKey)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "resolve db route: %v", err)
    }
    return sqlc.New(db), nil
}

// 业务方法改造示例
func (s *userService) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error) {
    routingKey := "sys:user:" + req.Username
    
    queries, err := s.getQueries(ctx, routingKey)
    if err != nil {
        return nil, err
    }
    
    user, err := queries.GetUserByUsername(ctx, req.Username)
    // ... 后续逻辑不变
}

func (s *userService) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterReply, error) {
    routingKey := "sys:user:" + req.Username
    
    queries, err := s.getQueries(ctx, routingKey)
    if err != nil {
        return nil, err
    }
    
    // ... 使用 queries
}
```

---

## 5. etcd 路由配置

初始化数据示例：

```bash
etcdctl put kd48/routing/mysql_routes '[
  {"prefix": "sys:user:", "pool": "default"},
  {"prefix": "sys:session", "pool": "default"},
  {"prefix": "game:", "pool": "game_pool"},
  {"prefix": "activity:", "pool": "activity_pool"},
  {"prefix": "", "pool": "default"}
]'

etcdctl put kd48/routing/redis_routes '[
  {"prefix": "sys:session", "pool": "default"},
  {"prefix": "", "pool": "default"}
]'
```

---

## 6. 测试要点

- [ ] `getQueries` 路由解析成功
- [ ] `getQueries` 路由失败返回错误
- [ ] Login 使用正确 routingKey
- [ ] Register 使用正确 routingKey
- [ ] 现有功能回归测试通过

---

## 7. 影响范围

| 文件 | 变更类型 |
|------|----------|
| `services/user/cmd/user/main.go` | 修改 |
| `services/user/cmd/user/server.go` | 修改 |

---

## 8. 后续扩展

- 新增业务表时，按 `{category}:{table}:` 格式定义 routingKey
- 分库时，修改 etcd 路由配置即可，无需改代码
