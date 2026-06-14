// db_writer/api -- /healthz 健康检查端点
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ============================================================
// 健康检查
// ============================================================

// HealthCheckFuncs 健康检查依赖的函数
type HealthCheckFuncs struct {
	RedisPing    func(ctx context.Context) error
	InfluxDBPing func(ctx context.Context) error
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Service   string                       `json:"service"`
	Version   string                       `json:"version"`
	StartTime string                       `json:"start_time"`
	UptimeSec int64                        `json:"uptime_sec"`
	Status    string                       `json:"status"`
	Checks    map[string]HealthCheckResult `json:"checks"`
}

// HealthCheckResult 单项检查结果
type HealthCheckResult struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
}

// HealthHandler 健康检查处理器
type HealthHandler struct {
	ServiceName string
	Version     string
	StartTime   time.Time
	Funcs       HealthCheckFuncs
}

// ServeHTTP 实现 http.Handler
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := h.Check(r.Context())
	w.Header().Set("Content-Type", "application/json")

	switch resp.Status {
	case "healthy", "degraded":
		w.WriteHeader(http.StatusOK)
	case "unhealthy":
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		w.WriteHeader(http.StatusOK)
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// Check 执行健康检查
func (h *HealthHandler) Check(ctx context.Context) HealthResponse {
	resp := HealthResponse{
		Service:   h.ServiceName,
		Version:   h.Version,
		StartTime: h.StartTime.Format(time.RFC3339),
		UptimeSec: int64(time.Since(h.StartTime).Seconds()),
		Checks:    make(map[string]HealthCheckResult),
	}

	redisOK := false
	influxOK := false

	if h.Funcs.RedisPing != nil {
		start := time.Now()
		err := h.Funcs.RedisPing(ctx)
		latency := time.Since(start).Milliseconds()
		redisOK = (err == nil)
		status := "ok"
		if !redisOK {
			status = "error"
		}
		resp.Checks["redis"] = HealthCheckResult{Status: status, LatencyMs: latency}
	} else {
		resp.Checks["redis"] = HealthCheckResult{Status: "not_configured", LatencyMs: 0}
	}

	if h.Funcs.InfluxDBPing != nil {
		start := time.Now()
		err := h.Funcs.InfluxDBPing(ctx)
		latency := time.Since(start).Milliseconds()
		influxOK = (err == nil)
		status := "ok"
		if !influxOK {
			status = "error"
		}
		resp.Checks["influxdb"] = HealthCheckResult{Status: status, LatencyMs: latency}
	} else {
		resp.Checks["influxdb"] = HealthCheckResult{Status: "not_configured", LatencyMs: 0}
	}

	switch {
	case redisOK && influxOK:
		resp.Status = "healthy"
	case redisOK || influxOK:
		resp.Status = "degraded"
	default:
		resp.Status = "unhealthy"
	}

	return resp
}
