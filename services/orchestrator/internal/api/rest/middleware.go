// orchestrator/api/rest — REST API 中间件
package rest

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/arbitrage-system/orchestrator/internal/domain"
)

// TradingSessionGuard 交易时段守卫中间件
// 拦截所有修改型操作（新增策略/修改风控/启停策略等）
// 紧急平仓例外：任何时段都允许
// 依赖 domain/trading_calendar.go 的交易日历逻辑
func TradingSessionGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 只读操作直接放行
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// 紧急平仓 = 例外（关乎安全，任何时段都允许）
		if strings.HasPrefix(r.URL.Path, "/api/v1/emergency_close") {
			next.ServeHTTP(w, r)
			return
		}

		// WebSocket 不受限
		if strings.HasPrefix(r.URL.Path, "/ws") {
			next.ServeHTTP(w, r)
			return
		}

		// 检查是否在交易时段
		now := time.Now()
		if domain.IsInTradingSession(now) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			resp := map[string]interface{}{
				"error":   "禁止在交易时段修改策略",
				"next":    domain.NextNonTradingWindow(now),
				"message": "请等待当前交易时段结束再操作",
			}
			_ = json.NewEncoder(w).Encode(resp)
			log.Printf("[trading_guard] blocked %s %s (in trading session)", r.Method, r.URL.Path)
			return
		}

		next.ServeHTTP(w, r)
	})
}
