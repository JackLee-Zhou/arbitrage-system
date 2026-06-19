// orchestrator/api — /metrics 端点
package api

import "net/http"

// MetricsHandler Prometheus 指标处理器
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	// TODO: 接入 Prometheus metrics，当前返回空
}
