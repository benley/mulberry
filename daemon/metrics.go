package daemon

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	restartsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "restarts_total",
		Help:      "Number of times each port has been (re)initialized, killing any existing connections.",
	}, []string{"port"})
	acceptsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "accepts_total",
		Help:      "Number of times each port has accepted a connection.",
	}, []string{"port"})
	connectsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "connects_total",
		Help:      "Number of times each port has successfully dialed a new connection.",
	}, []string{"port"})
	dialErrorsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "dial_errors_total",
		Help:      "Number of times each port has failed to dial a new connection.",
	}, []string{"port"})
	liveConnectionsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "mulberry",
		Name:      "live_connections_total",
		Help:      "Number of active connections through the proxy for each port.",
	}, []string{"port"})
)

func init() {
	prometheus.MustRegister(restartsTotal)
	prometheus.MustRegister(acceptsTotal)
	prometheus.MustRegister(connectsTotal)
	prometheus.MustRegister(dialErrorsTotal)
	prometheus.MustRegister(liveConnectionsTotal)
}
