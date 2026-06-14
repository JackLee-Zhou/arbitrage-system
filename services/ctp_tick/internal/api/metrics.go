// ctp_tick/api — /metrics 端点（Prometheus 格式）
package api

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// ============================================================
// Metrics 指标收集器
// ============================================================

// Metrics 服务指标
type Metrics struct {
	TickReceived  atomic.Int64 // tick_received_total
	TickDropped   atomic.Int64 // tick_dropped_total（无效数据）
	RedisWrites   atomic.Int64 // redis_write_total
	RedisFailures atomic.Int64 // redis_write_failure_total
	Reconnects    atomic.Int64 // ctp_reconnect_total
	CTPConnected  atomic.Int64 // ctp_connection_status (0 or 1)
}

// ServeHTTP 暴露 /metrics 端点（Prometheus text 格式）
func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	wWrite := func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(w, format, args...)
	}

	wWrite("# HELP tick_received_total Total ticks received from CTP\n")
	wWrite("# TYPE tick_received_total counter\n")
	wWrite("tick_received_total %d\n\n", m.TickReceived.Load())

	wWrite("# HELP tick_dropped_total Ticks dropped (invalid/malformed)\n")
	wWrite("# TYPE tick_dropped_total counter\n")
	wWrite("tick_dropped_total %d\n\n", m.TickDropped.Load())

	wWrite("# HELP redis_write_total Successful Redis XADD calls\n")
	wWrite("# TYPE redis_write_total counter\n")
	wWrite("redis_write_total %d\n\n", m.RedisWrites.Load())

	wWrite("# HELP redis_write_failure_total Failed Redis XADD calls\n")
	wWrite("# TYPE redis_write_failure_total counter\n")
	wWrite("redis_write_failure_total %d\n\n", m.RedisFailures.Load())

	wWrite("# HELP ctp_reconnect_total CTP reconnect count\n")
	wWrite("# TYPE ctp_reconnect_total counter\n")
	wWrite("ctp_reconnect_total %d\n\n", m.Reconnects.Load())

	wWrite("# HELP ctp_connection_status CTP connection status (1=connected, 0=disconnected)\n")
	wWrite("# TYPE ctp_connection_status gauge\n")
	wWrite("ctp_connection_status %d\n", m.CTPConnected.Load())
}
