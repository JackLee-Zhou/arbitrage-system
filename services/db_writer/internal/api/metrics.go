// db_writer/api -- /metrics Prometheus 指标端点
package api

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// ============================================================
// Metrics -- Prometheus 指标收集器
// ============================================================

// Metrics 指标收集器
// 使用 atomic.Int64 计数器，不引入 Prometheus client 库
type Metrics struct {
	// 按 bucket 统计
	WritesTotal      map[string]*atomic.Int64
	WriteErrorsTotal map[string]*atomic.Int64

	// DLQ 大小（按 bucket）
	DLQSize map[string]*atomic.Int64

	// Consumer 滞后（按 stream）
	ConsumerLag map[string]*atomic.Int64

	// 延迟分位数（按 bucket 的 p50，毫秒）
	WriteLatencyP50 map[string]*atomic.Int64
}

// NewMetrics 创建指标收集器
// buckets: bucket 名称列表
// streams: stream 名称列表
func NewMetrics(buckets []string, streams []string) *Metrics {
	m := &Metrics{
		WritesTotal:      make(map[string]*atomic.Int64, len(buckets)),
		WriteErrorsTotal: make(map[string]*atomic.Int64, len(buckets)),
		DLQSize:          make(map[string]*atomic.Int64, len(buckets)),
		WriteLatencyP50:  make(map[string]*atomic.Int64, len(buckets)),
		ConsumerLag:      make(map[string]*atomic.Int64, len(streams)),
	}

	for _, bucket := range buckets {
		m.WritesTotal[bucket] = &atomic.Int64{}
		m.WriteErrorsTotal[bucket] = &atomic.Int64{}
		m.DLQSize[bucket] = &atomic.Int64{}
		m.WriteLatencyP50[bucket] = &atomic.Int64{}
	}

	for _, stream := range streams {
		m.ConsumerLag[stream] = &atomic.Int64{}
	}

	return m
}

// ServeHTTP 实现 http.Handler，输出 Prometheus 文本格式
func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	var body string

	// WritesTotal
	for bucket, counter := range m.WritesTotal {
		val := counter.Load()
		body += fmt.Sprintf("db_writer_writes_total{bucket=%q} %d\n", bucket, val)
	}

	// WriteErrorsTotal
	for bucket, counter := range m.WriteErrorsTotal {
		val := counter.Load()
		body += fmt.Sprintf("db_writer_write_errors_total{bucket=%q} %d\n", bucket, val)
	}

	// WriteLatencyP50
	for bucket, counter := range m.WriteLatencyP50 {
		val := counter.Load()
		body += fmt.Sprintf("db_writer_write_latency_ms{bucket=%q,quantile=\"p50\"} %d\n", bucket, val)
	}

	// DLQSize
	for bucket, counter := range m.DLQSize {
		val := counter.Load()
		body += fmt.Sprintf("db_writer_dlq_size{bucket=%q} %d\n", bucket, val)
	}

	// ConsumerLag
	for stream, counter := range m.ConsumerLag {
		val := counter.Load()
		body += fmt.Sprintf("db_writer_consumer_lag{stream=%q} %d\n", stream, val)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, body)
}
