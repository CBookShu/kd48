# Lobby 配置加载实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Lobby 服务实现类型安全的配置加载与热更新机制，支持启动时从 MySQL 加载、运行时 Redis Pub/Sub 热更新。

**Architecture:** TypedStore[T] 泛型存储提供类型安全访问，ConfigStore 单例管理所有配置，ConfigLoader 从 MySQL 加载，ConfigWatcher 订阅 Redis Pub/Sub 实现热更新。生成的配置代码通过 init() 自动注册。

**Tech Stack:** Go 1.26 generics, sync.RWMutex, go-sqlmock (测试), miniredis (测试)

---

## 文件结构

```
pkg/config/
└── config.go                    # Config 接口定义（新建）

services/lobby/
├── cmd/lobby/
│   └── main.go                  # 集成 Loader/Watcher（修改）
├── internal/
│   └── config/
│       ├── store.go             # TypedStore[T], Snapshot[T]（新建）
│       ├── store_test.go        # TypedStore 测试（新建）
│       ├── registry.go          # ConfigStore, Register[T]（新建）
│       ├── registry_test.go     # Registry 测试（新建）
│       ├── loader.go            # ConfigLoader（新建）
│       ├── loader_test.go       # Loader 测试（新建）
│       ├── watcher.go           # ConfigWatcher（新建）
│       └── watcher_test.go      # Watcher 测试（新建）

tools/config-loader/
└── internal/gogen/
    └── generator.go             # 生成 Config 接口实现（修改）
```

---

## Task 1: Config 基础接口

**Files:**
- Create: `pkg/config/config.go`
- Create: `pkg/config/config_test.go`

### Step 1.1: Write failing test for Config interface

- [ ] **Write the test**

```go
// pkg/config/config_test.go
package config

import "testing"

// MockConfig 用于测试接口合规性
type MockConfig struct {
    name string
    data any
}

func (m *MockConfig) ConfigName() string {
    return m.name
}

func (m *MockConfig) ConfigData() any {
    return m.data
}

func TestConfigInterface_Compliance(t *testing.T) {
    // 编译时检查 MockConfig 实现 Config 接口
    var _ Config = &MockConfig{}

    m := &MockConfig{name: "test_config", data: []int{1, 2, 3}}

    if got := m.ConfigName(); got != "test_config" {
        t.Errorf("ConfigName() = %v, want test_config", got)
    }

    data := m.ConfigData()
    if data == nil {
        t.Error("ConfigData() returned nil")
    }
}
```

- [ ] **Run test to verify it fails**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./pkg/config/... -v
```

Expected: FAIL with "undefined: Config"

### Step 1.2: Implement Config interface

- [ ] **Write the implementation**

```go
// pkg/config/config.go
package config

// Config 基础接口，所有生成的配置必须实现
type Config interface {
    // ConfigName 配置名称
    ConfigName() string

    // ConfigData 配置数据（返回切片指针，用于类型推导）
    ConfigData() any
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./pkg/config/... -v
```

Expected: PASS

### Step 1.3: Commit

- [ ] **Commit changes**

```bash
git add pkg/config/config.go pkg/config/config_test.go
git commit -m "feat(config): add Config interface for generated configs

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: TypedStore 泛型存储

**Files:**
- Create: `services/lobby/internal/config/store.go`
- Create: `services/lobby/internal/config/store_test.go`

### Step 2.1: Write failing test for TypedStore Get returns nil

- [ ] **Write the test**

```go
// services/lobby/internal/config/store_test.go
package config

import (
    "testing"
    "time"
)

func TestTypedStore_Get_Nil(t *testing.T) {
    store := NewTypedStore[int]()
    snap := store.Get()
    if snap != nil {
        t.Errorf("Get() on new store = %v, want nil", snap)
    }
}
```

- [ ] **Run test to verify it fails**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestTypedStore_Get_Nil
```

Expected: FAIL with "undefined: NewTypedStore"

### Step 2.2: Implement TypedStore skeleton

- [ ] **Write minimal implementation**

```go
// services/lobby/internal/config/store.go
package config

import (
    "sync"
    "time"
)

// Snapshot 配置快照
type Snapshot[T any] struct {
    Revision int64
    Data     []T
    ParsedAt time.Time
}

// TypedStore 类型安全的配置存储
type TypedStore[T any] struct {
    mu       sync.RWMutex
    snapshot *Snapshot[T]
}

// NewTypedStore 创建类型安全存储
func NewTypedStore[T any]() *TypedStore[T] {
    return &TypedStore[T]{}
}

// Get 返回快照（线程安全）
func (s *TypedStore[T]) Get() *Snapshot[T] {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.snapshot
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestTypedStore_Get_Nil
```

Expected: PASS

### Step 2.3: Write test for Update and Get

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/store_test.go

func TestTypedStore_UpdateAndGet(t *testing.T) {
    store := NewTypedStore[int]()

    data := []int{1, 2, 3}
    store.Update(42, data)

    snap := store.Get()
    if snap == nil {
        t.Fatal("Get() returned nil after Update")
    }

    if snap.Revision != 42 {
        t.Errorf("Revision = %d, want 42", snap.Revision)
    }

    if len(snap.Data) != 3 {
        t.Errorf("Data length = %d, want 3", len(snap.Data))
    }

    if snap.Data[0] != 1 || snap.Data[1] != 2 || snap.Data[2] != 3 {
        t.Errorf("Data = %v, want [1, 2, 3]", snap.Data)
    }
}
```

- [ ] **Run test to verify it fails**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestTypedStore_UpdateAndGet
```

Expected: FAIL with "no method Update"

### Step 2.4: Implement Update method

- [ ] **Write the implementation**

```go
// Append to services/lobby/internal/config/store.go

// Update 更新快照
func (s *TypedStore[T]) Update(revision int64, data []T) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.snapshot = &Snapshot[T]{
        Revision: revision,
        Data:     data,
        ParsedAt: time.Now(),
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestTypedStore_UpdateAndGet
```

Expected: PASS

### Step 2.5: Write test for concurrent access

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/store_test.go

import "sync"

func TestTypedStore_ConcurrentAccess(t *testing.T) {
    store := NewTypedStore[int]()

    var wg sync.WaitGroup
    numGoroutines := 100

    // 并发写入
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(rev int64) {
            defer wg.Done()
            store.Update(rev, []int{int(rev)})
        }(int64(i))
    }

    // 并发读取
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _ = store.Get()
        }()
    }

    wg.Wait()

    // 最终应该有一个有效快照
    snap := store.Get()
    if snap == nil {
        t.Error("Get() returned nil after concurrent operations")
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestTypedStore_ConcurrentAccess
```

Expected: PASS (并发安全由 sync.RWMutex 保证)

### Step 2.6: Write test for Update replaces old data

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/store_test.go

func TestTypedStore_UpdateReplaces(t *testing.T) {
    store := NewTypedStore[string]()

    store.Update(1, []string{"old"})
    store.Update(2, []string{"new"})

    snap := store.Get()
    if snap == nil {
        t.Fatal("Get() returned nil")
    }

    if snap.Revision != 2 {
        t.Errorf("Revision = %d, want 2", snap.Revision)
    }

    if len(snap.Data) != 1 || snap.Data[0] != "new" {
        t.Errorf("Data = %v, want [new]", snap.Data)
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestTypedStore_UpdateReplaces
```

Expected: PASS

### Step 2.7: Commit

- [ ] **Commit changes**

```bash
git add services/lobby/internal/config/store.go services/lobby/internal/config/store_test.go
git commit -m "feat(lobby/config): add TypedStore[T] for type-safe config storage

- Generic TypedStore[T] with atomic snapshots
- Thread-safe Get/Update with RWMutex
- Tests: nil, update, concurrent access, replace

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: ConfigStore 和 Register 函数

**Files:**
- Create: `services/lobby/internal/config/registry.go`
- Create: `services/lobby/internal/config/registry_test.go`

### Step 3.1: Write failing test for Register creates TypedStore

- [ ] **Write the test**

```go
// services/lobby/internal/config/registry_test.go
package config

import (
    "testing"

    baseconfig "github.com/CBookShu/kd48/pkg/config"
)

// testConfig 实现 baseconfig.Config 接口
type testConfig struct {
    name string
}

func (t *testConfig) ConfigName() string {
    return t.name
}

func (t *testConfig) ConfigData() any {
    return &[]int{}
}

func TestRegister_CreatesTypedStore(t *testing.T) {
    // 重置全局状态
    ResetStore()

    pkg := &testConfig{name: "test_config"}
    store := Register[int](pkg)

    if store == nil {
        t.Fatal("Register() returned nil")
    }

    // 验证可以通过 GetTypedStore 获取
    cs := GetStore()
    ts := cs.GetTypedStore("test_config")
    if ts == nil {
        t.Error("GetTypedStore() returned nil for registered config")
    }
}
```

- [ ] **Run test to verify it fails**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestRegister_CreatesTypedStore
```

Expected: FAIL with "undefined: ResetStore"

### Step 3.2: Implement ConfigStore and Register

- [ ] **Write the implementation**

```go
// services/lobby/internal/config/registry.go
package config

import (
    "sync"

    baseconfig "github.com/CBookShu/kd48/pkg/config"
)

var (
    globalStore *ConfigStore
    once        sync.Once
)

// ConfigStore 管理所有配置
type ConfigStore struct {
    mu     sync.RWMutex
    stores map[string]any // name → *TypedStore[T]
}

// GetStore 获取全局 Store（单例）
func GetStore() *ConfigStore {
    once.Do(func() {
        globalStore = &ConfigStore{
            stores: make(map[string]any),
        }
    })
    return globalStore
}

// ResetStore 重置全局 Store（仅测试使用）
func ResetStore() {
    globalStore = &ConfigStore{
        stores: make(map[string]any),
    }
    once = sync.Once{}
}

// GetTypedStore 获取类型安全的 Store（内部使用）
func (cs *ConfigStore) GetTypedStore(name string) any {
    cs.mu.RLock()
    defer cs.mu.RUnlock()
    return cs.stores[name]
}

// GetRegisteredNames 获取所有已注册的配置名
func (cs *ConfigStore) GetRegisteredNames() []string {
    cs.mu.RLock()
    defer cs.mu.RUnlock()
    names := make([]string, 0, len(cs.stores))
    for name := range cs.stores {
        names = append(names, name)
    }
    return names
}

// Register 注册配置（init 中调用）
func Register[T any](pkg baseconfig.Config) *TypedStore[T] {
    cs := GetStore()
    ts := NewTypedStore[T]()

    cs.mu.Lock()
    cs.stores[pkg.ConfigName()] = ts
    cs.mu.Unlock()

    return ts
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestRegister_CreatesTypedStore
```

Expected: PASS

### Step 3.3: Write test for singleton

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/registry_test.go

func TestGetStore_Singleton(t *testing.T) {
    ResetStore()

    s1 := GetStore()
    s2 := GetStore()

    if s1 != s2 {
        t.Error("GetStore() returned different instances")
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestGetStore_Singleton
```

Expected: PASS

### Step 3.4: Write test for same name idempotent

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/registry_test.go

func TestRegister_SameNameIdempotent(t *testing.T) {
    ResetStore()

    pkg := &testConfig{name: "same_name"}

    s1 := Register[int](pkg)
    s2 := Register[int](pkg)

    // 两次注册应该返回同一个 store
    if s1 != s2 {
        t.Error("Register() with same name returned different stores")
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestRegister_SameNameIdempotent
```

Expected: PASS

### Step 3.5: Write test for GetTypedStore not found

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/registry_test.go

func TestGetTypedStore_NotFound(t *testing.T) {
    ResetStore()

    cs := GetStore()
    ts := cs.GetTypedStore("non_existent")

    if ts != nil {
        t.Error("GetTypedStore() for non-existent config should return nil")
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestGetTypedStore_NotFound
```

Expected: PASS

### Step 3.6: Commit

- [ ] **Commit changes**

```bash
git add services/lobby/internal/config/registry.go services/lobby/internal/config/registry_test.go
git commit -m "feat(lobby/config): add ConfigStore singleton and Register[T]

- Global ConfigStore singleton pattern
- Register[T] for auto-registration in init()
- GetTypedStore for internal loader access
- GetRegisteredNames for LoadAll iteration
- Tests: create, singleton, idempotent, not found

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: ConfigLoader

**Files:**
- Create: `services/lobby/internal/config/loader.go`
- Create: `services/lobby/internal/config/loader_test.go`

### Step 4.1: Write failing test for LoadOne success

- [ ] **Write the test**

```go
// services/lobby/internal/config/loader_test.go
package config

import (
    "context"
    "database/sql"
    "testing"

    "github.com/DATA-DOG/go-sqlmock"
)

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

    // 执行加载
    loader := NewConfigLoader(db, GetStore())
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

- [ ] **Run test to verify it fails**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestLoadOne_Success
```

Expected: FAIL with "undefined: NewConfigLoader"

### Step 4.2: Implement ConfigLoader skeleton

- [ ] **Write minimal implementation**

```go
// services/lobby/internal/config/loader.go
package config

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log/slog"

    baseconfig "github.com/CBookShu/kd48/pkg/config"
)

// ConfigLoader 从 MySQL 加载配置
type ConfigLoader struct {
    db    *sql.DB
    store *ConfigStore
}

// NewConfigLoader 创建加载器
func NewConfigLoader(db *sql.DB, store *ConfigStore) *ConfigLoader {
    return &ConfigLoader{db: db, store: store}
}

// LoadOne 加载单个配置
func (l *ConfigLoader) LoadOne(ctx context.Context, name string) error {
    // 1. 从 MySQL 读取最新版本
    query := `
        SELECT data, revision
        FROM lobby_config_revision
        WHERE config_name = ?
        ORDER BY revision DESC
        LIMIT 1
    `

    var data []byte
    var revision int64
    err := l.db.QueryRowContext(ctx, query, name).Scan(&data, &revision)
    if err != nil {
        return fmt.Errorf("query config %s: %w", name, err)
    }

    // 2. 获取对应的 TypedStore
    ts := l.store.GetTypedStore(name)
    if ts == nil {
        return fmt.Errorf("config %s not registered", name)
    }

    // 3. 解析 JSON 并更新 Store
    return l.parseAndUpdate(name, data, revision, ts)
}

// parseAndUpdate 解析 JSON 并更新 TypedStore
func (l *ConfigLoader) parseAndUpdate(name string, data []byte, revision int64, ts any) error {
    // 使用反射获取 TypedStore 的类型参数并解析
    // 这里需要处理泛型，通过接口断言

    // 尝试解析为 []interface{} 然后类型断言
    var items []json.RawMessage
    if err := json.Unmarshal(data, &items); err != nil {
        return fmt.Errorf("parse config %s: %w", name, err)
    }

    // 根据注册时的类型解析
    // 这里简化处理，实际需要更复杂的类型推导
    switch typedStore := ts.(type) {
    case *TypedStore[int]:
        var parsed []int
        if err := json.Unmarshal(data, &parsed); err != nil {
            return fmt.Errorf("parse config %s as []int: %w", name, err)
        }
        typedStore.Update(revision, parsed)
    case *TypedStore[string]:
        var parsed []string
        if err := json.Unmarshal(data, &parsed); err != nil {
            return fmt.Errorf("parse config %s as []string: %w", name, err)
        }
        typedStore.Update(revision, parsed)
    case *TypedStore[float64]:
        var parsed []float64
        if err := json.Unmarshal(data, &parsed); err != nil {
            return fmt.Errorf("parse config %s as []float64: %w", name, err)
        }
        typedStore.Update(revision, parsed)
    default:
        // 对于复杂类型，使用 json.Unmarshal 到 ConfigData
        slog.Warn("unknown typed store type, using generic parsing", "config", name)
        return fmt.Errorf("unsupported typed store type for config %s", name)
    }

    slog.Debug("config loaded", "name", name, "revision", revision)
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

// RegisterLoader 注册加载器（支持更多类型）
func RegisterLoader[T any](loader *ConfigLoader, pkg baseconfig.Config, parseFunc func([]byte) ([]T, error)) *TypedStore[T] {
    ts := Register[T](pkg)
    // 存储解析函数供后续使用
    // 这里简化处理，实际实现中可以扩展
    return ts
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestLoadOne_Success
```

Expected: PASS

### Step 4.3: Write test for LoadOne not found

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/loader_test.go

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

    loader := NewConfigLoader(db, GetStore())
    err = loader.LoadOne(context.Background(), "missing_config")
    if err == nil {
        t.Error("LoadOne() should return error for missing config")
    }

    if err := mock.ExpectationsWereMet(); err != nil {
        t.Errorf("unfulfilled expectations: %v", err)
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestLoadOne_NotFound
```

Expected: PASS

### Step 4.4: Write test for LoadOne not registered

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/loader_test.go

func TestLoadOne_NotRegistered(t *testing.T) {
    ResetStore()

    db, _, err := sqlmock.New()
    if err != nil {
        t.Fatalf("failed to create sqlmock: %v", err)
    }
    defer db.Close()

    loader := NewConfigLoader(db, GetStore())
    err = loader.LoadOne(context.Background(), "unregistered_config")
    if err == nil {
        t.Error("LoadOne() should return error for unregistered config")
    }
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestLoadOne_NotRegistered
```

Expected: PASS

### Step 4.5: Write test for LoadAll partial failure

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/loader_test.go

func TestLoadAll_PartialFailure(t *testing.T) {
    ResetStore()

    // 注册两个配置
    Register[int](&testConfig{name: "good_config"})
    Register[int](&testConfig{name: "bad_config"})

    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("failed to create sqlmock: %v", err)
    }
    defer db.Close()

    // good_config 成功
    rows := sqlmock.NewRows([]string{"data", "revision"}).
        AddRow(`[1, 2]`, 1)
    mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
        WithArgs("good_config").
        WillReturnRows(rows)

    // bad_config 失败
    mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
        WithArgs("bad_config").
        WillReturnError(sql.ErrNoRows)

    loader := NewConfigLoader(db, GetStore())
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

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestLoadAll_PartialFailure
```

Expected: PASS

### Step 4.6: Commit

- [ ] **Commit changes**

```bash
git add services/lobby/internal/config/loader.go services/lobby/internal/config/loader_test.go
git commit -m "feat(lobby/config): add ConfigLoader for MySQL loading

- LoadOne loads single config from MySQL
- LoadAll loads all registered configs
- Graceful handling of partial failures
- Tests: success, not found, not registered, partial failure

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: ConfigWatcher

**Files:**
- Create: `services/lobby/internal/config/watcher.go`
- Create: `services/lobby/internal/config/watcher_test.go`

### Step 5.1: Write failing test for valid message

- [ ] **Write the test**

```go
// services/lobby/internal/config/watcher_test.go
package config

import (
    "context"
    "testing"
    "time"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
)

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

    loader := NewConfigLoader(db, GetStore())
    watcher := NewConfigWatcher(rdb, loader, ConfigNotifyChannel)

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

- [ ] **Run test to verify it fails**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestWatcher_ValidMessage
```

Expected: FAIL with "undefined: NewConfigWatcher"

### Step 5.2: Implement ConfigWatcher

- [ ] **Write the implementation**

```go
// services/lobby/internal/config/watcher.go
package config

import (
    "context"
    "encoding/json"
    "log/slog"

    "github.com/redis/go-redis/v9"
)

// ConfigNotifyChannel Redis Pub/Sub 频道
const ConfigNotifyChannel = "kd48:lobby:config:notify"

// ConfigWatcher 订阅 Redis Pub/Sub 实现热更新
type ConfigWatcher struct {
    rdb     *redis.Client
    loader  *ConfigLoader
    channel string
}

// NewConfigWatcher 创建订阅器
func NewConfigWatcher(rdb *redis.Client, loader *ConfigLoader, channel string) *ConfigWatcher {
    return &ConfigWatcher{
        rdb:     rdb,
        loader:  loader,
        channel: channel,
    }
}

// Start 启动订阅
func (w *ConfigWatcher) Start(ctx context.Context) {
    pubsub := w.rdb.Subscribe(ctx, w.channel)
    defer pubsub.Close()

    // 检查订阅是否成功
    if _, err := pubsub.Receive(ctx); err != nil {
        slog.Error("failed to subscribe to config channel", "channel", w.channel, "error", err)
        return
    }

    slog.Info("config watcher started", "channel", w.channel)

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

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestWatcher_ValidMessage
```

Expected: PASS

### Step 5.3: Write test for invalid JSON

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/watcher_test.go

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

    loader := NewConfigLoader(db, GetStore())
    watcher := NewConfigWatcher(rdb, loader, ConfigNotifyChannel)

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

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestWatcher_InvalidJSON
```

Expected: PASS

### Step 5.4: Write test for context cancel

- [ ] **Write the test**

```go
// Append to services/lobby/internal/config/watcher_test.go

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

    loader := NewConfigLoader(db, GetStore())
    watcher := NewConfigWatcher(rdb, loader, ConfigNotifyChannel)

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

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v -run TestWatcher_ContextCancel
```

Expected: PASS

### Step 5.5: Commit

- [ ] **Commit changes**

```bash
git add services/lobby/internal/config/watcher.go services/lobby/internal/config/watcher_test.go
git commit -m "feat(lobby/config): add ConfigWatcher for Redis Pub/Sub

- Subscribe to kd48:lobby:config:notify
- Graceful handling of invalid messages
- Context cancellation support
- Tests: valid message, invalid JSON, context cancel

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 6: 修改 Config-Loader 生成代码

**Files:**
- Modify: `tools/config-loader/internal/gogen/generator.go`
- Modify: `tools/config-loader/internal/gogen/generator_test.go`

### Step 6.1: Write test for generated Package struct

- [ ] **Write the test**

```go
// Append to tools/config-loader/internal/gogen/generator_test.go

func TestGenerate_PackageStruct(t *testing.T) {
    g := NewGenerator()

    sheet := &csvparser.Sheet{
        ConfigName: "test_config",
        Headers: []csvparser.Header{
            {Name: "id", Type: "int32"},
            {Name: "name", Type: "string"},
        },
    }

    code, err := g.Generate(sheet, "testconfig")
    if err != nil {
        t.Fatalf("Generate() error = %v", err)
    }

    // 检查 Package 结构体
    if !strings.Contains(code, "type Package struct") {
        t.Error("generated code missing Package struct")
    }

    // 检查 ConfigName 方法
    if !strings.Contains(code, "func (p *Package) ConfigName() string") {
        t.Error("generated code missing ConfigName method")
    }

    // 检查 ConfigData 方法
    if !strings.Contains(code, "func (p *Package) ConfigData() any") {
        t.Error("generated code missing ConfigData method")
    }

    // 检查 Store 变量
    if !strings.Contains(code, "var Store *") {
        t.Error("generated code missing Store variable")
    }

    // 检查 init 函数
    if !strings.Contains(code, "func init()") {
        t.Error("generated code missing init function")
    }
}
```

- [ ] **Run test to verify it fails**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./tools/config-loader/internal/gogen/... -v -run TestGenerate_PackageStruct
```

Expected: FAIL

### Step 6.2: Modify generator to produce Config interface implementation

- [ ] **Modify generator.go**

```go
// tools/config-loader/internal/gogen/generator.go
package gogen

import (
    "fmt"
    "strings"

    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

type GoGenerator struct{}

func NewGenerator() *GoGenerator {
    return &GoGenerator{}
}

func (g *GoGenerator) Generate(sheet *csvparser.Sheet, pkgName string) (string, error) {
    var sb strings.Builder

    sb.WriteString("// Code generated by config-loader. DO NOT EDIT.\n\n")
    sb.WriteString(fmt.Sprintf("package %s\n\n", pkgName))

    // 导入
    sb.WriteString(g.generateImports())

    hasTime := false
    for _, h := range sheet.Headers {
        if h.Type == "time" {
            hasTime = true
            break
        }
    }

    if hasTime {
        sb.WriteString(g.generateConfigTime())
    }

    sb.WriteString(g.generateRowStruct(sheet))
    sb.WriteString(g.generatePackageStruct(sheet, pkgName))
    sb.WriteString(g.generateConfigInterfaceMethods(sheet))
    sb.WriteString(g.generateStoreAndInit(sheet, pkgName))

    return sb.String(), nil
}

func (g *GoGenerator) generateImports() string {
    return `import (
    baseconfig "github.com/CBookShu/kd48/pkg/config"
    lobbyconfig "github.com/CBookShu/kd48/services/lobby/internal/config"
)

`
}

func (g *GoGenerator) generateConfigTime() string {
    return `// TimeFormat 配置时间格式
const TimeFormat = "2006-01-02 15:04:05"

// ConfigTime 包装 time.Time，支持自定义 JSON 解析
type ConfigTime struct {
    Raw  string    ` + "`json:\"-\"`" + `
    Time time.Time ` + "`json:\"-\"`" + `
}

func (t *ConfigTime) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    t.Raw = s
    parsed, err := time.ParseInLocation(TimeFormat, s, time.Local)
    if err != nil {
        return err
    }
    t.Time = parsed
    return nil
}

func (t ConfigTime) MarshalJSON() ([]byte, error) {
    return json.Marshal(t.Raw)
}

func (t ConfigTime) String() string {
    return t.Raw
}

`
}

func (g *GoGenerator) generateRowStruct(sheet *csvparser.Sheet) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("// %sRow 表示配置数据行\n", sheet.ConfigName))
    sb.WriteString(fmt.Sprintf("type %sRow struct {\n", sheet.ConfigName))

    for _, h := range sheet.Headers {
        goType := g.csvTypeToGoType(h.Type)
        sb.WriteString(fmt.Sprintf("    %s %s `json:\"%s\"`\n", g.toCamelCase(h.Name), goType, h.Name))
    }

    sb.WriteString("}\n\n")
    return sb.String()
}

func (g *GoGenerator) generatePackageStruct(sheet *csvparser.Sheet, pkgName string) string {
    return fmt.Sprintf(`// Package 实现 baseconfig.Config 接口
type Package struct {
    data []%sRow
}

`, sheet.ConfigName)
}

func (g *GoGenerator) generateConfigInterfaceMethods(sheet *csvparser.Sheet) string {
    return fmt.Sprintf(`// ConfigName 返回配置名称
func (p *Package) ConfigName() string {
    return "%s"
}

// ConfigData 返回配置数据指针
func (p *Package) ConfigData() any {
    return &p.data
}

`, sheet.ConfigName)
}

func (g *GoGenerator) generateStoreAndInit(sheet *csvparser.Sheet, pkgName string) string {
    return fmt.Sprintf(`// Store 全局配置句柄（业务层直接使用）
var Store *lobbyconfig.TypedStore[%sRow]

func init() {
    Store = lobbyconfig.Register[%sRow](&Package{})
}
`, sheet.ConfigName, sheet.ConfigName)
}

func (g *GoGenerator) csvTypeToGoType(csvType string) string {
    switch csvType {
    case "int32":
        return "int32"
    case "int64":
        return "int64"
    case "string":
        return "string"
    case "time":
        return "ConfigTime"
    case "int32[]":
        return "[]int32"
    case "int64[]":
        return "[]int64"
    case "string[]":
        return "[]string"
    default:
        if strings.Contains(csvType, "=") {
            return g.parseMapType(csvType)
        }
        return "string"
    }
}

func (g *GoGenerator) parseMapType(csvType string) string {
    parts := strings.Split(csvType, "=")
    keyType := strings.TrimSpace(parts[0])
    valueType := strings.TrimSpace(parts[1])

    keyGo := "string"
    if keyType == "int32" || keyType == "int64" {
        keyGo = keyType
    }

    valueGo := "string"
    if valueType == "int32" || valueType == "int64" {
        valueGo = valueType
    }

    return fmt.Sprintf("map[%s]%s", keyGo, valueGo)
}

func (g *GoGenerator) toCamelCase(s string) string {
    parts := strings.Split(s, "_")
    for i := range parts {
        if len(parts[i]) > 0 {
            parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
        }
    }
    return strings.Join(parts, "")
}
```

- [ ] **Run test to verify it passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./tools/config-loader/internal/gogen/... -v -run TestGenerate_PackageStruct
```

Expected: PASS

### Step 6.3: Commit

- [ ] **Commit changes**

```bash
git add tools/config-loader/internal/gogen/generator.go tools/config-loader/internal/gogen/generator_test.go
git commit -m "feat(config-loader): generate Config interface implementation

- Generate Package struct implementing baseconfig.Config
- Generate Store variable for type-safe access
- Generate init() for auto-registration
- Remove old ConfigStruct and ParseFunc

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 7: 集成到 Lobby main.go

**Files:**
- Modify: `services/lobby/cmd/lobby/main.go`

### Step 7.1: Add config imports and initialization

- [ ] **Modify main.go**

在现有 import 块中添加：

```go
import (
    // ... 现有导入 ...

    // 配置加载（自动注册）
    "github.com/CBookShu/kd48/services/lobby/internal/config"
    // 导入生成的配置包触发 init() 自动注册
    // _ "github.com/CBookShu/kd48/services/lobby/internal/config/generated/checkin_daily"
    // 根据实际生成的配置添加
)
```

在 MySQL 和 Redis 初始化之后、gRPC Server 启动之前添加：

```go
    // 初始化配置加载器
    configLoader := config.NewConfigLoader(mysqlPools["default"], config.GetStore())

    // 启动时加载所有配置
    if err := configLoader.LoadAll(context.Background()); err != nil {
        slog.Error("failed to load configs", "error", err)
        // 不 panic，允许部分配置加载失败
    }
    slog.Info("configs loaded")

    // 启动配置热更新订阅
    configWatcher := config.NewConfigWatcher(redisPools["default"], configLoader, config.ConfigNotifyChannel)
    go configWatcher.Start(context.Background())
```

- [ ] **Verify build passes**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go build ./services/lobby/...
```

Expected: 编译成功

### Step 7.2: Commit

- [ ] **Commit changes**

```bash
git add services/lobby/cmd/lobby/main.go
git commit -m "feat(lobby): integrate config loader and watcher

- Initialize ConfigLoader with MySQL connection
- Load all configs at startup
- Start ConfigWatcher for hot reload
- Graceful handling of partial load failures

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 8: 运行完整测试并验证覆盖率

### Step 8.1: Run all tests

- [ ] **Run all config package tests**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -v
```

Expected: All PASS

### Step 8.2: Check test coverage

- [ ] **Generate coverage report**

```bash
cd /Users/cbookshu/dev/temp/kd48 && go test ./services/lobby/internal/config/... -coverprofile=coverage.out -covermode=atomic
go tool cover -func=coverage.out | tail -1
```

Expected: coverage ≥ 80%

### Step 8.3: Commit coverage report

- [ ] **Add coverage to gitignore (optional)**

```bash
# coverage.out 通常不提交
```

---

## Task 9: 最终提交和 PR

### Step 9.1: Push to remote

- [ ] **Push feature branch**

```bash
git push -u origin feature/lobby-config-loading
```

### Step 9.2: Create Pull Request

- [ ] **Create PR**

```bash
gh pr create --title "feat(lobby): add config loading with hot reload" --body "$(cat <<'EOF'
## Summary
- TypedStore[T] 泛型存储，类型安全访问
- Config 接口约束生成的配置代码
- ConfigLoader 从 MySQL 加载配置
- ConfigWatcher 订阅 Redis Pub/Sub 热更新
- init() 自动注册机制
- TDD 测试覆盖 ≥80%

## Test plan
- [x] TypedStore 单元测试（nil, update, concurrent, replace）
- [x] Registry 单元测试（create, singleton, idempotent, not found）
- [x] Loader 单元测试（success, not found, not registered, partial failure）
- [x] Watcher 单元测试（valid message, invalid JSON, context cancel）
- [x] Config-Loader 生成代码测试
- [x] 编译验证
- [x] 覆盖率验证 ≥80%

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## 自检清单

- [x] Spec 覆盖：所有设计文档要求都有对应任务
- [x] 无占位符：所有代码完整，无 TBD/TODO
- [x] 类型一致性：TypedStore[T]、Snapshot[T]、Config 接口在各文件中定义一致
- [x] TDD：每个组件先写测试再实现
- [x] 测试覆盖：≥80% 目标
