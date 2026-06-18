// trading_agent/api — /healthz HTTP 端点
//
// 返回服务健康状态给 watchdog 和 orchestrator 探测。
// 使用 HealthChecker 模式，通过 CheckFunc 依赖注入实现下游依赖检查。
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// CheckFunc 依赖检查函数，返回 nil 表示正常，非 nil 表示异常
type CheckFunc func(ctx context.Context) error

// CheckResult 单个检查项的结果
type CheckResult struct {
	Status  string `json:"status"` // "ok" / "fail"
	Message string `json:"message,omitempty"`
}

// HealthStatus 健康检查整体响应
type HealthStatus struct {
	Status string                 `json:"status"` // "healthy" / "unhealthy"
	Checks map[string]CheckResult `json:"checks"`
}

// HealthChecker 健康检查处理器，通过注册 CheckFunc 实现依赖注入
type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]CheckFunc),
	}
}

// Register 注册一个依赖检查项
// name 为检查项名称，如 "redis"、"influxdb"、"ctp"
func (h *HealthChecker) Register(name string, fn CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = fn
}

// Handler 返回 http.HandlerFunc，用于挂载到 /healthz
func (h *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		h.mu.RLock()
		checkList := make(map[string]CheckFunc, len(h.checks))
		for k, v := range h.checks {
			checkList[k] = v
		}
		h.mu.RUnlock()

		overall := "healthy"
		results := make(map[string]CheckResult, len(checkList))

		for name, fn := range checkList {
			if err := fn(ctx); err != nil {
				results[name] = CheckResult{
					Status:  "fail",
					Message: err.Error(),
				}
				overall = "unhealthy"
			} else {
				results[name] = CheckResult{Status: "ok"}
			}
		}

		status := HealthStatus{
			Status: overall,
			Checks: results,
		}

		if overall != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(status)
	}
}
