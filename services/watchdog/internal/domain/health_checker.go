// watchdog/domain — 健康判定规则（纯逻辑）
package domain

// HealthStatus 健康状态
type HealthStatus struct {
	Service string `json:"service"`
	Status  string `json:"status"` // "healthy" / "degraded" / "unhealthy"
	Latency int64  `json:"latency_ms"`
}

// IsUnhealthy 判断是否需要告警
func IsUnhealthy(s HealthStatus) bool {
	return s.Status == "unhealthy"
}
