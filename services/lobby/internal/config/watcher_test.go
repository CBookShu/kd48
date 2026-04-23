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
