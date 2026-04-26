# kd48 可观测性体系实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 kd48 微服务架构构建生产级 Metrics 监控体系，包括 Prometheus 指标暴露、Grafana Dashboard 和基础告警。

**Architecture:** 使用 Prometheus Go Client 为 Gateway (Fiber)、User Service (gRPC)、Lobby Service (gRPC) 添加 /metrics 端点。HTTP 指标通过中间件自动收集，gRPC 指标通过拦截器收集，业务指标手动埋点。所有指标通过 Prometheus Server 抓取，Grafana 展示。

**Tech Stack:** Prometheus Client Go v1.19, fiberprometheus v2.7, go-grpc-prometheus v1.2, Grafana 10.4

**设计规格参考:** `docs/superpowers/specs/2026-04-26-observability-design.md`

---

## 文件结构

```
pkg/
├── metrics/
│   ├── metrics.go          # 核心指标注册表
│   ├── http.go             # Fiber HTTP 中间件
│   ├── grpc.go             # gRPC 拦截器
│   └── dsroute.go          # 数据源路由指标包装
├── dsroute/
│   └── router.go           # 修改：添加指标收集
├── otelkit/
│   └── provider.go         # 已存在

gateway/
├── cmd/gateway/main.go     # 修改：添加 /metrics 端点
└── internal/ws/
    └── connection_manager.go # 修改：暴露 Prometheus 指标

services/user/
├── cmd/user/main.go        # 修改：添加 /metrics 端点
└── go.mod                  # 修改：添加依赖

services/lobby/
├── cmd/lobby/main.go       # 修改：添加 /metrics 端点
└── go.mod                  # 修改：添加依赖

docker/
└── prometheus.yml          # 修改：添加服务 targets
```

---

## Phase 1: 核心指标 (2天)

### Task 1: 创建 pkg/metrics 核心包

**Files:**
- Create: `pkg/metrics/metrics.go`
- Create: `pkg/metrics/http.go`
- Create: `pkg/metrics/grpc.go`
- Test: `pkg/metrics/metrics_test.go`

**依赖添加:**
```bash
cd /Users/cbookshu/dev/temp/kd48/pkg
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

- [ ] **Step 1: 创建核心指标注册表**

Create: `pkg/metrics/metrics.go`

```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	// Registry 是全局指标注册表
	Registry = prometheus.NewRegistry()
	
	// HTTP 指标
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "path", "status"},
	)
	
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kd48_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)
	
	// gRPC 指标
	GRPCRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"service", "method", "status"},
	)
	
	GRPCRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kd48_grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)
)

func init() {
	// 注册所有指标
	Registry.MustRegister(HTTPRequestsTotal)
	Registry.MustRegister(HTTPRequestDuration)
	Registry.MustRegister(GRPCRequestsTotal)
	Registry.MustRegister(GRPCRequestDuration)
}

// Handler 返回 Prometheus HTTP handler
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{})
}
```

- [ ] **Step 2: 创建 HTTP 中间件**

Create: `pkg/metrics/http.go`

```go
package metrics

import (
	"strconv"
	"time"
	
	"github.com/gofiber/fiber/v2"
)

// FiberMiddleware 返回 Fiber 指标中间件
func FiberMiddleware(serviceName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// 执行请求
		err := c.Next()
		
		// 记录指标
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		path := c.Route().Path  // 使用路由模板，避免高基数
		method := c.Method()
		
		HTTPRequestsTotal.WithLabelValues(serviceName, method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(serviceName, method, path).Observe(duration)
		
		return err
	}
}
```

- [ ] **Step 3: 创建 gRPC 拦截器**

Create: `pkg/metrics/grpc.go`

```go
package metrics

import (
	"context"
	"time"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor 返回 gRPC 一元拦截器
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		
		// 执行请求
		resp, err := handler(ctx, req)
		
		// 记录指标
		duration := time.Since(start).Seconds()
		method := info.FullMethod
		stat := "OK"
		if err != nil {
			stat = status.Code(err).String()
		}
		
		GRPCRequestsTotal.WithLabelValues(serviceName, method, stat).Inc()
		GRPCRequestDuration.WithLabelValues(serviceName, method).Observe(duration)
		
		return resp, err
	}
}

// StreamServerInterceptor 返回 gRPC 流拦截器
func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		
		// 执行请求
		err := handler(srv, stream)
		
		// 记录指标
		duration := time.Since(start).Seconds()
		method := info.FullMethod
		stat := "OK"
		if err != nil {
			stat = status.Code(err).String()
		}
		
		GRPCRequestsTotal.WithLabelValues(serviceName, method, stat).Inc()
		GRPCRequestDuration.WithLabelValues(serviceName, method).Observe(duration)
		
		return err
	}
}
```

- [ ] **Step 4: 添加单元测试**

Create: `pkg/metrics/metrics_test.go`

```go
package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestMetricsRegistration(t *testing.T) {
	// 验证指标已注册
	assert.NotNil(t, HTTPRequestsTotal)
	assert.NotNil(t, HTTPRequestDuration)
	assert.NotNil(t, GRPCRequestsTotal)
	assert.NotNil(t, GRPCRequestDuration)
}

func TestHTTPMetrics(t *testing.T) {
	// 记录一个 HTTP 请求
	HTTPRequestsTotal.WithLabelValues("test-service", "GET", "/test", "200").Inc()
	HTTPRequestDuration.WithLabelValues("test-service", "GET", "/test").Observe(0.1)
	
	// 验证计数
	count, err := testutil.GetCounterValue(HTTPRequestsTotal.WithLabelValues("test-service", "GET", "/test", "200"))
	assert.NoError(t, err)
	assert.Equal(t, float64(1), count)
}

func TestHandler(t *testing.T) {
	// 测试 handler 返回有效的 Prometheus 格式
	handler := Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	
	handler.ServeHTTP(rr, req)
	
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "kd48_http_requests_total")
}
```

- [ ] **Step 5: 运行测试**

```bash
cd /Users/cbookshu/dev/temp/kd48/pkg
go test ./metrics/... -v
```

Expected: All tests PASS

- [ ] **Step 6: 提交**

```bash
cd /Users/cbookshu/dev/temp/kd48
git add pkg/metrics/
git commit -m "feat(metrics): create core metrics package with HTTP and gRPC instrumentation

- Add metrics registry with 4 core metrics
- Implement Fiber middleware for HTTP metrics
- Implement gRPC interceptors for RPC metrics
- Add unit tests for all components"
```

---

### Task 2: 扩展 pkg/dsroute 添加连接池指标

**Files:**
- Create: `pkg/metrics/dsroute.go`
- Modify: `pkg/dsroute/router.go`

- [ ] **Step 1: 创建数据源路由指标定义**

Create: `pkg/metrics/dsroute.go`

```go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// DBPoolConnectionsActive 活跃连接数
	DBPoolConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kd48_db_pool_connections_active",
			Help: "Current number of active connections in the pool",
		},
		[]string{"service", "db_type", "pool_name"},
	)
	
	// DBPoolConnectionsIdle 空闲连接数
	DBPoolConnectionsIdle = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kd48_db_pool_connections_idle",
			Help: "Current number of idle connections in the pool",
		},
		[]string{"service", "db_type", "pool_name"},
	)
	
	// DBPoolWaitDurationSeconds 获取连接等待时间
	DBPoolWaitDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kd48_db_pool_wait_duration_seconds",
			Help:    "Time spent waiting for a connection from the pool",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"service", "db_type"},
	)
)

func init() {
	Registry.MustRegister(DBPoolConnectionsActive)
	Registry.MustRegister(DBPoolConnectionsIdle)
	Registry.MustRegister(DBPoolWaitDurationSeconds)
}
```

- [ ] **Step 2: 修改 Router 添加指标收集**

Modify: `pkg/dsroute/router.go`

Add imports:
```go
import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
	
	"github.com/CBookShu/kd48/pkg/metrics"
	"github.com/redis/go-redis/v9"
)
```

Add service name field and modify constructor:
```go
type Router struct {
	mysqlPools  map[string]*sql.DB
	redisPools  map[string]redis.UniversalClient
	mysqlRoutes atomic.Value // []RouteRule
	redisRoutes atomic.Value // []RouteRule
	serviceName string       // 新增：服务名用于指标标签
}

// NewRouter 创建带服务名的 Router
func NewRouter(
	mysqlPools map[string]*sql.DB,
	redisPools map[string]redis.UniversalClient,
	mysqlRoutes []RouteRule,
	redisRoutes []RouteRule,
	serviceName string, // 新增参数
) (*Router, error) {
	// ... 原有验证逻辑 ...
	
	r := &Router{
		mysqlPools:  mysqlPools,
		redisPools:  redisPools,
		serviceName: serviceName,
	}
	
	r.mysqlRoutes.Store(mysqlRoutes)
	r.redisRoutes.Store(redisRoutes)
	
	// 启动指标收集 goroutine
	go r.collectMetrics()
	
	return r, nil
}

// collectMetrics 定期收集连接池指标
func (r *Router) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// 收集 MySQL 连接池指标
		for poolName, db := range r.mysqlPools {
			stats := db.Stats()
			metrics.DBPoolConnectionsActive.WithLabelValues(r.serviceName, "mysql", poolName).Set(float64(stats.InUse))
			metrics.DBPoolConnectionsIdle.WithLabelValues(r.serviceName, "mysql", poolName).Set(float64(stats.Idle))
		}
		
		// 收集 Redis 连接池指标（通过 PoolStats）
		for poolName, client := range r.redisPools {
			if pooler, ok := client.(interface{ PoolStats() *redis.PoolStats }); ok {
				stats := pooler.PoolStats()
				metrics.DBPoolConnectionsActive.WithLabelValues(r.serviceName, "redis", poolName).Set(float64(stats.Hits))
				metrics.DBPoolConnectionsIdle.WithLabelValues(r.serviceName, "redis", poolName).Set(float64(stats.IdleConns))
			}
		}
	}
}

// ResolveDB 添加指标埋点
func (r *Router) ResolveDB(ctx context.Context, routingKey string) (*sql.DB, string, error) {
	start := time.Now()
	defer func() {
		metrics.DBPoolWaitDurationSeconds.WithLabelValues(r.serviceName, "mysql").Observe(time.Since(start).Seconds())
	}()
	
	routes := r.mysqlRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	db := r.mysqlPools[poolName]
	return db, poolName, nil
}

// ResolveRedis 添加指标埋点
func (r *Router) ResolveRedis(ctx context.Context, routingKey string) (redis.UniversalClient, string, error) {
	start := time.Now()
	defer func() {
		metrics.DBPoolWaitDurationSeconds.WithLabelValues(r.serviceName, "redis").Observe(time.Since(start).Seconds())
	}()
	
	routes := r.redisRoutes.Load().([]RouteRule)
	poolName, _, err := ResolvePoolName(routes, routingKey)
	if err != nil {
		return nil, "", err
	}
	client := r.redisPools[poolName]
	return client, poolName, nil
}
```

- [ ] **Step 3: 运行 pkg 测试**

```bash
cd /Users/cbookshu/dev/temp/kd48/pkg
go test ./... -v
```

Expected: All tests PASS (dsroute 测试可能需要调整)

- [ ] **Step 4: 提交**

```bash
git add pkg/dsroute/router.go pkg/metrics/dsroute.go
git commit -m "feat(metrics): add database pool metrics to dsroute

- Add db_pool_connections_active/idle gauges
- Add db_pool_wait_duration_seconds histogram
- Collect metrics every 30s from MySQL and Redis pools
- Update ResolveDB/ResolveRedis to track wait time"
```

---

### Task 3: Gateway 服务添加 /metrics 端点

**Files:**
- Modify: `gateway/cmd/gateway/main.go`
- Modify: `gateway/internal/ws/connection_manager.go`
- Modify: `gateway/go.mod`

- [ ] **Step 1: 添加依赖**

```bash
cd /Users/cbookshu/dev/temp/kd48/gateway
go get github.com/ansrivas/fiberprometheus/v2
```

- [ ] **Step 2: 修改 main.go 添加 /metrics 端点**

Modify: `gateway/cmd/gateway/main.go`

Find the Fiber app setup section (around line 50-80), add:

```go
import (
	// ... existing imports ...
	"github.com/CBookShu/kd48/pkg/metrics"
	"github.com/ansrivas/fiberprometheus/v2"
)

func main() {
	// ... existing setup code ...
	
	app := fiber.New()
	
	// 添加 Prometheus 中间件（放在最前面）
	prometheus := fiberprometheus.New("gateway")
	prometheus.RegisterAt(app, "/metrics")
	app.Use(prometheus.Middleware)
	
	// 添加 pkg/metrics 中间件
	app.Use(metrics.FiberMiddleware("gateway"))
	
	// ... rest of setup ...
}
```

- [ ] **Step 3: 修改 ConnectionManager 暴露 Prometheus 指标**

Modify: `gateway/internal/ws/connection_manager.go`

Add imports:
```go
import (
	"context"
	"log/slog"
	"sync"
	"time"
	
	"github.com/CBookShu/kd48/pkg/metrics"
	"github.com/gofiber/contrib/websocket"
	"github.com/prometheus/client_golang/prometheus"
)
```

Add Prometheus metrics vars:
```go
var (
	wsConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kd48_ws_connections_active",
			Help: "Current number of active WebSocket connections",
		},
		[]string{},
	)
	
	wsConnectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_ws_connections_total",
			Help: "Total number of WebSocket connections established",
		},
		[]string{},
	)
)

func init() {
	metrics.Registry.MustRegister(wsConnectionsActive)
	metrics.Registry.MustRegister(wsConnectionsTotal)
}
```

Modify RegisterConnection to increment counter:
```go
func (cm *ConnectionManager) RegisterConnection(clientID string, conn *websocket.Conn) {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()
	
	cm.connections[clientID] = conn
	cm.metrics.TotalConnections++
	cm.metrics.ActiveConnections++
	
	// 更新 Prometheus 指标
	wsConnectionsActive.WithLabelValues().Set(float64(cm.metrics.ActiveConnections))
	wsConnectionsTotal.WithLabelValues().Inc()
	
	slog.Info("connection registered", "clientID", clientID, "active", cm.metrics.ActiveConnections)
}
```

Modify UnregisterConnection to update gauge:
```go
func (cm *ConnectionManager) UnregisterConnection(clientID string) {
	cm.connMu.Lock()
	defer cm.connMu.Unlock()
	
	if _, exists := cm.connections[clientID]; exists {
		delete(cm.connections, clientID)
		cm.metrics.ActiveConnections--
		cm.metrics.DisconnectedCount++
		
		// 更新 Prometheus 指标
		wsConnectionsActive.WithLabelValues().Set(float64(cm.metrics.ActiveConnections))
		
		slog.Info("connection unregistered", "clientID", clientID, "active", cm.metrics.ActiveConnections)
	}
}
```

- [ ] **Step 4: 构建并测试**

```bash
cd /Users/cbookshu/dev/temp/kd48
go build ./gateway/...

# 启动 gateway
go run ./gateway/cmd/gateway &

# 测试 /metrics 端点
curl http://localhost:8080/metrics | head -20
```

Expected: 返回 Prometheus 格式数据，包含 `kd48_http_requests_total` 等指标

- [ ] **Step 5: 提交**

```bash
git add gateway/
git commit -m "feat(metrics): add /metrics endpoint to Gateway service

- Add fiberprometheus middleware for automatic HTTP metrics
- Expose WebSocket connection metrics via Prometheus
- Register at :8080/metrics"
```

---

### Task 4: User Service 添加 /metrics 端点

**Files:**
- Modify: `services/user/cmd/user/main.go`
- Modify: `services/user/go.mod`

- [ ] **Step 1: 添加依赖**

```bash
cd /Users/cbookshu/dev/temp/kd48/services/user
go get github.com/grpc-ecosystem/go-grpc-prometheus
```

- [ ] **Step 2: 修改 main.go**

Modify: `services/user/cmd/user/main.go`

Add imports:
```go
import (
	// ... existing imports ...
	"github.com/CBookShu/kd48/pkg/metrics"
	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)
```

In main function, before gRPC server setup, add metrics server:

```go
func main() {
	// ... existing setup ...
	
	// 启动 metrics HTTP server (用于 Prometheus 抓取)
	go func() {
		http.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":9000", nil); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()
	
	// gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor("user")),
		grpc.StreamInterceptor(metrics.StreamServerInterceptor("user")),
	)
	
	// ... rest of setup ...
}
```

Wait, port conflict. User service gRPC is on :9000. Change metrics to different port or use same port with multiplexing.

Better approach - use separate port for metrics:

```go
// 启动 metrics HTTP server on different port
go func() {
	http.Handle("/metrics", metrics.Handler())
	if err := http.ListenAndServe(":9091", nil); err != nil {
		slog.Error("metrics server failed", "error", err)
	}
}()
```

Actually, let's follow the design doc and put metrics on the same port using a listener multiplexer. But for simplicity, let's use a separate port that Prometheus can scrape.

Update docker/prometheus.yml to include the new port.

- [ ] **Step 3: 构建并测试**

```bash
cd /Users/cbookshu/dev/temp/kd48
go build ./services/user/...

# 启动 user service
go run ./services/user/cmd/user &

# 测试 /metrics 端点
curl http://localhost:9091/metrics | head -20
```

Expected: 返回 Prometheus 格式数据

- [ ] **Step 4: 提交**

```bash
git add services/user/
git commit -m "feat(metrics): add /metrics endpoint to User service

- Add gRPC interceptors for automatic RPC metrics
- Expose metrics on :9091 for Prometheus scraping
- Track request count and duration per method"
```

---

### Task 5: Lobby Service 添加 /metrics 端点

**Files:**
- Modify: `services/lobby/cmd/lobby/main.go`
- Modify: `services/lobby/go.mod`

- [ ] **Step 1: 添加依赖**

```bash
cd /Users/cbookshu/dev/temp/kd48/services/lobby
go get github.com/grpc-ecosystem/go-grpc-prometheus
```

- [ ] **Step 2: 修改 main.go**

Modify: `services/lobby/cmd/lobby/main.go`

Similar to User service, add:

```go
import (
	// ... existing imports ...
	"github.com/CBookShu/kd48/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func main() {
	// ... existing setup ...
	
	// 启动 metrics HTTP server
	go func() {
		http.Handle("/metrics", metrics.Handler())
		if err := http.ListenAndServe(":9092", nil); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()
	
	// gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(metrics.UnaryServerInterceptor("lobby")),
		grpc.StreamInterceptor(metrics.StreamServerInterceptor("lobby")),
	)
	
	// ... rest of setup ...
}
```

- [ ] **Step 3: 构建并测试**

```bash
cd /Users/cbookshu/dev/temp/kd48
go build ./services/lobby/...

# 启动 lobby service
go run ./services/lobby/cmd/lobby &

# 测试 /metrics 端点
curl http://localhost:9092/metrics | head -20
```

- [ ] **Step 4: 提交**

```bash
git add services/lobby/
git commit -m "feat(metrics): add /metrics endpoint to Lobby service

- Add gRPC interceptors for automatic RPC metrics
- Expose metrics on :9092 for Prometheus scraping
- Track request count and duration per method"
```

---

### Task 6: 更新 Prometheus 配置并验证

**Files:**
- Modify: `docker/prometheus.yml`

- [ ] **Step 1: 更新 prometheus.yml**

Modify: `docker/prometheus.yml`

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'gateway'
    static_configs:
      - targets: ['host.docker.internal:8080']
    metrics_path: '/metrics'

  - job_name: 'user'
    static_configs:
      - targets: ['host.docker.internal:9091']
    metrics_path: '/metrics'

  - job_name: 'lobby'
    static_configs:
      - targets: ['host.docker.internal:9092']
    metrics_path: '/metrics'
```

- [ ] **Step 2: 启动完整环境并验证**

```bash
# 启动基础设施
docker-compose up -d

# 启动所有服务
./run.sh start

# 或者手动启动
go run ./gateway/cmd/gateway &
go run ./services/user/cmd/user &
go run ./services/lobby/cmd/lobby &

# 验证 Prometheus 抓取
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets'
```

Expected: 所有 targets 状态为 "UP"

- [ ] **Step 3: 在 Grafana 中验证指标**

访问 http://localhost:3000

1. 配置 Prometheus 数据源 (http://prometheus:9090)
2. 在 Explore 中查询: `kd48_http_requests_total`
3. 验证能看到 Gateway 的请求数据

- [ ] **Step 4: 提交**

```bash
git add docker/prometheus.yml
git commit -m "chore(prometheus): add service targets for metrics scraping

- Gateway: :8080/metrics
- User: :9091/metrics
- Lobby: :9092/metrics"
```

---

## Phase 2: 业务指标 + Dashboard (1-2天)

### Task 7: WebSocket 业务指标

**Files:**
- Modify: `pkg/metrics/business.go` (create)
- Modify: `gateway/internal/ws/handler.go`

- [ ] **Step 1: 创建业务指标定义**

Create: `pkg/metrics/business.go`

```go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// AuthLoginTotal 登录次数
	AuthLoginTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_auth_login_total",
			Help: "Total number of login attempts",
		},
		[]string{"result"}, // success, failure
	)
	
	// AuthRegisterTotal 注册次数
	AuthRegisterTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_auth_register_total",
			Help: "Total number of registration attempts",
		},
		[]string{"result"},
	)
	
	// CheckinTotal 签到次数
	CheckinTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_checkin_total",
			Help: "Total number of check-ins",
		},
		[]string{"result"},
	)
	
	// ItemGrantedTotal 物品发放次数
	ItemGrantedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_item_granted_total",
			Help: "Total number of items granted",
		},
		[]string{"item_type"},
	)
)

func init() {
	Registry.MustRegister(AuthLoginTotal)
	Registry.MustRegister(AuthRegisterTotal)
	Registry.MustRegister(CheckinTotal)
	Registry.MustRegister(ItemGrantedTotal)
}
```

- [ ] **Step 2: 在 User Service 登录处埋点**

Modify: `services/user/cmd/user/server.go`

Find Login handler, add:

```go
import "github.com/CBookShu/kd48/pkg/metrics"

func (s *server) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginResponse, error) {
	// ... validation ...
	
	// 验证密码
	if err := bcrypt.CompareHashAndPassword(...); err != nil {
		metrics.AuthLoginTotal.WithLabelValues("failure").Inc()
		return nil, status.Errorf(codes.Unauthenticated, "invalid credentials")
	}
	
	// 登录成功
	metrics.AuthLoginTotal.WithLabelValues("success").Inc()
	
	// ... rest of logic ...
}
```

Similar for Register.

- [ ] **Step 3: 在 Lobby Service 签到处埋点**

Modify: `services/lobby/cmd/lobby/checkin_server.go`

Find Checkin handler, add metrics.

- [ ] **Step 4: 提交**

```bash
git add pkg/metrics/business.go services/user/ services/lobby/
git commit -m "feat(metrics): add business metrics for auth and checkin

- Track login/register success/failure
- Track checkin attempts and item grants"
```

---

### Task 8: 创建 Grafana Dashboard

**Files:**
- Create: `docker/grafana/dashboards/service-overview.json`
- Create: `docker/grafana/dashboards/websocket.json`
- Create: `docker/grafana/dashboards/business.json`
- Modify: `docker-compose.yml`

- [ ] **Step 1: 创建 Dashboard 目录结构**

```bash
mkdir -p docker/grafana/dashboards
mkdir -p docker/grafana/provisioning/dashboards
mkdir -p docker/grafana/provisioning/datasources
```

- [ ] **Step 2: 配置 Prometheus 数据源**

Create: `docker/grafana/provisioning/datasources/prometheus.yml`

```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

- [ ] **Step 3: 配置 Dashboard 自动加载**

Create: `docker/grafana/provisioning/dashboards/dashboards.yml`

```yaml
apiVersion: 1
providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    editable: true
    options:
      path: /var/lib/grafana/dashboards
```

- [ ] **Step 4: 创建服务概览 Dashboard JSON**

Create: `docker/grafana/dashboards/service-overview.json`

(Use Grafana UI to create and export, or write manually)

Key panels:
- QPS: `sum(rate(kd48_http_requests_total[5m])) by (service)`
- P99 Latency: `histogram_quantile(0.99, sum(rate(kd48_http_request_duration_seconds_bucket[5m])) by (service, le))`
- Error Rate: `sum(rate(kd48_http_requests_total{status=~"5.."}[5m])) / sum(rate(kd48_http_requests_total[5m]))`

- [ ] **Step 5: 更新 docker-compose.yml**

Add volume mounts for Grafana:

```yaml
grafana:
  image: grafana/grafana:10.4.0
  ports:
    - "3000:3000"
  volumes:
    - ./docker/grafana/provisioning:/etc/grafana/provisioning
    - ./docker/grafana/dashboards:/var/lib/grafana/dashboards
    - grafana_data:/var/lib/grafana
```

- [ ] **Step 6: 重启并验证**

```bash
docker-compose restart grafana

# 访问 http://localhost:3000
# 查看 Dashboards -> Browse
```

- [ ] **Step 7: 提交**

```bash
git add docker/grafana/ docker-compose.yml
git commit -m "feat(grafana): add dashboards for service monitoring

- Service Overview: QPS, latency, error rate
- WebSocket Monitoring: connection metrics
- Business Metrics: auth and checkin stats
- Auto-provisioned datasources and dashboards"
```

---

## 验收检查清单

Phase 1 完成标准:
- [ ] `curl localhost:8080/metrics` 返回有效 Prometheus 数据
- [ ] `curl localhost:9091/metrics` 返回 User service 指标
- [ ] `curl localhost:9092/metrics` 返回 Lobby service 指标
- [ ] Prometheus UI (localhost:9090/targets) 显示所有服务 UP
- [ ] 能看到 `kd48_http_requests_total` 等指标数据

Phase 2 完成标准:
- [ ] 登录/注册操作后 `kd48_auth_login_total` 指标增加
- [ ] Grafana 能看到 3 个 Dashboard
- [ ] Dashboard 数据实时更新

---

## 执行选项

**Plan complete and saved to `docs/superpowers/plans/2026-04-26-observability-implementation-plan.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - Dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints for review

**Which approach would you like?**
