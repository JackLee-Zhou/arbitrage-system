// trading_agent/api — /metrics 端点
package api

import "net/http"

// MetricsHandler Prometheus 指标处理器
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("# placeholder trading_agent metrics\n"))
}
