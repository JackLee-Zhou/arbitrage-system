// orchestrator/api — /healthz 端点
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker 健康检查处理器，注入依赖检查函数
type HealthChecker struct {
	ServiceName string
	Version     string
	StartTime   time.Time
	RedisPing   func(ctx context.Context) error
	TradingPing func(ctx context.Context) error
}

// healthResponse 健康检查 JSON 响应
type healthResponse struct {
	Service string           `json:"service"`
	Version string           `json:"version"`
	Status  string           `json:"status"`
	Uptime  int64            `json:"uptime_sec"`
	Checks  map[string]check `json:"checks"`
}

type check struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// ServeHTTP 实现 http.Handler
func (h *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	overall := "healthy"
	checks := make(map[string]check)

	// Redis 检查
	if h.RedisPing != nil {
		if err := h.RedisPing(ctx); err != nil {
			checks["redis"] = check{Status: "error", Error: err.Error()}
			overall = "degraded"
		} else {
			checks["redis"] = check{Status: "ok"}
		}
	}

	// trading_agent gRPC 检查
	if h.TradingPing != nil {
		if err := h.TradingPing(ctx); err != nil {
			checks["trading_agent"] = check{Status: "error", Error: err.Error()}
			overall = "degraded"
		} else {
			checks["trading_agent"] = check{Status: "ok"}
		}
	}

	resp := healthResponse{
		Service: h.ServiceName,
		Version: h.Version,
		Status:  overall,
		Uptime:  int64(time.Since(h.StartTime).Seconds()),
		Checks:  checks,
	}

	statusCode := http.StatusOK
	if overall != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}
