// ctp_tick — CTP 行情订阅服务入口
// 启动时查询全市场期货合约 → 订阅所有 → 异步批量落库 + 关注合约近实时推送
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

	pkgredis "github.com/arbitrage-system/pkg/redis"

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

	// Redis（pkg/redis 统一连接）
	redisClient := pkgredis.NewAdapter(cfg.RedisAddr)

	// 异步行情写入器（内部启动 writerLoop goroutine）
	tickWriter := redis.NewTickWriter(redisClient, cfg.SubscribeSymbols)

	// 合约查询器（TD API — Mock 或 cgo 真实现）
	contractQuerier := ctp.NewContractQuerier()

	// 行情订阅器（MD API — Mock 或 cgo 真实现）
	mdAPI := ctp.NewMarketDataSubscriber(
		cfg.FlowPath, cfg.CTPFrontAddr, cfg.CTPBrokerID, cfg.CTPUserID, cfg.CTPPassword,
	)

	// 指标收集
	metrics := &api.Metrics{}
	var lastTickTime atomic.Pointer[time.Time]
	now := time.Time{}
	lastTickTime.Store(&now)

	// 健康检查
	healthChecker := &api.HealthChecker{
		ServiceName:  cfg.ServiceName,
		Version:      "0.2.0",
		StartTime:    time.Now(),
		CTPConnected: mdAPI.IsConnected,
		RedisConnected: func() bool {
			return redisClient.Ping(context.Background()) == nil
		},
		LastTickTime: func() time.Time {
			t := lastTickTime.Load()
			if t == nil {
				return time.Time{}
			}
			return *t
		},
	}

	// ---- HTTP 服务 ----
	mux := http.NewServeMux()
	mux.Handle("/healthz", healthChecker)
	mux.Handle("/metrics", metrics)

	httpServer := &http.Server{
		Addr:         ":" + cfg.HealthPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		log.Printf("[ctp_tick] health server on :%s", cfg.HealthPort)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[ctp_tick] HTTP server error: %v", err)
		}
	}()

	// ---- Step 1: 查询全市场期货合约 ----
	allSymbols, err := contractQuerier.QueryAllFutures(
		cfg.CTPFrontAddr, cfg.CTPBrokerID, cfg.CTPUserID, cfg.CTPPassword,
	)
	if err != nil {
		log.Printf("[ctp_tick] TD query failed: %v — falling back to focus symbols only", err)
		allSymbols = cfg.SubscribeSymbols
	}
	log.Printf("[ctp_tick] total %d futures to subscribe, focus=%v", len(allSymbols), cfg.SubscribeSymbols)

	// ---- Step 2: 注册行情回调（绝对不能阻塞 CTP 回调线程）----
	mdAPI.OnTick(func(raw *ctp.DepthMarketData) {
		metrics.TickReceived.Add(1)

		// 转 domain 层标准化 + 安全过滤
		tick, err := domain.FromRawMarketData(convertToRaw(raw))
		if err != nil {
			metrics.TickDropped.Add(1)
			return
		}

		// 非阻塞发送到 writer goroutine —— 绝对不能阻塞
		if tickWriter.Send(tick.Symbol, tick) {
			metrics.RedisWrites.Add(1)
		} else {
			metrics.RedisFailures.Add(1)
		}

		// 更新最后 Tick 时间
		now := time.Now()
		lastTickTime.Store(&now)
	})

	// ---- Step 3: 连接 CTP + 订阅全市场合约 ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("[ctp_tick] connecting to %s, subscribing %d symbols...", cfg.CTPFrontAddr, len(allSymbols))
	if err := mdAPI.Connect(allSymbols); err != nil {
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
	tickWriter.Close() // 等待 writerLoop 结束 + flush 残余数据
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ctp_tick] HTTP shutdown timeout, force exit")
	}
	_ = redisClient.Close()

	log.Printf("[ctp_tick] shutdown complete (received=%d dropped=%d)",
		metrics.TickReceived.Load(), metrics.TickDropped.Load())
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

		BidPrice2:  raw.BidPrice2,
		BidVolume2: raw.BidVolume2,
		BidPrice3:  raw.BidPrice3,
		BidVolume3: raw.BidVolume3,
		BidPrice4:  raw.BidPrice4,
		BidVolume4: raw.BidVolume4,
		BidPrice5:  raw.BidPrice5,
		BidVolume5: raw.BidVolume5,
		AskPrice2:  raw.AskPrice2,
		AskVolume2: raw.AskVolume2,
		AskPrice3:  raw.AskPrice3,
		AskVolume3: raw.AskVolume3,
		AskPrice4:  raw.AskPrice4,
		AskVolume4: raw.AskVolume4,
		AskPrice5:  raw.AskPrice5,
		AskVolume5: raw.AskVolume5,
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
	SubscribeSymbols []string // 关注合约（走 tick:<symbol> + write:tick）；其他仅 write:tick
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
