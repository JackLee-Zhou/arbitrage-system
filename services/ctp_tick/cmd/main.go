// ctp_tick — CTP 行情订阅服务入口
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[ctp_tick] starting...")

	cfg := loadConfig()

	// 连接 Redis
	redisClient := connectWithRetry(cfg.RedisAddr, 60*time.Second)
	defer func() {
		if redisClient != nil {
			_ = redisClient.Close()
		}
	}()

	// 启动 /healthz + /metrics
	go startHealthServer(cfg.HealthPort)

	// 启动行情订阅（V1: CTP mock 模式）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go subscribeMarketData(ctx, redisClient, cfg)

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("[ctp_tick] shutting down...")
}

type Config struct {
	RedisAddr  string
	HealthPort string
}

func loadConfig() Config {
	return Config{
		RedisAddr:  getEnv("REDIS_ADDR", "redis:6379"),
		HealthPort: getEnv("HEALTH_PORT", "8081"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// RedisClient 抽象 Redis 连接（V1 用 go-redis 实现）
type RedisClient struct {
	// TODO: 内嵌 *redis.Client
}

func (c *RedisClient) Close() error {
	// TODO: 实现 go-redis 连接关闭
	return nil
}

func connectWithRetry(addr string, timeout time.Duration) *RedisClient {
	// TODO: 实现 go-redis 客户端连接 + 重试
	log.Printf("[ctp_tick] connecting to Redis at %s...", addr)
	return &RedisClient{}
}

func startHealthServer(port string) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"ctp_tick","status":"healthy"}`))
	})
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("# placeholder metrics\n"))
	})
	log.Printf("[ctp_tick] health server on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func subscribeMarketData(ctx context.Context, client *RedisClient, cfg Config) {
	// TODO: 实现 CTP 行情订阅 + Tick 标准化 + 写 Redis Streams
	log.Println("[ctp_tick] market data subscription started (mock mode)")
	<-ctx.Done()
}
