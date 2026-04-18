# dbpool Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 扩展 dbpool 配置项，实现基于 etcd 的动态路由配置管理

**Architecture:** 新增 RouteLoader 组件从 etcd 获取/监听路由配置，Router 原子更新路由表；连接池配置扩展生命周期和超时控制

**Tech Stack:** Go 1.26, etcd client v3, protobuf, database/sql, go-redis/v9

**设计文档:** `docs/superpowers/specs/2026-04-17-dbpool-enhancement-design.md`

---

## 文件结构

| 文件 | 职责 | 操作 |
|------|------|------|
| `api/proto/dsroute/v1/routing.proto` | 路由配置 Protobuf 定义 | 新建 |
| `pkg/dsroute/config.go` | 连接池配置结构体扩展 | 修改 |
| `pkg/dsroute/loader.go` | RouteLoader 接口与实现 | 新建 |
| `pkg/dsroute/loader_test.go` | RouteLoader 测试 | 新建 |
| `pkg/dsroute/router.go` | Router 支持动态更新 | 修改 |
| `pkg/conf/conf.go` | 配置结构体扩展 | 修改 |
| `services/user/cmd/user/main.go` | 集成 RouteLoader 和扩展配置 | 修改 |

---

## Task 1: Protobuf 定义

**Files:**
- Create: `api/proto/dsroute/v1/routing.proto`

- [ ] **Step 1: 创建 proto 文件**

```protobuf
syntax = "proto3";
package dsroute.v1;

option go_package = "github.com/CBookShu/kd48/api/proto/dsroute/v1";

// 单条路由规则
message RouteRule {
  // 路由前缀，空字符串表示默认匹配
  string prefix = 1;
  // 目标连接池名称
  string pool   = 2;
}

// MySQL 路由配置
message MySQLRoutes {
  repeated RouteRule rules = 1;
}

// Redis 路由配置
message RedisRoutes {
  repeated RouteRule rules = 1;
}
```

- [ ] **Step 2: 生成 Go 代码**

```bash
cd /Users/cbookshu/dev/temp/kd48
protoc --go_out=. --go_opt=paths=source_relative api/proto/dsroute/v1/routing.proto
```

Expected: 生成 `api/proto/dsroute/v1/routing.pb.go`

- [ ] **Step 3: 验证生成文件存在**

```bash
ls -la api/proto/dsroute/v1/routing.pb.go
```

Expected: 文件存在

- [ ] **Step 4: Commit**

```bash
git add api/proto/dsroute/v1/
git commit -m "feat(dsroute): add routing protobuf definitions"
```

---

## Task 2: 扩展连接池配置

**Files:**
- Modify: `pkg/dsroute/config.go`

- [ ] **Step 1: 扩展 MySQLPoolSpec**

```go
type MySQLPoolSpec struct {
	DSN              string        `yaml:"dsn" json:"dsn"`
	MaxOpen          int           `yaml:"max_open" json:"max_open"`
	MaxIdle          int           `yaml:"max_idle" json:"max_idle"`
	ConnMaxLifetime  time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime  time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
	MaxRetry         int           `yaml:"max_retry" json:"max_retry"`
	RetryInterval    time.Duration `yaml:"retry_interval" json:"retry_interval"`
}
```

- [ ] **Step 2: 扩展 RedisPoolSpec**

```go
type RedisPoolSpec struct {
	Addr         string        `yaml:"addr" json:"addr"`
	DB           int           `yaml:"db" json:"db"`
	Password     string        `yaml:"password" json:"password"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
}
```

- [ ] **Step 3: 添加 time 包导入**

```go
import "time"
```

- [ ] **Step 4: Commit**

```bash
git add pkg/dsroute/config.go
git commit -m "feat(dsroute): extend pool configuration with lifecycle and timeout settings"
```

---

## Task 3: pkg/conf 配置扩展

**Files:**
- Modify: `pkg/conf/conf.go`

- [ ] **Step 1: 扩展 MySQLPoolConf**

在 `MySQLPoolConf` struct 中添加：
```go
ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
```

- [ ] **Step 2: 扩展 RedisPoolConf**

在 `RedisPoolConf` struct 中添加：
```go
MinIdleConns int           `mapstructure:"min_idle_conns"`
DialTimeout  time.Duration `mapstructure:"dial_timeout"`
ReadTimeout  time.Duration `mapstructure:"read_timeout"`
WriteTimeout time.Duration `mapstructure:"write_timeout"`
```

- [ ] **Step 3: 更新 ToDSRouteConfig 转换逻辑**

在 `ToDSRouteConfig` 函数中，转换新字段：
```go
mysqlPools[k] = dsroute.MySQLPoolSpec{
	DSN:             v.DSN,
	MaxOpen:         v.MaxOpen,
	MaxIdle:         v.MaxIdle,
	ConnMaxLifetime: v.ConnMaxLifetime,
	ConnMaxIdleTime: v.ConnMaxIdleTime,
}
```

```go
redisPools[k] = dsroute.RedisPoolSpec{
	Addr:         v.Addr,
	DB:           v.DB,
	Password:     v.Password,
	PoolSize:     v.PoolSize,
	MinIdleConns: v.MinIdleConns,
	DialTimeout:  v.DialTimeout,
	ReadTimeout:  v.ReadTimeout,
	WriteTimeout: v.WriteTimeout,
}
```

- [ ] **Step 4: 添加 time 包导入**

- [ ] **Step 5: Commit**

```bash
git add pkg/conf/conf.go
git commit -m "feat(conf): extend pool config structs with new fields"
```

---

## Task 4: RouteLoader 接口与实现

**Files:**
- Create: `pkg/dsroute/loader.go`
- Create: `pkg/dsroute/loader_test.go`

- [ ] **Step 1: 编写 RouteLoader 接口和结构体**

```go
package dsroute

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// RoutingConfig 包含所有路由配置
type RoutingConfig struct {
	MySQLRoutes []RouteRule
	RedisRoutes []RouteRule
}

// RouteLoader 从 etcd 加载和监听路由配置
type RouteLoader struct {
	client *clientv3.Client
	prefix string
}

// NewRouteLoader 创建 RouteLoader
func NewRouteLoader(client *clientv3.Client, prefix string) *RouteLoader {
	if prefix == "" {
		prefix = "kd48/routing"
	}
	return &RouteLoader{
		client: client,
		prefix: prefix,
	}
}

// Get 从 etcd 获取当前路由配置
func (l *RouteLoader) Get(ctx context.Context) (*RoutingConfig, error) {
	mysqlKey := l.prefix + "/mysql_routes"
	redisKey := l.prefix + "/redis_routes"

	mysqlRoutes, err := l.getRoutes(ctx, mysqlKey)
	if err != nil {
		return nil, fmt.Errorf("get mysql routes: %w", err)
	}

	redisRoutes, err := l.getRoutes(ctx, redisKey)
	if err != nil {
		return nil, fmt.Errorf("get redis routes: %w", err)
	}

	return &RoutingConfig{
		MySQLRoutes: mysqlRoutes,
		RedisRoutes: redisRoutes,
	}, nil
}

func (l *RouteLoader) getRoutes(ctx context.Context, key string) ([]RouteRule, error) {
	resp, err := l.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("key %s not found", key)
	}

	var rules []RouteRule
	if err := json.Unmarshal(resp.Kvs[0].Value, &rules); err != nil {
		return nil, fmt.Errorf("unmarshal routes: %w", err)
	}

	return rules, nil
}
```

- [ ] **Step 2: 编写 Watch 方法**

```go
// Watch 监听配置变更
func (l *RouteLoader) Watch(ctx context.Context, onChange func(*RoutingConfig)) error {
	mysqlKey := l.prefix + "/mysql_routes"
	redisKey := l.prefix + "/redis_routes"

	watchChan := l.client.Watch(ctx, l.prefix, clientv3.WithPrefix())

	for resp := range watchChan {
		if resp.Err() != nil {
			slog.Error("watch error", "error", resp.Err())
			continue
		}

		// 检查是否是我们关心的 key
		needUpdate := false
		for _, ev := range resp.Events {
			key := string(ev.Kv.Key)
			if key == mysqlKey || key == redisKey {
				needUpdate = true
				break
			}
		}

		if !needUpdate {
			continue
		}

		// 重新获取配置
		config, err := l.Get(ctx)
		if err != nil {
			slog.Error("failed to get updated routes", "error", err)
			continue
		}

		onChange(config)
	}

	return ctx.Err()
}
```

- [ ] **Step 3: 编写测试（失败优先）**

```go
package dsroute

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestRouteLoader_Get_NotFound(t *testing.T) {
	loader := NewRouteLoader(nil, "test/routing")
	_, err := loader.Get(context.Background())
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}
```

- [ ] **Step 4: 运行测试验证失败**

```bash
cd /Users/cbookshu/dev/temp/kd48
go test ./pkg/dsroute/... -run TestRouteLoader_Get_NotFound -v
```

Expected: FAIL with "runtime error: invalid memory address"

- [ ] **Step 5: Commit**

```bash
git add pkg/dsroute/loader.go pkg/dsroute/loader_test.go
git commit -m "feat(dsroute): add RouteLoader for etcd-based routing config"
```

---

## Task 5: Router 支持动态更新

**Files:**
- Modify: `pkg/dsroute/router.go`

- [ ] **Step 1: 添加 atomic 路由表**

```go
import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
)

type Router struct {
	mysqlPools  map[string]*sql.DB
	redisPools  map[string]redis.UniversalClient
	mysqlRoutes atomic.Value // []RouteRule
	redisRoutes atomic.Value // []RouteRule
}
```

- [ ] **Step 2: 修改 NewRouter 初始化**

```go
func NewRouter(
	mysqlPools map[string]*sql.DB,
	redisPools map[string]redis.UniversalClient,
	mysqlRoutes []RouteRule,
	redisRoutes []RouteRule,
) (*Router, error) {
	// 验证 pool 存在性...
	// (原有验证逻辑)

	r := &Router{
		mysqlPools: mysqlPools,
		redisPools: redisPools,
	}

	// 原子存储初始路由
	r.mysqlRoutes.Store(mysqlRoutes)
	r.redisRoutes.Store(redisRoutes)

	return r, nil
}
```

- [ ] **Step 3: 添加 UpdateRoutes 方法**

```go
// UpdateRoutes 原子更新路由表
func (r *Router) UpdateRoutes(mysqlRoutes, redisRoutes []RouteRule) {
	r.mysqlRoutes.Store(mysqlRoutes)
	r.redisRoutes.Store(redisRoutes)
	slog.Info("router routes updated", 
		"mysql_routes", len(mysqlRoutes),
		"redis_routes", len(redisRoutes))
}
```

- [ ] **Step 4: 修改 Resolve 方法使用 atomic 值**

```go
func (r *Router) ResolveDB(ctx context.Context, routingKey string) (*sql.DB, string, error) {
	routes := r.mysqlRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	db := r.mysqlPools[poolName]
	return db, poolName, nil
}

func (r *Router) ResolveRedis(ctx context.Context, routingKey string) (redis.UniversalClient, string, error) {
	routes := r.redisRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	client := r.redisPools[poolName]
	return client, poolName, nil
}
```

- [ ] **Step 5: Commit**

```bash
git add pkg/dsroute/router.go
git commit -m "feat(dsroute): router supports atomic route updates"
```

---

## Task 6: main.go 集成

**Files:**
- Modify: `services/user/cmd/user/main.go`

- [ ] **Step 1: 扩展 MySQL 连接池创建**

```go
for name, spec := range dsCfg.MySQLPools {
	db, err := sql.Open("mysql", spec.DSN)
	if err != nil {
		panic(fmt.Errorf("failed to open mysql pool %q: %w", name, err))
	}
	
	// 设置连接池参数
	if spec.MaxOpen > 0 {
		db.SetMaxOpenConns(spec.MaxOpen)
	}
	if spec.MaxIdle > 0 {
		db.SetMaxIdleConns(spec.MaxIdle)
	}
	if spec.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(spec.ConnMaxLifetime)
	}
	if spec.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(spec.ConnMaxIdleTime)
	}
	
	// 连接健康检查
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.Ping(ctx); err != nil {
		cancel()
		panic(fmt.Errorf("failed to ping mysql pool %q: %w", name, err))
	}
	cancel()
	
	mysqlPools[name] = db
}
```

- [ ] **Step 2: 扩展 Redis 连接池创建**

```go
for name, spec := range dsCfg.RedisPools {
	opts := &redis.Options{
		Addr:         spec.Addr,
		Password:     spec.Password,
		DB:           spec.DB,
		PoolSize:     spec.PoolSize,
		MinIdleConns: spec.MinIdleConns,
	}
	if spec.DialTimeout > 0 {
		opts.DialTimeout = spec.DialTimeout
	}
	if spec.ReadTimeout > 0 {
		opts.ReadTimeout = spec.ReadTimeout
	}
	if spec.WriteTimeout > 0 {
		opts.WriteTimeout = spec.WriteTimeout
	}
	
	rdb := redis.NewClient(opts)
	
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(fmt.Errorf("failed to ping redis pool %q: %w", name, err))
	}
	
	redisPools[name] = rdb
}
```

- [ ] **Step 3: 从 etcd 加载路由配置**

```go
// 从 etcd 加载路由配置
routeLoader := dsroute.NewRouteLoader(etcdCli, "kd48/routing")
routingCfg, err := routeLoader.Get(context.Background())
if err != nil {
	slog.Error("failed to load routing config from etcd", "error", err)
	os.Exit(1)
}

// 创建 Router
router, err := dsroute.NewRouter(
	mysqlPools, 
	redisPools,
	routingCfg.MySQLRoutes,
	routingCfg.RedisRoutes,
)
if err != nil {
	panic(fmt.Errorf("failed to create router: %w", err))
}

// 启动路由配置监听
go func() {
	if err := routeLoader.Watch(context.Background(), func(cfg *dsroute.RoutingConfig) {
		router.UpdateRoutes(cfg.MySQLRoutes, cfg.RedisRoutes)
	}); err != nil {
		slog.Error("route watcher stopped", "error", err)
	}
}()
```

- [ ] **Step 4: 运行构建验证**

```bash
cd /Users/cbookshu/dev/temp/kd48
go build ./services/user/cmd/user/...
```

Expected: 编译成功

- [ ] **Step 5: Commit**

```bash
git add services/user/cmd/user/main.go
git commit -m "feat(user): integrate RouteLoader and extended pool configs"
```

---

## Task 7: 添加 RouteLoader 集成测试

**Files:**
- Create: `pkg/dsroute/loader_integration_test.go`

- [ ] **Step 1: 编写集成测试框架**

```go
//go:build integration
// +build integration

package dsroute

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestRouteLoader_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to connect etcd: %v", err)
	}
	defer cli.Close()

	loader := NewRouteLoader(cli, "test/routing")
	ctx := context.Background()

	// 设置测试数据
	mysqlRoutes := []RouteRule{{Prefix: "user_", Pool: "user_pool"}}
	redisRoutes := []RouteRule{{Prefix: "session_", Pool: "session_pool"}}

	mysqlData, _ := json.Marshal(mysqlRoutes)
	redisData, _ := json.Marshal(redisRoutes)

	_, err = cli.Put(ctx, "test/routing/mysql_routes", string(mysqlData))
	if err != nil {
		t.Fatalf("failed to put mysql routes: %v", err)
	}
	_, err = cli.Put(ctx, "test/routing/redis_routes", string(redisData))
	if err != nil {
		t.Fatalf("failed to put redis routes: %v", err)
	}

	// 测试 Get
	cfg, err := loader.Get(ctx)
	if err != nil {
		t.Fatalf("failed to get routes: %v", err)
	}

	if len(cfg.MySQLRoutes) != 1 || cfg.MySQLRoutes[0].Prefix != "user_" {
		t.Errorf("unexpected mysql routes: %+v", cfg.MySQLRoutes)
	}

	// 清理
	cli.Delete(ctx, "test/routing/", clientv3.WithPrefix())
}
```

- [ ] **Step 2: Commit**

```bash
git add pkg/dsroute/loader_integration_test.go
git commit -m "test(dsroute): add RouteLoader integration tests"
```

---

## 自我审查

| 检查项 | 状态 |
|--------|------|
| **Spec 覆盖** | ✅ 所有设计点已覆盖 |
| **Placeholder 扫描** | ✅ 无 TBD/TODO/"add validation" 等 |
| **类型一致性** | `RouteRule` / `RoutingConfig` / `RouteLoader` 全 plan 一致 |

**覆盖检查：**
- [x] Protobuf 定义 → Task 1
- [x] 连接池配置扩展 → Task 2, Task 3
- [x] RouteLoader 实现 → Task 4
- [x] Router 动态更新 → Task 5
- [x] main.go 集成 → Task 6
- [x] 连接健康检查 → Task 6 Step 1
- [x] etcd Watch 机制 → Task 4 Step 2
- [x] 失败处理 → 各 Step 代码中包含

---

## 执行方式选择

**Plan complete and saved to `docs/superpowers/plans/2026-04-17-dbpool-enhancement-plan.md`.**

**Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints for review

**Which approach?**
