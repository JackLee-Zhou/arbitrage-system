// orchestrator — 编排服务入口
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[orchestrator] starting...")

	cfg := loadConfig()

	// 注册 REST API 路由
	mux := http.NewServeMux()

	// API v1
	mux.HandleFunc("/api/v1/overview", handleOverview)
	mux.HandleFunc("/api/v1/positions", handlePositions)
	mux.HandleFunc("/api/v1/emergency_close", handleEmergencyClose)
	mux.HandleFunc("/api/v1/strategies", handleStrategies)
	mux.HandleFunc("/api/v1/risk", handleRisk)

	// WebSocket 实时推送
	mux.HandleFunc("/ws", handleWebSocket)

	// 健康检查 + 指标
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/metrics", handleMetrics)

	// 启动服务
	addr := ":" + cfg.HTTPPort
	log.Printf("[orchestrator] listening on %s", addr)
	if err := http.ListenAndServe(addr, withMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

type Config struct {
	HTTPPort string
}

func loadConfig() Config {
	return Config{
		HTTPPort: getEnv("HTTP_PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --- 路由处理器（占位） ---

func handleOverview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","service":"orchestrator"}`))
}

func handlePositions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"positions":[]}`))
}

func handleEmergencyClose(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true,"message":"emergency_close_requested"}`))
}

func handleStrategies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"strategies":[]}`))
}

func handleRisk(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"risk_params":{}}`))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现 WebSocket 升级 + 实时状态推送
	w.Write([]byte("websocket endpoint"))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"service":"orchestrator","status":"healthy"}`))
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("# placeholder metrics\n"))
}

// withMiddleware 应用中间件（日志、交易时段守卫等）
func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[orchestrator] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
