package web

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	MetricMessagesProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mailflow_messages_processed_total",
		Help: "Total number of messages processed by the poller.",
	})
	MetricRulesMatched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mailflow_rules_matched_total",
		Help: "Total number of rule matches.",
	}, []string{"rule"})
	MetricActionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mailflow_actions_total",
		Help: "Total number of actions executed.",
	}, []string{"type", "status"})
	MetricErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mailflow_errors_total",
		Help: "Total number of processing errors.",
	})
	MetricPollerTicks = promauto.NewCounter(prometheus.CounterOpts{
		Name: "mailflow_poller_ticks_total",
		Help: "Total number of poller ticks.",
	})
	MetricPollerTickDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "mailflow_poller_tick_duration_seconds",
		Help:    "Duration of poller ticks in seconds.",
		Buckets: prometheus.DefBuckets,
	})
	MetricCPUPercent = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mailflow_cpu_percent",
		Help: "Current CPU usage percent (Getrusage).",
	})
	MetricMemoryBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mailflow_memory_bytes",
		Help: "Current memory usage in bytes.",
	})
	MetricUptimeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mailflow_uptime_seconds",
		Help: "Process uptime in seconds.",
	})
)

func PromHandler() http.Handler {
	return promhttp.Handler()
}
