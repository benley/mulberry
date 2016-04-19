package config

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	configErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "config_errors_total",
		Help:      "Number of failed attempts to read the Mulberry configuration from disk.",
	})
)

func init() {
	prometheus.MustRegister(configErrorsTotal)
}
