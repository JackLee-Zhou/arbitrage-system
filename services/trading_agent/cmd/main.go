// trading_agent — 决策大脑入口
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkgconfig "github.com/arbitrage-system/pkg/config"
	pkgredis "github.com/arbitrage-system/pkg/redis"
	pkgregistry "github.com/arbitrage-system/pkg/registry"
)

// serviceVersion 当前服务版本
const serviceVersion = "0.1.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[trading_agent] starting...")

	cfg := loadConfig()

	// 连接 Redis（使用 pkg/redis 统一入口）
	redisClient := pkgredis.NewAdapter(cfg.RedisAddr)

	// ---- 服务注册 ----
	registryCancel, err := pkgregistry.RegisterAndKeepAlive(
		context.Background(), redisClient, pkgregistry.ServiceInfo{
			Name:         "trading_agent",
			Version:      serviceVersion,
			StartedAt:    time.Now(),
			HealthURL:    ":" + cfg.HealthPort,
			GRPCAddr:     ":" + cfg.GRPCPort,
			Capabilities: []string{"decision", "indicator", "risk"},
		},
	)
	if err != nil {
		log.Printf("[trading_agent] warning: service registration failed: %v (continuing)", err)
	}
	defer func() {
		if registryCancel != nil {
			registryCancel()
		}
	}()

	// 连接 InfluxDB（带重试）
	influxClient := connectInfluxDB(cfg.InfluxDBURL, 60*time.Second)

	// ⭐ 三步对账：恢复状态
	state := reconcileState(redisClient, influxClient, cfg)

	// 启动 gRPC 服务
	go startGRPCServer(cfg.GRPCPort, state)

	// 启动 /healthz + /metrics
	go startHealthServer(cfg.HealthPort)

	// 启动决策引擎主循环
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runEngine(ctx, state, redisClient, cfg)

	log.Println("[trading_agent] all systems ready")

	// 等待退出信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("[trading_agent] shutting down...")
}

type Config struct {
	RedisAddr   string
	InfluxDBURL string
	HealthPort  string
	GRPCPort    string
	Symbol      string
	AIAlpha     float64
}

func loadConfig() Config {
	return Config{
		RedisAddr:   getEnv("REDIS_ADDR", "redis:6379"),
		InfluxDBURL: getEnv("INFLUXDB_URL", "http://influxdb:8086"),
		HealthPort:  getEnv("HEALTH_PORT", "8081"),
		GRPCPort:    getEnv("GRPC_PORT", "9090"),
		Symbol:      getEnv("SYMBOL", "MA2509"),
		AIAlpha:     0.2,
	}
}

func getEnv(key, fallback string) string {
	return pkgconfig.GetEnv(key, fallback)
}

// EngineState 引擎状态（持久化在 Redis）
type EngineState struct {
	Mode       string // "normal" / "safe_mode"
	Positions  interface{}
	Strategies interface{}
}

func connectInfluxDB(url string, timeout time.Duration) interface{} {
	log.Printf("[trading_agent] connecting to InfluxDB at %s...", url)
	return nil
}

func reconcileState(redis, influx interface{}, cfg Config) *EngineState {
	// TODO: 三步对账
	// Step 1: 从 Redis 读最新状态
	// Step 2: 从 InfluxDB 补齐最近 1 小时数据
	// Step 3: CTP 查询真实持仓 → 对账
	return &EngineState{Mode: "normal"}
}

func startGRPCServer(port string, state *EngineState) {
	// TODO: 实现 gRPC 服务
	log.Printf("[trading_agent] gRPC server on :%s", port)
}

func startHealthServer(port string) {
	// TODO: 实现 /healthz + /metrics
	log.Printf("[trading_agent] health server on :%s", port)
}

func runEngine(ctx context.Context, state *EngineState, redis interface{}, cfg Config) {
	// TODO: 主循环 — 订阅 tick → 计算指标 → 跑策略 → 决策融合 → 风控 → 报单
	log.Println("[trading_agent] engine main loop started")
	<-ctx.Done()
}
