// orchestrator — 编排服务入口
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
	pkgregistry "github.com/arbitrage-system/pkg/registry"
)

// serviceVersion 当前服务版本
const serviceVersion = "0.1.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[orchestrator] starting...")

	cfg := loadConfig()

	// ---- 连接 Redis（用于服务注册等）----
	redisClient := pkgredis.NewAdapter(cfg.RedisAddr)
	log.Printf("[orchestrator] redis connected to %s", cfg.RedisAddr)

	// ---- 服务注册 ----
	registryCancel, err := pkgregistry.RegisterAndKeepAlive(
		context.Background(), redisClient, pkgregistry.ServiceInfo{
			Name:       "orchestrator",
			Version:    serviceVersion,
			StartedAt:  time.Now(),
			HealthURL:  ":" + cfg.HTTPPort,
			Capabilities: []string{"rest_api", "websocket", "grpc_gateway"},
		},
	)
	if err != nil {
		log.Printf("[orchestrator] warning: service registration failed: %v (continuing)", err)
	}
	defer func() {
		if registryCancel != nil {
			registryCancel()
		}
	}()

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

	// 启动 HTTP 服务
	addr := ":" + cfg.HTTPPort
	httpServer := &http.Server{
		Addr:    addr,
		Handler: withMiddleware(mux),
	}
	go func() {
		log.Printf("[orchestrator] listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[orchestrator] HTTP server error: %v", err)
		}
	}()

	// ---- 等待退出信号 ----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[orchestrator] received signal %v, shutting down...", sig)

	// ---- 优雅关停 ----
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[orchestrator] HTTP shutdown timeout, force exit")
	}

	log.Println("[orchestrator] shutdown complete")
}

type Config struct {
	HTTPPort  string
	RedisAddr string
}

func loadConfig() Config {
	return Config{
		HTTPPort:  getEnv("HTTP_PORT", "8080"),
		RedisAddr: getEnv("REDIS_ADDR", "redis:6379"),
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
	_, _ = w.Write([]byte(`{"status":"ok","service":"orchestrator"}`))
}

func handlePositions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"positions":[]}`))
}

func handleEmergencyClose(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"success":true,"message":"emergency_close_requested"}`))
}

func handleStrategies(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"strategies":[]}`))
}

func handleRisk(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"risk_params":{}}`))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现 WebSocket 升级 + 实时状态推送
	_, _ = w.Write([]byte("websocket endpoint"))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"service":"orchestrator","status":"healthy"}`))
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("# placeholder metrics\n"))
}

// withMiddleware 应用中间件（日志、交易时段守卫等）
func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[orchestrator] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
