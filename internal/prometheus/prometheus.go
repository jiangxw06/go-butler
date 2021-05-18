package prometheus

import (
	"github.com/jiangxw06/go-butler/internal/env"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"strconv"
	"sync"
)

const NAMESPACE = "interaction"

var (
	subsystem        string
	errorCounters    *prometheus.CounterVec
	httpReqDurations *prometheus.HistogramVec
	promOnce         sync.Once
)

func initProm() {
	subsystem = env.GetAppName()
	ip := env.GetIP()
	//errors
	errorCounters = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   NAMESPACE,
			Subsystem:   subsystem,
			Help:        "处理消息失败数量",
			Name:        "handle_msg_failed_count",
			ConstLabels: map[string]string{"ip": ip},
		},
		[]string{"type"})

	httpReqDurations = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: NAMESPACE,
			Subsystem: subsystem,
			Name:      "common_api_duration",
			Help:      "请求及其耗时",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"host", "resp_http_code"})
}

func ErrorCounter(errType string) prometheus.Counter {
	promOnce.Do(initProm)
	return errorCounters.With(prometheus.Labels{"type": errType})
}

func HttpReqDuration(host, path string, httpCode int) prometheus.Observer {
	promOnce.Do(initProm)
	return httpReqDurations.With(prometheus.Labels{"host": host, "resp_http_code": strconv.Itoa(httpCode)})
}
