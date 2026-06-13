// ctp_tick/adapters/redis — 行情写入（基于 pkg/redis 共享连接）
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

// ============================================================
// 常量
// ============================================================
const (
	maxRetries       = 3
	retryBackoffBase = 500 * time.Millisecond
	writeTimeout     = 3 * time.Second
)

// ============================================================
// TickWriter — 行情写入器
// 依赖 pkg/redis 提供的统一 RedisClient 接口
// ============================================================

// TickWriter 将标准化 Tick 写入 Redis Streams（业务逻辑层）
// Redis 连接由 pkg/redis.NewAdapter 统一创建并注入
type TickWriter struct {
	client pkgredis.RedisClient
}

// NewTickWriter 创建行情写入器
func NewTickWriter(client pkgredis.RedisClient) *TickWriter {
	return &TickWriter{client: client}
}

// WriteTick 双写：tick:<symbol>（业务流 → trading_agent）+ write:tick（持久化流 → db_writer）
func (w *TickWriter) WriteTick(ctx context.Context, tick *domain.StandardTick) error {
	protoTick := tick.ToProto()

	payload, err := json.Marshal(protoTick)
	if err != nil {
		return fmt.Errorf("tick marshal: %w", err)
	}

	values := map[string]interface{}{
		"symbol":    protoTick.Symbol,
		"timestamp": protoTick.Timestamp,
		"price":     protoTick.LastPrice,
		"volume":    protoTick.Volume,
		"data":      string(payload),
	}

	businessStream := fmt.Sprintf("tick:%s", tick.Symbol)
	persistStream := "write:tick"

	// 业务流（失败则报错）
	if err := w.xaddWithRetry(ctx, businessStream, values); err != nil {
		return fmt.Errorf("xadd %s: %w", businessStream, err)
	}

	// 持久化流（失败不阻塞业务）
	if err := w.xaddWithRetry(ctx, persistStream, values); err != nil {
		log.Printf("[redis.writer] xadd %s failed (non-fatal): %v", persistStream, err)
	}

	return nil
}

// xaddWithRetry 指数退避重试
func (w *TickWriter) xaddWithRetry(ctx context.Context, stream string, values map[string]interface{}) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := retryBackoffBase * time.Duration(1<<(attempt-1))
			time.Sleep(backoff)
		}

		writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
		err := w.client.XAdd(writeCtx, stream, values)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("[redis.writer] XADD %s attempt %d/%d: %v", stream, attempt+1, maxRetries, err)
	}
	return lastErr
}

// Ping 检查 Redis 连通性
func (w *TickWriter) Ping(ctx context.Context) error {
	return w.client.Ping(ctx)
}

// Close 关闭连接
func (w *TickWriter) Close() error {
	return w.client.Close()
}
