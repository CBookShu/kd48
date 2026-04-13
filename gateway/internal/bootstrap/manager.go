package bootstrap

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/CBookShu/kd48/gateway/internal/ws"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

const defaultWatchPrefix = "kd48/meta/"

// Manager Bootstrap + Watch；debounce 后全量重建（§11.5 全量 resync 的简化一致实现）。
type Manager struct {
	etcd          *clientv3.Client
	router        *ws.AtomicRouter
	typesPrefix   string
	routesPrefix  string
	watchPrefix   string
	dialOpts      []grpc.DialOption
	debounce      time.Duration
	drainDelay    time.Duration

	buildMu       sync.Mutex
	nextWatchRev  int64
	activeMu      sync.Mutex
	activeConns   []*grpc.ClientConn
	reloadMu      sync.Mutex
	reloadTimer   *time.Timer
}

// NewManager watchPrefix 为空时使用 kd48/meta/。
func NewManager(cli *clientv3.Client, router *ws.AtomicRouter, typesPrefix, routesPrefix string, dialOpts []grpc.DialOption) *Manager {
	wp := defaultWatchPrefix
	return &Manager{
		etcd:         cli,
		router:       router,
		typesPrefix:  typesPrefix,
		routesPrefix: routesPrefix,
		watchPrefix:  wp,
		dialOpts:     dialOpts,
		debounce:     200 * time.Millisecond,
		drainDelay:   30 * time.Second,
	}
}

// Bootstrap 全量 Range 并替换路由快照与连接池。
func (m *Manager) Bootstrap(ctx context.Context) error {
	m.buildMu.Lock()
	defer m.buildMu.Unlock()

	routes, conns, nextRev, err := Build(ctx, m.etcd, m.typesPrefix, m.routesPrefix, m.dialOpts)
	if err != nil {
		return err
	}
	atomic.StoreInt64(&m.nextWatchRev, nextRev)
	m.router.Store(routes)

	m.activeMu.Lock()
	old := m.activeConns
	m.activeConns = conns
	m.activeMu.Unlock()

	if len(old) > 0 {
		delay := m.drainDelay
		go func(toClose []*grpc.ClientConn, d time.Duration) {
			time.Sleep(d)
			for _, c := range toClose {
				_ = c.Close()
			}
		}(old, delay)
	}
	return nil
}

func (m *Manager) scheduleReload(ctx context.Context) {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	if m.reloadTimer != nil {
		m.reloadTimer.Stop()
	}
	m.reloadTimer = time.AfterFunc(m.debounce, func() {
		if err := m.Bootstrap(ctx); err != nil {
			slog.Error("debounced bootstrap failed", "error", err)
		}
	})
}

// Run 阻塞直至 ctx 取消；Watch 断连或错误时 sleep 后重 Bootstrap 并重建 Watch。
func (m *Manager) Run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		rev := atomic.LoadInt64(&m.nextWatchRev)
		ch := m.etcd.Watch(ctx, m.watchPrefix, clientv3.WithPrefix(), clientv3.WithRev(rev))
		failed := false
		for wr := range ch {
			if ctx.Err() != nil {
				return
			}
			if err := wr.Err(); err != nil {
				slog.Error("etcd watch", "error", err)
				failed = true
				break
			}
			m.scheduleReload(ctx)
		}
		if ctx.Err() != nil {
			return
		}
		if !failed {
			slog.Warn("etcd watch channel closed, reconnecting")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if err := m.Bootstrap(ctx); err != nil {
			slog.Error("bootstrap after watch drop", "error", err)
		}
	}
}

// Close 立即关闭当前持有的 gRPC 连接（进程退出时调用）。
func (m *Manager) Close() {
	m.activeMu.Lock()
	for _, c := range m.activeConns {
		_ = c.Close()
	}
	m.activeConns = nil
	m.activeMu.Unlock()
}
