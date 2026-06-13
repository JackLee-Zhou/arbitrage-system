// ctp_tick/api — /healthz 端点
package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// ============================================================
// 健康检查
// ============================================================

// HealthChecker 健康检查器
type HealthChecker struct {
	CTPConnected   func() bool
	RedisConnected func() bool
	LastTickTime   func() time.Time
	ServiceName    string
	Version        string
	StartTime      time.Time
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Service   string `json:"service"`
	Status    string `json:"status"` // "healthy" / "degraded" / "unhealthy"
	Version   string `json:"version"`
	UptimeSec int64  `json:"uptime_sec"`
	CTP       string `json:"ctp"`
	Redis     string `json:"redis"`
	TickAgeMs int64  `json:"tick_age_ms,omitempty"`
}

// ServeHTTP 实现 http.Handler
func (h *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp := h.Check()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Check 执行健康检查
func (h *HealthChecker) Check() *HealthResponse {
	resp := &HealthResponse{
		Service:   h.ServiceName,
		Version:   h.Version,
		UptimeSec: int64(time.Since(h.StartTime).Seconds()),
	}

	ctpOK := false
	redisOK := false

	if h.CTPConnected != nil {
		ctpOK = h.CTPConnected()
	}
	if h.RedisConnected != nil {
		redisOK = h.RedisConnected()
	}

	resp.CTP = statusStr(ctpOK)
	resp.Redis = statusStr(redisOK)

	if ctpOK && redisOK {
		resp.Status = "healthy"
	} else if ctpOK || redisOK {
		resp.Status = "degraded"
	} else {
		resp.Status = "unhealthy"
	}

	// Tick 数据新鲜度
	if h.LastTickTime != nil {
		lastTick := h.LastTickTime()
		if !lastTick.IsZero() {
			resp.TickAgeMs = time.Since(lastTick).Milliseconds()
		}
	}

	return resp
}

func statusStr(ok bool) string {
	if ok {
		return "ok"
	}
	return "error"
}
