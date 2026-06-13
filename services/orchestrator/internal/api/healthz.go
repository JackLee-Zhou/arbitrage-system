// orchestrator/api — /healthz 端点
package api

import "net/http"

// HealthzHandler 健康检查处理器
func HealthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"service":"orchestrator","status":"healthy"}`))
}
