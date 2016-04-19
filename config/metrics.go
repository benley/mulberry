package config

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	configLoadsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "config_loads_total",
		Help:      "Number of started attempts to load the Mulberry configuration.",
	})
	configReadErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "config_read_errors_total",
		Help:      "Number of failed attempts to read the Mulberry configuration.",
	})
	configParseErrorsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "config_parse_errors_total",
		Help:      "Number of failed attempts to parse the Mulberry configuration.",
	})
	configSuccessesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mulberry",
		Name:      "config_successes_total",
		Help:      "Number of successful attempts to load the Mulberry configuration.",
	})
)

func init() {
	prometheus.MustRegister(configLoadsTotal)
	prometheus.MustRegister(configReadErrorsTotal)
	prometheus.MustRegister(configParseErrorsTotal)
	prometheus.MustRegister(configSuccessesTotal)
}
