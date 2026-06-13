// orchestrator/api/rest — REST API 中间件
package rest

import "net/http"

// TradingSessionGuard 交易时段守卫中间件
// 拦截所有修改型操作（新增策略/修改风控/启停策略等）
// 紧急平仓例外：任何时段都允许
func TradingSessionGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: 实现交易时段检查
		// if isStrategyMutation(r) && isInTradingSession(time.Now()) {
		//     http.Error(w, "禁止在交易时段修改策略", http.StatusForbidden)
		//     return
		// }
		next.ServeHTTP(w, r)
	})
}
