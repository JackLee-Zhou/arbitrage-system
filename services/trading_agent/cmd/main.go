// trading_agent — 决策大脑入口
//
// 启动流程：
//  1. 加载环境变量配置
//  2. 连接 Redis（带重试）+ 服务注册
//  3. 连接 InfluxDB（带重试）
//  4. ⭐ 三步对账（Redis → InfluxDB → CTP）
//  5. 装配所有依赖（app.NewDependencies）
//  6. 启动 gRPC / health HTTP / 引擎主循环
//  7. 等待信号 → 优雅关闭
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkgconfig "github.com/arbitrage-system/pkg/config"
	pkgredis "github.com/arbitrage-system/pkg/redis"
	pkgregistry "github.com/arbitrage-system/pkg/registry"

	"github.com/arbitrage-system/trading_agent/internal/adapters/influxdb"
	redisadapters "github.com/arbitrage-system/trading_agent/internal/adapters/redis"
	"github.com/arbitrage-system/trading_agent/internal/app"
	"github.com/arbitrage-system/trading_agent/internal/domain/engine"
)

// serviceVersion 当前服务版本
const serviceVersion = "0.1.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[trading_agent] ══════════════════════════════════════")
	log.Println("[trading_agent]  starting up (v" + serviceVersion + ")...")
	log.Println("[trading_agent] ══════════════════════════════════════")

	cfg := loadConfig()

	// ---- 1. 连接 Redis（带重试） ----
	redisClient := connectRedisWithRetry(cfg.RedisAddr, 60*time.Second)

	// ---- 2. 服务注册 ----
	registryCancel, err := pkgregistry.RegisterAndKeepAlive(
		context.Background(), redisClient, pkgregistry.ServiceInfo{
			Name:         "trading_agent",
			Version:      serviceVersion,
			StartedAt:    time.Now(),
			HealthURL:    ":" + cfg.HealthPort,
			GRPCAddr:     ":" + cfg.GRPCPort,
			Capabilities: []string{"decision", "indicator", "risk", "strategy"},
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

	// ---- 3. 连接 InfluxDB（带重试） ----
	influxReader := connectInfluxWithRetry(cfg.InfluxDBURL, cfg.InfluxDBToken, cfg.InfluxDBOrg, 60*time.Second)

	// ---- 4. ⭐ 三步对账 ----
	stateStore := redisadapters.NewStateStore(redisClient)
	recon := reconcileState(cfg, redisClient, stateStore, influxReader)

	// ---- 5. 装配所有依赖 ----
	appCfg := app.Config{
		RedisAddr:       cfg.RedisAddr,
		InfluxDBURL:     cfg.InfluxDBURL,
		InfluxDBToken:   cfg.InfluxDBToken,
		InfluxDBOrg:     cfg.InfluxDBOrg,
		HealthPort:      cfg.HealthPort,
		GRPCPort:        cfg.GRPCPort,
		Symbol:          cfg.Symbol,
		AIAlpha:         cfg.AIAlpha,
		InitialCapital:  cfg.InitialCapital,
		MaxPosition:     cfg.MaxPosition,
		DailyMaxLossPct: cfg.DailyMaxLossPct,
		StopLossPct:     cfg.SingleStopLossPct,
		LLMScorerURL:    cfg.LLMScorerURL,
	}

	deps, err := app.NewDependencies(appCfg, redisClient, influxReader)
	if err != nil {
		log.Fatalf("[trading_agent] FATAL: dependency assembly failed: %v", err)
	}

	// 将对账恢复的持仓载入 engine
	if recon.pos != nil && recon.pos.Volume > 0 {
		deps.Engine.LoadReconciledPosition(recon.pos.Symbol, recon.pos.Direction, recon.pos.Volume, recon.pos.AvgPrice)
		log.Printf("[trading_agent] ✅ 对账持仓已载入: %s %s %d手 @%.2f",
			recon.pos.Symbol, recon.pos.Direction, recon.pos.Volume, recon.pos.AvgPrice)
	}

	// 如果对账发现不一致，进 SafeMode
	if recon.mode == "safe_mode" {
		log.Println("[trading_agent] ⚠️  entering SAFE MODE (reconciliation mismatch)")
		deps.Engine.SetMode(engine.ModeSafe)
	}

	// ---- 6. 启动各服务 ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// gRPC 服务
	go func() {
		if err := deps.GRPCServer.Start(); err != nil {
			log.Printf("[trading_agent] gRPC server error: %v", err)
		}
	}()

	// Health HTTP 服务
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", deps.HealthChecker.Handler())
	healthMux.HandleFunc("/metrics", deps.Metrics.ServeHTTP)
	healthServer := &http.Server{
		Addr:    ":" + cfg.HealthPort,
		Handler: healthMux,
	}
	go func() {
		log.Printf("[trading_agent] health server listening on :%s", cfg.HealthPort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[trading_agent] health server error: %v", err)
		}
	}()

	// 引擎主循环 — 桥接 TickSubscriber(推模式) 和 Engine.ProcessTick
	go func() {
		log.Printf("[trading_agent] engine loop starting, symbol=%s mode=%s", cfg.Symbol, recon.mode)
		if err := deps.TickSub.Subscribe(ctx, cfg.Symbol, func(tick redisadapters.Tick) {
			engTick := engine.Tick{
				Symbol:    tick.Symbol,
				Timestamp: tick.Timestamp,
				LastPrice: tick.LastPrice,
				Volume:    tick.Volume,
			}
			_ = deps.Engine.ProcessTick(engTick)
		}); err != nil && err != context.Canceled {
			log.Printf("[trading_agent] engine subscribe exited: %v", err)
		}
	}()

	log.Println("[trading_agent] ══════════════════════════════════════")
	log.Printf("[trading_agent]  all systems ready  mode=%s  symbol=%s", recon.mode, cfg.Symbol)
	log.Println("[trading_agent] ══════════════════════════════════════")

	// ---- 7. 等待退出信号 ----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("[trading_agent] received signal %v, shutting down...", sig)

	// ---- 8. 优雅关闭 ----
	cancel() // 通知引擎停止
	log.Println("[trading_agent] stopping gRPC server...")
	deps.GRPCServer.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[trading_agent] health server shutdown error: %v", err)
	}

	if influxReader != nil {
		influxReader.Close()
	}

	log.Println("[trading_agent] shutdown complete")
}

// ============================================================
// 配置
// ============================================================

// Config 服务配置（从环境变量加载）
type Config struct {
	RedisAddr         string
	InfluxDBURL       string
	InfluxDBToken     string
	InfluxDBOrg       string
	HealthPort        string
	GRPCPort          string
	Symbol            string
	AIAlpha           float64
	InitialCapital    float64
	MaxPosition       int
	DailyMaxLossPct   float64
	SingleStopLossPct float64
	LLMScorerURL      string
}

func loadConfig() Config {
	return Config{
		RedisAddr:         pkgconfig.GetEnv("REDIS_ADDR", "redis:6379"),
		InfluxDBURL:       pkgconfig.GetEnv("INFLUXDB_URL", "http://influxdb:8086"),
		InfluxDBToken:     pkgconfig.GetEnv("INFLUXDB_TOKEN", ""),
		InfluxDBOrg:       pkgconfig.GetEnv("INFLUXDB_ORG", "arbitrage"),
		HealthPort:        pkgconfig.GetEnv("HEALTH_PORT", "8081"),
		GRPCPort:          pkgconfig.GetEnv("GRPC_PORT", "9090"),
		Symbol:            pkgconfig.GetEnv("SYMBOL", "MA2509"),
		AIAlpha:           getEnvFloat("AI_ALPHA", 0.2),
		InitialCapital:    getEnvFloat("INITIAL_CAPITAL", 100000),
		MaxPosition:       getEnvInt("MAX_POSITION", 2),
		DailyMaxLossPct:   getEnvFloat("DAILY_MAX_LOSS_PCT", 0.02),
		SingleStopLossPct: getEnvFloat("SINGLE_STOP_LOSS_PCT", 0.005),
		LLMScorerURL:      pkgconfig.GetEnv("LLM_SCORER_URL", "http://llm_scorer:5000"),
	}
}

// ============================================================
// 三步对账（机制 3）
// ============================================================

// reconciledState 对账结果
type reconciledState struct {
	mode string
	pos  *redisadapters.Position // 恢复的持仓（Redis 或 InfluxDB）
}

// reconcileState 执行三步对账，返回建议的引擎模式和恢复的持仓数据。
//
// Step 1: 从 Redis 读最新状态
// Step 2: 如果 Redis 无数据，从 InfluxDB 补齐最近 1h
// Step 3: CTP 查询真实持仓 → 与本地对账（V1 跳过，等 CTP adapter 就绪）
//
// 核心原则：CTP 查询是黄金真相，本地状态只是缓存。
func reconcileState(cfg Config, redisClient *pkgredis.GoRedisAdapter, stateStore *redisadapters.StateStore, influxReader *influxdb.InfluxReader) reconciledState {
	log.Println("[reconcile] ═══ 三步对账开始 ═══")

	ctx := context.Background()
	result := reconciledState{mode: "normal"}

	// ---- Step 1: 从 Redis 读最新状态 ----
	log.Println("[reconcile] Step 1/3: 从 Redis 加载状态...")
	pos, err := stateStore.GetPosition(ctx, cfg.Symbol)
	if err != nil {
		log.Printf("[reconcile] Step 1/3: Redis 读取失败: %v (非关键，继续)", err)
	} else if pos != nil && pos.Volume > 0 {
		log.Printf("[reconcile] Step 1/3: ✅ Redis 中有持仓: %s %s %d手 @%.2f",
			pos.Symbol, pos.Direction, pos.Volume, pos.AvgPrice)
		result.pos = pos
	} else {
		log.Println("[reconcile] Step 1/3: Redis 中无持仓数据")
	}

	// ---- Step 2: InfluxDB 兜底（Redis 无数据时） ----
	if result.pos == nil {
		log.Println("[reconcile] Step 2/3: Redis 无持仓，尝试从 InfluxDB 补齐...")
		if influxReader != nil {
			snapshot, err := influxReader.QueryLatestPosition(ctx, cfg.Symbol)
			if err != nil {
				log.Printf("[reconcile] Step 2/3: InfluxDB 查询失败: %v", err)
			} else if snapshot != nil && snapshot.Volume > 0 {
				log.Printf("[reconcile] Step 2/3: ✅ InfluxDB 补齐成功: %s %d手 @%.2f",
					snapshot.Direction, snapshot.Volume, snapshot.AvgPrice)
				result.pos = &redisadapters.Position{
					Symbol:    snapshot.Symbol,
					Direction: snapshot.Direction,
					Volume:    snapshot.Volume,
					AvgPrice:  snapshot.AvgPrice,
				}
			} else {
				log.Println("[reconcile] Step 2/3: InfluxDB 中也无数据（可能新部署）")
			}
		} else {
			log.Println("[reconcile] Step 2/3: 跳过（InfluxDB 未配置）")
		}
	} else {
		log.Println("[reconcile] Step 2/3: 跳过（Redis 已有数据）")
	}

	// ---- Step 3: CTP 真实持仓对账 ----
	// V1: CTP adapter 尚未实现，暂时跳过。
	ctpFront := pkgconfig.GetEnv("CTP_FRONT_ADDR", "")
	if ctpFront == "" {
		log.Println("[reconcile] Step 3/3: 跳过（CTP 未配置，本地 mock 模式）")
	} else {
		log.Println("[reconcile] Step 3/3: ⏳ CTP 对账待实现（CTP adapter 尚未就绪）")
	}

	log.Printf("[reconcile] ═══ 三步对账完成，建议模式: %s ═══", result.mode)
	return result
}

// ============================================================
// 连接辅助函数
// ============================================================

// connectRedisWithRetry 连接 Redis，指数退避重试直到超时。
func connectRedisWithRetry(addr string, timeout time.Duration) *pkgredis.GoRedisAdapter {
	deadline := time.Now().Add(timeout)
	delay := 1 * time.Second
	for time.Now().Before(deadline) {
		client := pkgredis.NewAdapter(addr)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := client.Ping(ctx)
		cancel()
		if err == nil {
			log.Printf("[trading_agent] Redis connected: %s", addr)
			return client
		}
		log.Printf("[trading_agent] waiting for Redis at %s (retry in %v)...", addr, delay)
		time.Sleep(delay)
		delay *= 2
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}
	}
	log.Fatalf("[trading_agent] FATAL: cannot connect to Redis at %s after %v", addr, timeout)
	return nil
}

// connectInfluxWithRetry 连接 InfluxDB，指数退避重试直到超时。
func connectInfluxWithRetry(url, token, org string, timeout time.Duration) *influxdb.InfluxReader {
	if url == "" || token == "" {
		log.Println("[trading_agent] InfluxDB not configured, skip")
		return nil
	}

	deadline := time.Now().Add(timeout)
	delay := 1 * time.Second
	for time.Now().Before(deadline) {
		reader := influxdb.NewInfluxReader(url, token, org)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := reader.Ping(ctx)
		cancel()
		if err == nil {
			log.Printf("[trading_agent] InfluxDB connected: %s", url)
			return reader
		}
		log.Printf("[trading_agent] waiting for InfluxDB at %s (retry in %v)...", url, delay)
		time.Sleep(delay)
		delay *= 2
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}
	}
	log.Printf("[trading_agent] WARNING: cannot connect to InfluxDB at %s after %v (continuing)", url, timeout)
	return nil
}

// ============================================================
// 工具函数
// ============================================================

func getEnvFloat(key string, fallback float64) float64 {
	s := pkgconfig.GetEnv(key, "")
	if s == "" {
		return fallback
	}
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		log.Printf("[trading_agent] WARNING: invalid float for %s=%q, using default %.4f", key, s, fallback)
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	s := pkgconfig.GetEnv(key, "")
	if s == "" {
		return fallback
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		log.Printf("[trading_agent] WARNING: invalid int for %s=%q, using default %d", key, s, fallback)
		return fallback
	}
	return v
}
