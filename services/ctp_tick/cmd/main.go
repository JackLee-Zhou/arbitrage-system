// ctp_tick — CTP 行情订阅服务入口
// 装配 domain + adapters + api，启动行情订阅主循环
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/arbitrage-system/ctp_tick/internal/adapters/ctp"
	"github.com/arbitrage-system/ctp_tick/internal/adapters/redis"
	"github.com/arbitrage-system/ctp_tick/internal/api"
	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[ctp_tick] starting...")

	cfg := loadConfig()

	// ---- DI 装配 ----

	// Redis 客户端
	redisAdapter := redis.NewGoRedisAdapter(cfg.RedisAddr)
	tickWriter := redis.NewTickWriter(redisAdapter)

	// CTP 行情订阅（mock 模式 macOS / cgo 模式 Linux）
	mdAPI := ctp.NewMarketDataSubscriber(
		cfg.FlowPath,
		cfg.CTPFrontAddr,
		cfg.CTPBrokerID,
		cfg.CTPUserID,
		cfg.CTPPassword,
	)

	// 指标收集器
	metrics := &api.Metrics{}

	// 最后 Tick 时间（健康检查用）
	var lastTickTime atomic.Pointer[time.Time]
	now := time.Time{}
	lastTickTime.Store(&now)

	// 健康检查
	healthChecker := &api.HealthChecker{
		ServiceName:  cfg.ServiceName,
		Version:      "0.1.0",
		StartTime:    time.Now(),
		CTPConnected: mdAPI.IsConnected,
		RedisConnected: func() bool {
			return tickWriter.Ping(context.Background()) == nil
		},
		LastTickTime: func() time.Time {
			t := lastTickTime.Load()
			if t == nil {
				return time.Time{}
			}
			return *t
		},
	}

	// ---- 注册行情回调 ----
	mdAPI.OnTick(func(raw *ctp.DepthMarketData) {
		go processTick(raw, tickWriter, metrics, &lastTickTime)
	})

	// ---- 启动 HTTP 服务 ----
	mux := http.NewServeMux()
	mux.Handle("/healthz", healthChecker)
	mux.Handle("/metrics", metrics)

	httpServer := &http.Server{
		Addr:    ":" + cfg.HealthPort,
		Handler: mux,
	}
	go func() {
		log.Printf("[ctp_tick] health server on :%s", cfg.HealthPort)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[ctp_tick] HTTP server error: %v", err)
		}
	}()

	// ---- 连接 CTP + 订阅 ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	symbols := cfg.SubscribeSymbols
	log.Printf("[ctp_tick] connecting to %s, symbols=%v", cfg.CTPFrontAddr, symbols)

	if err := mdAPI.Connect(symbols); err != nil {
		log.Printf("[ctp_tick] CTP connect failed: %v (mock mode continues)", err)
	}

	// ---- 等待退出信号 ----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("[ctp_tick] received signal %v, shutting down...", sig)
	case <-ctx.Done():
	}

	// ---- 优雅退出 ----
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	mdAPI.Close()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = tickWriter.Close()

	log.Println("[ctp_tick] shutdown complete")
}

// ============================================================
// Tick 处理管道
// ============================================================

// processTick 行情处理管道：CTP 原始数据 → 标准化 → Redis Stream
func processTick(
	raw *ctp.DepthMarketData,
	writer *redis.TickWriter,
	metrics *api.Metrics,
	lastTickTime *atomic.Pointer[time.Time],
) {
	metrics.TickReceived.Add(1)

	// Step 1: 转换为 domain 层标准结构 + 安全过滤
	tick, err := domain.FromRawMarketData(convertToRaw(raw))
	if err != nil {
		metrics.TickDropped.Add(1)
		return
	}

	// Step 2: 写入 Redis Stream
	ctx := context.Background()
	if err := writer.WriteTick(ctx, tick); err != nil {
		metrics.RedisFailures.Add(1)
		log.Printf("[ctp_tick] write tick failed: %v", err)
		return
	}
	metrics.RedisWrites.Add(1)

	// Step 3: 更新最后 Tick 时间
	now := time.Now()
	lastTickTime.Store(&now)
}

// convertToRaw 将 adapters/ctp.DepthMarketData 转为 domain.RawMarketData
func convertToRaw(raw *ctp.DepthMarketData) *domain.RawMarketData {
	return &domain.RawMarketData{
		TradingDay:         raw.TradingDay,
		InstrumentID:       raw.InstrumentID,
		UpdateTime:         raw.UpdateTime,
		UpdateMillisec:     raw.UpdateMillisec,
		LastPrice:          raw.LastPrice,
		PreSettlementPrice: raw.PreSettlementPrice,
		PreClosePrice:      raw.PreClosePrice,
		OpenPrice:          raw.OpenPrice,
		HighestPrice:       raw.HighestPrice,
		LowestPrice:        raw.LowestPrice,
		Volume:             raw.Volume,
		OpenInterest:       raw.OpenInterest,
		UpperLimitPrice:    raw.UpperLimitPrice,
		LowerLimitPrice:    raw.LowerLimitPrice,
		BidPrice1:          raw.BidPrice1,
		BidVolume1:         raw.BidVolume1,
		AskPrice1:          raw.AskPrice1,
		AskVolume1:         raw.AskVolume1,
	}
}

// ============================================================
// 配置
// ============================================================

type Config struct {
	ServiceName      string
	HealthPort       string
	RedisAddr        string
	CTPFrontAddr     string
	CTPBrokerID      string
	CTPUserID        string
	CTPPassword      string
	FlowPath         string
	SubscribeSymbols []string
}

func loadConfig() Config {
	return Config{
		ServiceName:      getEnv("SERVICE_NAME", "ctp_tick"),
		HealthPort:       getEnv("HEALTH_PORT", "8081"),
		RedisAddr:        getEnv("REDIS_ADDR", "redis:6379"),
		CTPFrontAddr:     getEnv("CTP_FRONT_ADDR", "tcp://180.168.146.187:10130"),
		CTPBrokerID:      getEnv("CTP_BROKER_ID", "9999"),
		CTPUserID:        getEnv("CTP_USER_ID", ""),
		CTPPassword:      getEnv("CTP_PASSWORD", ""),
		FlowPath:         getEnv("CTP_FLOW_PATH", "./flow_md"),
		SubscribeSymbols: parseSymbols(getEnv("SUBSCRIBE_SYMBOLS", "MA2509")),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseSymbols(raw string) []string {
	if raw == "" {
		return nil
	}
	// 逗号分隔的合约列表
	var result []string
	for _, s := range splitTrim(raw, ",") {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func splitTrim(s, sep string) []string {
	var result []string
	current := ""
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}
