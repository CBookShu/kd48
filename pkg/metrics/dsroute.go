package metrics

import "github.com/prometheus/client_golang/prometheus"

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
