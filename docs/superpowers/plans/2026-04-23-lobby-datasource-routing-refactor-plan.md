# lobby 服务数据源访问规范化重构 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 lobby 服务的数据源访问从不规范的直接访问改为通过 `dsroute.Router` 解析

**Architecture:** 所有组件（lobbyService、ConfigLoader、ConfigWatcher）改为持有 `*dsroute.Router`，通过 routing key 在运行时解析连接。参考 user 服务的实现模式。

**Tech Stack:** Go 1.26, dsroute.Router, etcd, gRPC

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `services/lobby/internal/config/loader.go` | 修改 | ConfigLoader 改为注入 Router |
| `services/lobby/internal/config/watcher.go` | 修改 | ConfigWatcher 改为注入 Router |
| `services/lobby/internal/config/loader_test.go` | 修改 | 适配新构造函数 |
| `services/lobby/internal/config/watcher_test.go` | 修改 | 适配新构造函数 |
| `services/lobby/cmd/lobby/server.go` | 修改 | lobbyService 改为注入 Router |
| `services/lobby/cmd/lobby/server_test.go` | 修改 | 适配新构造函数 |
| `services/lobby/cmd/lobby/main.go` | 修改 | 创建 Router 并注入所有组件 |

---

## Task 1: 重构 ConfigLoader 使用 dsroute.Router

**Files:**
- Modify: `services/lobby/internal/config/loader.go`
- Modify: `services/lobby/internal/config/loader_test.go`

- [ ] **Step 1: 修改 ConfigLoader 结构体和构造函数**

修改 `services/lobby/internal/config/loader.go`：

```go
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/CBookShu/kd48/pkg/dsroute"
)

// ConfigLoader 从 MySQL 加载配置
type ConfigLoader struct {
	router     *dsroute.Router
	routingKey string
	store      *ConfigStore
}

// NewConfigLoader 创建加载器
func NewConfigLoader(router *dsroute.Router, routingKey string, store *ConfigStore) *ConfigLoader {
	return &ConfigLoader{router: router, routingKey: routingKey, store: store}
}

// LoadOne 加载单个配置
func (l *ConfigLoader) LoadOne(ctx context.Context, name string) error {
	// 1. 通过 Router 解析数据库连接
	db, poolName, err := l.router.ResolveDB(ctx, l.routingKey)
	if err != nil {
		return fmt.Errorf("resolve db for routing key %q: %w", l.routingKey, err)
	}

	// 2. 获取对应的 TypedStore
	ts := l.store.GetTypedStore(name)
	if ts == nil {
		return fmt.Errorf("config %s not registered", name)
	}

	// 3. 从 MySQL 读取最新版本
	query := `
		SELECT data, revision
		FROM lobby_config_revision
		WHERE config_name = ?
		ORDER BY revision DESC
		LIMIT 1
	`

	var data []byte
	var revision int64
	err = db.QueryRowContext(ctx, query, name).Scan(&data, &revision)
	if err != nil {
		return fmt.Errorf("query config %s: %w", name, err)
	}

	// 4. 解析 JSON 并更新 Store
	return l.parseAndUpdate(name, data, revision, ts)
}

// parseAndUpdate 解析 JSON 并更新 TypedStore
func (l *ConfigLoader) parseAndUpdate(name string, data []byte, revision int64, ts any) error {
	// 使用反射获取 TypedStore 的类型参数并解析
	storeValue := reflect.ValueOf(ts)
	if storeValue.Kind() != reflect.Ptr {
		return fmt.Errorf("typed store must be a pointer")
	}

	// 获取 TypedStore[T] 的类型参数 T
	storeType := storeValue.Elem().Type()
	// TypedStore 结构体中 snapshot 字段是 *Snapshot[T]
	snapshotField, ok := storeType.FieldByName("snapshot")
	if !ok {
		return fmt.Errorf("typed store missing snapshot field")
	}

	// snapshot 是 *Snapshot[T]，所以先去掉指针，再找 Data 字段
	snapshotType := snapshotField.Type
	if snapshotType.Kind() == reflect.Ptr {
		snapshotType = snapshotType.Elem()
	}

	dataField, ok := snapshotType.FieldByName("Data")
	if !ok {
		return fmt.Errorf("snapshot missing Data field")
	}

	// Data 是 []T，获取元素类型 T
	sliceType := dataField.Type
	if sliceType.Kind() != reflect.Slice {
		return fmt.Errorf("Data field must be a slice")
	}

	// 创建 []T 类型的变量来解析 JSON
	slicePtr := reflect.New(sliceType)
	if err := json.Unmarshal(data, slicePtr.Interface()); err != nil {
		return fmt.Errorf("parse config %s as %v: %w", name, sliceType, err)
	}

	// 调用 Update 方法
	updateMethod := storeValue.MethodByName("Update")
	if !updateMethod.IsValid() {
		return fmt.Errorf("typed store missing Update method")
	}

	// 准备参数：revision (int64), data ([]T)
	args := []reflect.Value{
		reflect.ValueOf(revision),
		slicePtr.Elem(),
	}
	updateMethod.Call(args)

	slog.Debug("config loaded", "name", name, "revision", revision, "pool", l.routingKey)
	return nil
}

// LoadAll 加载所有已注册的配置
func (l *ConfigLoader) LoadAll(ctx context.Context) error {
	names := l.store.GetRegisteredNames()
	for _, name := range names {
		if err := l.LoadOne(ctx, name); err != nil {
			slog.Warn("failed to load config", "name", name, "error", err)
			// 继续加载其他配置
		}
	}
	return nil
}
```

- [ ] **Step 2: 创建测试辅助函数 newTestRouter**

修改 `services/lobby/internal/config/loader_test.go`，添加测试 Router 创建函数：

```go
package config

import (
	"context"
	"database/sql"
	"testing"

	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/DATA-DOG/go-sqlmock"
)

const testRoutingKey = "lobby:config-data"

// newTestRouter 创建测试用 Router，包含 mock 数据库
func newTestRouter(t *testing.T, db *sql.DB) *dsroute.Router {
	mysqlPools := map[string]*sql.DB{"default": db}
	redisPools := map[string]redis.UniversalClient{} // loader 不需要 redis

	routes := []dsroute.RouteRule{
		{Prefix: testRoutingKey, Pool: "default"},
	}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, routes, nil)
	if err != nil {
		t.Fatalf("failed to create test router: %v", err)
	}
	return router
}
```

需要在文件顶部添加 import：

```go
import (
	"context"
	"database/sql"
	"testing"

	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/redis/go-redis/v9"
)
```

- [ ] **Step 3: 更新 TestLoadOne_Success 测试**

修改 `services/lobby/internal/config/loader_test.go`：

```go
func TestLoadOne_Success(t *testing.T) {
	ResetStore()

	// 注册配置
	pkg := &testConfig{name: "test_config"}
	Register[int](pkg)

	// 创建 mock 数据库
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 模拟查询返回
	rows := sqlmock.NewRows([]string{"data", "revision"}).
		AddRow(`[1, 2, 3]`, 42)
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("test_config").
		WillReturnRows(rows)

	// 创建测试 Router
	router := newTestRouter(t, db)

	// 执行加载
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	err = loader.LoadOne(context.Background(), "test_config")
	if err != nil {
		t.Fatalf("LoadOne() error = %v", err)
	}

	// 验证数据已加载
	ts := GetStore().GetTypedStore("test_config").(*TypedStore[int])
	snap := ts.Get()
	if snap == nil {
		t.Fatal("snapshot is nil after LoadOne")
	}
	if snap.Revision != 42 {
		t.Errorf("Revision = %d, want 42", snap.Revision)
	}
	if len(snap.Data) != 3 {
		t.Errorf("Data length = %d, want 3", len(snap.Data))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
```

- [ ] **Step 4: 更新 TestLoadOne_NotFound 测试**

修改 `services/lobby/internal/config/loader_test.go`：

```go
func TestLoadOne_NotFound(t *testing.T) {
	ResetStore()

	pkg := &testConfig{name: "missing_config"}
	Register[int](pkg)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 模拟查询无结果
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("missing_config").
		WillReturnError(sql.ErrNoRows)

	router := newTestRouter(t, db)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	err = loader.LoadOne(context.Background(), "missing_config")
	if err == nil {
		t.Error("LoadOne() should return error for missing config")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
```

- [ ] **Step 5: 更新 TestLoadOne_NotRegistered 测试**

修改 `services/lobby/internal/config/loader_test.go`：

```go
func TestLoadOne_NotRegistered(t *testing.T) {
	ResetStore()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	router := newTestRouter(t, db)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	err = loader.LoadOne(context.Background(), "unregistered_config")
	if err == nil {
		t.Error("LoadOne() should return error for unregistered config")
	}
}
```

- [ ] **Step 6: 更新 TestLoadAll_PartialFailure 测试**

修改 `services/lobby/internal/config/loader_test.go`：

```go
func TestLoadAll_PartialFailure(t *testing.T) {
	ResetStore()

	// 注册两个配置（按字母顺序，bad_config 在前）
	Register[int](&testConfig{name: "bad_config"})
	Register[int](&testConfig{name: "good_config"})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// bad_config 失败（按字母顺序先执行）
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("bad_config").
		WillReturnError(sql.ErrNoRows)

	// good_config 成功
	rows := sqlmock.NewRows([]string{"data", "revision"}).
		AddRow(`[1, 2]`, 1)
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("good_config").
		WillReturnRows(rows)

	router := newTestRouter(t, db)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	err = loader.LoadAll(context.Background())
	// LoadAll 不应该返回错误，即使部分失败
	if err != nil {
		t.Errorf("LoadAll() error = %v, want nil", err)
	}

	// good_config 应该加载成功
	ts := GetStore().GetTypedStore("good_config").(*TypedStore[int])
	if ts.Get() == nil {
		t.Error("good_config should be loaded")
	}

	// bad_config 应该为 nil
	ts2 := GetStore().GetTypedStore("bad_config").(*TypedStore[int])
	if ts2.Get() != nil {
		t.Error("bad_config should not be loaded")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
```

- [ ] **Step 7: 更新 TestLoadOne_InvalidJSON 测试**

修改 `services/lobby/internal/config/loader_test.go`：

```go
func TestLoadOne_InvalidJSON(t *testing.T) {
	ResetStore()

	pkg := &testConfig{name: "invalid_json_config"}
	Register[int](pkg)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 模拟返回无效 JSON
	rows := sqlmock.NewRows([]string{"data", "revision"}).
		AddRow(`not valid json`, 1)
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("invalid_json_config").
		WillReturnRows(rows)

	router := newTestRouter(t, db)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	err = loader.LoadOne(context.Background(), "invalid_json_config")
	if err == nil {
		t.Error("LoadOne() should return error for invalid JSON")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
```

- [ ] **Step 8: 更新 TestLoadAll_EmptyRegistry 测试**

修改 `services/lobby/internal/config/loader_test.go`：

```go
func TestLoadAll_EmptyRegistry(t *testing.T) {
	ResetStore()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	router := newTestRouter(t, db)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	err = loader.LoadAll(context.Background())
	if err != nil {
		t.Errorf("LoadAll() on empty registry error = %v, want nil", err)
	}
}
```

- [ ] **Step 9: 运行 loader 测试验证通过**

Run: `go test ./services/lobby/internal/config/... -run TestLoad -v`
Expected: All tests PASS

- [ ] **Step 10: 提交 ConfigLoader 重构**

```bash
git add services/lobby/internal/config/loader.go services/lobby/internal/config/loader_test.go
git commit -m "$(cat <<'EOF'
refactor(lobby/config): use dsroute.Router in ConfigLoader

- Change ConfigLoader to hold *dsroute.Router instead of *sql.DB
- Add routingKey parameter for connection resolution
- Update all tests to use test Router

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: 重构 ConfigWatcher 使用 dsroute.Router

**Files:**
- Modify: `services/lobby/internal/config/watcher.go`
- Modify: `services/lobby/internal/config/watcher_test.go`

- [ ] **Step 1: 修改 ConfigWatcher 结构体和构造函数**

修改 `services/lobby/internal/config/watcher.go`：

```go
package config

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/redis/go-redis/v9"
)

// ConfigNotifyChannel Redis Pub/Sub 频道
const ConfigNotifyChannel = "kd48:lobby:config:notify"

// ConfigWatcher 订阅 Redis Pub/Sub 实现热更新
type ConfigWatcher struct {
	router     *dsroute.Router
	routingKey string
	loader     *ConfigLoader
	channel    string
}

// NewConfigWatcher 创建订阅器
func NewConfigWatcher(router *dsroute.Router, routingKey string, loader *ConfigLoader, channel string) *ConfigWatcher {
	return &ConfigWatcher{
		router:     router,
		routingKey: routingKey,
		loader:     loader,
		channel:    channel,
	}
}

// Start 启动订阅
func (w *ConfigWatcher) Start(ctx context.Context) {
	// 通过 Router 解析 Redis 连接
	rdb, poolName, err := w.router.ResolveRedis(ctx, w.routingKey)
	if err != nil {
		slog.Error("failed to resolve redis for config watcher", "error", err, "routingKey", w.routingKey)
		return
	}

	// 转换为 *redis.Client（Pub/Sub 需要具体类型）
	client, ok := rdb.(*redis.Client)
	if !ok {
		slog.Error("resolved redis is not a *redis.Client", "pool", poolName)
		return
	}

	pubsub := client.Subscribe(ctx, w.channel)
	defer pubsub.Close()

	// 检查订阅是否成功
	if _, err := pubsub.Receive(ctx); err != nil {
		slog.Error("failed to subscribe to config channel", "channel", w.channel, "error", err)
		return
	}

	slog.Info("config watcher started", "channel", w.channel, "pool", poolName)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			slog.Info("config watcher stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			w.handleMessage(ctx, msg.Payload)
		}
	}
}

// handleMessage 处理热更新消息
func (w *ConfigWatcher) handleMessage(ctx context.Context, payload string) {
	var notify struct {
		ConfigName string `json:"config_name"`
		Revision   int64  `json:"revision"`
	}

	if err := json.Unmarshal([]byte(payload), &notify); err != nil {
		slog.Warn("invalid config notify message", "error", err, "payload", payload)
		return
	}

	if notify.ConfigName == "" {
		slog.Warn("config notify message missing config_name", "payload", payload)
		return
	}

	slog.Info("received config update",
		"config_name", notify.ConfigName,
		"revision", notify.Revision)

	if err := w.loader.LoadOne(ctx, notify.ConfigName); err != nil {
		slog.Error("failed to reload config",
			"config_name", notify.ConfigName,
			"error", err)
	}
}
```

- [ ] **Step 2: 更新 watcher_test.go 的 import 和测试辅助函数**

修改 `services/lobby/internal/config/watcher_test.go`，更新 import 并添加测试辅助函数：

```go
package config

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const testWatcherRoutingKey = "lobby:config-notify"

// newTestWatcherRouter 创建测试用 Router，包含 mock 数据库和 miniredis
func newTestWatcherRouter(t *testing.T, db *sql.DB, rdb *redis.Client) *dsroute.Router {
	mysqlPools := map[string]*sql.DB{"default": db}
	redisPools := map[string]redis.UniversalClient{"default": rdb}

	mysqlRoutes := []dsroute.RouteRule{
		{Prefix: testRoutingKey, Pool: "default"},
	}
	redisRoutes := []dsroute.RouteRule{
		{Prefix: testWatcherRoutingKey, Pool: "default"},
	}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, mysqlRoutes, redisRoutes)
	if err != nil {
		t.Fatalf("failed to create test router: %v", err)
	}
	return router
}
```

- [ ] **Step 3: 更新 TestWatcher_ValidMessage 测试**

修改 `services/lobby/internal/config/watcher_test.go`：

```go
func TestWatcher_ValidMessage(t *testing.T) {
	ResetStore()

	// 注册配置
	Register[int](&testConfig{name: "test_config"})

	// 创建 miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// 创建 mock 数据库
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 模拟 MySQL 返回
	rows := sqlmock.NewRows([]string{"data", "revision"}).
		AddRow(`[10, 20]`, 100)
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("test_config").
		WillReturnRows(rows)

	// 创建测试 Router
	router := newTestWatcherRouter(t, db, rdb)

	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	watcher := NewConfigWatcher(router, testWatcherRoutingKey, loader, ConfigNotifyChannel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watcher.Start(ctx)

	// 等待订阅启动
	time.Sleep(100 * time.Millisecond)

	// 发布消息
	err = rdb.Publish(ctx, ConfigNotifyChannel, `{"config_name":"test_config","revision":100}`).Err()
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	// 验证配置已更新
	ts := GetStore().GetTypedStore("test_config").(*TypedStore[int])
	snap := ts.Get()
	if snap == nil {
		t.Fatal("snapshot is nil")
	}
	if snap.Revision != 100 {
		t.Errorf("Revision = %d, want 100", snap.Revision)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
```

- [ ] **Step 4: 更新 TestWatcher_InvalidJSON 测试**

修改 `services/lobby/internal/config/watcher_test.go`：

```go
func TestWatcher_InvalidJSON(t *testing.T) {
	ResetStore()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	router := newTestWatcherRouter(t, db, rdb)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	watcher := NewConfigWatcher(router, testWatcherRoutingKey, loader, ConfigNotifyChannel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watcher.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// 发布无效 JSON
	err = rdb.Publish(ctx, ConfigNotifyChannel, `not valid json`).Err()
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	// 测试通过即表示没有 panic，无效消息被忽略
}
```

- [ ] **Step 5: 更新 TestWatcher_MissingField 测试**

修改 `services/lobby/internal/config/watcher_test.go`：

```go
func TestWatcher_MissingField(t *testing.T) {
	ResetStore()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	router := newTestWatcherRouter(t, db, rdb)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	watcher := NewConfigWatcher(router, testWatcherRoutingKey, loader, ConfigNotifyChannel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go watcher.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	// 发布缺少 config_name 的消息
	err = rdb.Publish(ctx, ConfigNotifyChannel, `{"revision":100}`).Err()
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	// 测试通过即表示没有 panic，缺少字段的消息被忽略
}
```

- [ ] **Step 6: 更新 TestWatcher_ContextCancel 测试**

修改 `services/lobby/internal/config/watcher_test.go`：

```go
func TestWatcher_ContextCancel(t *testing.T) {
	ResetStore()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to create miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	router := newTestWatcherRouter(t, db, rdb)
	loader := NewConfigLoader(router, testRoutingKey, GetStore())
	watcher := NewConfigWatcher(router, testWatcherRoutingKey, loader, ConfigNotifyChannel)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		watcher.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	// 取消 context
	cancel()

	select {
	case <-done:
		// 正常退出
	case <-time.After(2 * time.Second):
		t.Error("watcher did not stop after context cancel")
	}
}
```

- [ ] **Step 7: 运行 watcher 测试验证通过**

Run: `go test ./services/lobby/internal/config/... -run TestWatcher -v`
Expected: All tests PASS

- [ ] **Step 8: 提交 ConfigWatcher 重构**

```bash
git add services/lobby/internal/config/watcher.go services/lobby/internal/config/watcher_test.go
git commit -m "$(cat <<'EOF'
refactor(lobby/config): use dsroute.Router in ConfigWatcher

- Change ConfigWatcher to hold *dsroute.Router instead of *redis.Client
- Add routingKey parameter for connection resolution
- Update all tests to use test Router

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: 重构 lobbyService 使用 dsroute.Router

**Files:**
- Modify: `services/lobby/cmd/lobby/server.go`
- Modify: `services/lobby/cmd/lobby/server_test.go`

- [ ] **Step 1: 修改 lobbyService 结构体**

修改 `services/lobby/cmd/lobby/server.go`：

```go
package main

import (
	"context"
	"log/slog"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/dsroute"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// lobbyService 大厅服务实现
type lobbyService struct {
	lobbyv1.UnimplementedLobbyServiceServer
	router *dsroute.Router
}

// NewLobbyService 创建大厅服务实例
func NewLobbyService(router *dsroute.Router) *lobbyService {
	return &lobbyService{
		router: router,
	}
}

// Ping 健康检查/心跳接口
func (s *lobbyService) Ping(ctx context.Context, req *lobbyv1.PingRequest) (*lobbyv1.PingReply, error) {
	slog.InfoContext(ctx, "Received Ping request", "client_hint", req.GetClientHint())

	// 当前返回占位值，后续 Task 实现配置加载后返回实际的 config_revision
	return &lobbyv1.PingReply{
		Pong:           "pong",
		ConfigRevision: 0,
	}, nil
}
```

- [ ] **Step 2: 更新 server_test.go**

修改 `services/lobby/cmd/lobby/server_test.go`：

```go
package main

import (
	"context"
	"database/sql"
	"net"
	"testing"

	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// newTestLobbyRouter 创建测试用 Router
func newTestLobbyRouter(t *testing.T) *dsroute.Router {
	mysqlPools := map[string]*sql.DB{}
	redisPools := map[string]redis.UniversalClient{}

	router, err := dsroute.NewRouter(mysqlPools, redisPools, nil, nil)
	if err != nil {
		t.Fatalf("failed to create test router: %v", err)
	}
	return router
}

func setupTestServer(t *testing.T) (lobbyv1.LobbyServiceClient, func()) {
	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer()

	// 创建测试 Router
	router := newTestLobbyRouter(t)

	lobbySvc := NewLobbyService(router)
	lobbyv1.RegisterLobbyServiceServer(server, lobbySvc)

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("Server exited with error: %v", err)
		}
	}()

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer),
		grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client := lobbyv1.NewLobbyServiceClient(conn)

	cleanup := func() {
		conn.Close()
		server.Stop()
		listener.Close()
	}

	return client, cleanup
}

func TestPing(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	req := &lobbyv1.PingRequest{
		ClientHint: stringPtr("test-client"),
	}

	resp, err := client.Ping(ctx, req)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// 验证 pong 返回值
	if resp.GetPong() != "pong" {
		t.Errorf("Expected pong to be 'pong', got %q", resp.GetPong())
	}

	// 验证 config_revision 为 0（占位值）
	if resp.GetConfigRevision() != 0 {
		t.Errorf("Expected config_revision to be 0, got %d", resp.GetConfigRevision())
	}
}

func TestPingWithoutClientHint(t *testing.T) {
	client, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	req := &lobbyv1.PingRequest{} // 不使用 client_hint

	resp, err := client.Ping(ctx, req)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.GetPong() != "pong" {
		t.Errorf("Expected pong to be 'pong', got %q", resp.GetPong())
	}

	if resp.GetConfigRevision() != 0 {
		t.Errorf("Expected config_revision to be 0, got %d", resp.GetConfigRevision())
	}
}

func stringPtr(s string) *string {
	return &s
}
```

- [ ] **Step 3: 运行 server 测试验证通过**

Run: `go test ./services/lobby/cmd/lobby/... -v`
Expected: All tests PASS

- [ ] **Step 4: 提交 lobbyService 重构**

```bash
git add services/lobby/cmd/lobby/server.go services/lobby/cmd/lobby/server_test.go
git commit -m "$(cat <<'EOF'
refactor(lobby): use dsroute.Router in lobbyService

- Remove mysqlPools and redisPools fields from lobbyService
- Inject *dsroute.Router instead
- Remove unused getMySQLDB and getRedis methods
- Update tests to use test Router

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 重构 main.go 集成 dsroute.Router

**Files:**
- Modify: `services/lobby/cmd/lobby/main.go`

- [ ] **Step 1: 更新 main.go 导入和初始化逻辑**

修改 `services/lobby/cmd/lobby/main.go`：

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	lobbyv1 "github.com/CBookShu/kd48/api/proto/lobby/v1"
	"github.com/CBookShu/kd48/pkg/conf"
	"github.com/CBookShu/kd48/pkg/dsroute"
	"github.com/CBookShu/kd48/pkg/logzap"
	"github.com/CBookShu/kd48/pkg/otelkit"
	"github.com/CBookShu/kd48/pkg/registry"
	"github.com/CBookShu/kd48/services/lobby/internal/config"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

// Routing keys for lobby service
const (
	routingKeyConfigData   = "lobby:config-data"
	routingKeyConfigNotify = "lobby:config-notify"
)

func main() {
	c, err := conf.Load("./config.yaml")
	if err != nil {
		panic(err)
	}

	logPath := filepath.Join(c.Log.FilePath, "lobby-service.log")
	handler := logzap.New(c.Log.Level, logPath)
	slog.SetDefault(slog.New(handler))

	shutdown, err := otelkit.InitTracer(c.Server.Name + "-lobby-service")
	if err != nil {
		panic(err)
	}
	defer shutdown(context.Background())

	dsCfg := c.GetDataSourcesOrSynthesize().ToDSRouteConfig()

	// 初始化 MySQL 连接
	mysqlPools := make(map[string]*sql.DB)
	for name, spec := range dsCfg.MySQLPools {
		db, err := sql.Open("mysql", spec.DSN)
		if err != nil {
			panic(fmt.Errorf("failed to open mysql pool %q: %w", name, err))
		}

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

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := db.PingContext(ctx); err != nil {
			cancel()
			panic(fmt.Errorf("failed to ping mysql pool %q: %w", name, err))
		}
		cancel()

		mysqlPools[name] = db
		slog.Info("MySQL pool connected", "name", name)
	}
	defer func() {
		for _, db := range mysqlPools {
			db.Close()
		}
	}()

	// 初始化 Redis 连接
	redisPools := make(map[string]redis.UniversalClient)
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
		slog.Info("Redis pool connected", "name", name, "addr", spec.Addr)
	}
	defer func() {
		for _, rdb := range redisPools {
			rdb.Close()
		}
	}()

	// 创建 etcd 客户端
	etcdCli, err := registry.NewClient(c.Etcd)
	if err != nil {
		panic(err)
	}
	defer etcdCli.Close()

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

	// 初始化配置加载器（使用 Router）
	configLoader := config.NewConfigLoader(router, routingKeyConfigData, config.GetStore())

	// 启动时加载所有配置
	if err := configLoader.LoadAll(context.Background()); err != nil {
		slog.Error("failed to load configs", "error", err)
		// 不 panic，允许部分配置加载失败
	}
	slog.Info("configs loaded")

	// 启动配置热更新订阅（使用 Router）
	configWatcher := config.NewConfigWatcher(router, routingKeyConfigNotify, configLoader, config.ConfigNotifyChannel)
	go configWatcher.Start(context.Background())

	// 启动 gRPC Server
	// 从配置读取端口，默认 9001
	port := c.LobbyService.Port
	if port == 0 {
		port = 9001
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	lobbySvc := NewLobbyService(router)
	lobbyv1.RegisterLobbyServiceServer(s, lobbySvc)
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(lobbySvc))

	go func() {
		slog.Info("Lobby Service gRPC server listening", "port", port)
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()

	// 注册到 Etcd
	// 广播地址从环境变量 ADVERTISE_ADDR 读取
	// - K8s: 通过 Downward API 注入 POD_IP
	// - Docker Compose: 使用服务名
	// - 本地开发: 留空则使用 localhost
	advertiseAddr := os.Getenv("ADVERTISE_ADDR")
	if advertiseAddr == "" {
		if c.Server.Env == "dev" {
			advertiseAddr = "localhost"
			slog.Warn("ADVERTISE_ADDR not set, falling back to localhost (dev mode)")
		} else {
			slog.Error("ADVERTISE_ADDR environment variable is required in non-dev environment", "env", c.Server.Env)
			os.Exit(1)
		}
	}
	localAddr := fmt.Sprintf("%s:%d", advertiseAddr, port)
	serviceName := "kd48/lobby-service"

	if err := registry.RegisterService(etcdCli, serviceName, localAddr); err != nil {
		panic(err)
	}
	slog.Info("Lobby Service registered to Etcd", "name", serviceName, "addr", localAddr)

	// 阻塞等待退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down Lobby Service...")
	s.GracefulStop()
}
```

- [ ] **Step 2: 运行构建验证**

Run: `go build ./services/lobby/cmd/lobby`
Expected: Build succeeds with no errors

- [ ] **Step 3: 运行全部测试验证**

Run: `go test ./services/lobby/... -v`
Expected: All tests PASS

- [ ] **Step 4: 提交 main.go 重构**

```bash
git add services/lobby/cmd/lobby/main.go
git commit -m "$(cat <<'EOF'
refactor(lobby): integrate dsroute.Router in main.go

- Create Router from etcd routing config
- Pass Router to ConfigLoader, ConfigWatcher, and lobbyService
- Add routing key constants for config-data and config-notify
- Start route watcher for hot updates

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 验收测试

- [ ] **Step 1: 运行全量测试**

Run: `go test ./...`
Expected: All tests PASS

- [ ] **Step 2: 运行构建验证**

Run: `go build ./...`
Expected: Build succeeds

- [ ] **Step 3: 验证无违规代码**

Run: `grep -r "mysqlPools\[" services/lobby/`
Expected: No matches

Run: `grep -r "redisPools\[" services/lobby/`
Expected: No matches

- [ ] **Step 4: 最终提交（如果有遗漏）**

```bash
git status
# 如果有未提交的更改，提交它们
```

---

## Routing Key 配置提醒

部署前需在 etcd 中添加以下路由规则：

```json
{
  "mysql_routes": [
    {"prefix": "lobby:config-data", "pool": "default"}
  ],
  "redis_routes": [
    {"prefix": "lobby:config-notify", "pool": "default"}
  ]
}
```
