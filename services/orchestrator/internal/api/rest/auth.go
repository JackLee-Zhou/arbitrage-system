// orchestrator/api/rest — 认证中间件（V2 加）
package rest

import "net/http"

// AuthMiddleware 认证中间件（V1 暂不实现，预留）
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: V2 阶段实现 JWT token 验证
		// V1 阶段：Tailscale 内网访问，暂时不需要认证
		next.ServeHTTP(w, r)
	})
}
