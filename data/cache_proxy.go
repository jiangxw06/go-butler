package data

import (
	"github.com/jiangxw06/go-butler/internal/env"
	prometheus2 "github.com/jiangxw06/go-butler/internal/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cacheCounters  *prometheus.CounterVec
	cacheDurations *prometheus.HistogramVec
)

func init() {
	cacheCounters = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   prometheus2.NAMESPACE,
			Subsystem:   env.GetAppName(),
			Help:        "缓存代理访问计数",
			Name:        "cache_count",
			ConstLabels: map[string]string{"ip": env.GetIP()},
		},
		[]string{"template", "event"})
	cacheDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: prometheus2.NAMESPACE,
			Subsystem: env.GetAppName(),
			Name:      "cache_duration",
			Help:      "缓存代理访问耗时",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"template", "type"})
}
