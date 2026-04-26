package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	WSConnectionsActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kd48_ws_connections_active",
			Help: "Current number of active WebSocket connections",
		},
		[]string{},
	)

	WSConnectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_ws_connections_total",
			Help: "Total number of WebSocket connections established",
		},
		[]string{},
	)

	AuthLoginTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_auth_login_total",
			Help: "Total number of login attempts",
		},
		[]string{"result"},
	)

	AuthRegisterTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_auth_register_total",
			Help: "Total number of registration attempts",
		},
		[]string{"result"},
	)

	CheckinTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_checkin_total",
			Help: "Total number of check-ins",
		},
		[]string{"result"},
	)

	ItemGrantedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kd48_item_granted_total",
			Help: "Total number of items granted",
		},
		[]string{"item_type"},
	)
)

func init() {
	Registry.MustRegister(WSConnectionsActive)
	Registry.MustRegister(WSConnectionsTotal)
	Registry.MustRegister(AuthLoginTotal)
	Registry.MustRegister(AuthRegisterTotal)
	Registry.MustRegister(CheckinTotal)
	Registry.MustRegister(ItemGrantedTotal)
}
