// ctp_tick/adapters/redis — Redis Streams 写入适配器
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/arbitrage-system/ctp_tick/internal/domain"
)

// ============================================================
// 常量
// ============================================================
const (
	MaxRetries       = 3
	RetryBackoffBase = 500 * time.Millisecond
	StreamMaxLen     = 10000 // 保留最近 10000 条
	WriteTimeout     = 3 * time.Second
)

// ============================================================
// TickWriter — 将 Tick 写入 Redis Streams
// ============================================================

// TickWriter 行情写入器
// 负责 XADD tick:<symbol>（业务流）+ write:tick（持久化流 → db_writer）
type TickWriter struct {
	client RedisClient
}

// RedisClient Redis 客户端接口（解耦 go-redis 具体实现）
type RedisClient interface {
	XAdd(ctx context.Context, stream string, values map[string]interface{}) error
	Ping(ctx context.Context) error
	Close() error
}

// NewTickWriter 创建行情写入器
func NewTickWriter(client RedisClient) *TickWriter {
	return &TickWriter{client: client}
}

// WriteTick 将标准化 Tick 写入 Redis Stream
// 双写：tick:<symbol>（业务流）+ write:tick（持久化流）
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

	// 写入业务流（trading_agent 消费）
	if err := w.xaddWithRetry(ctx, businessStream, values); err != nil {
		return fmt.Errorf("xadd %s: %w", businessStream, err)
	}

	// 写入持久化流（db_writer 消费 → InfluxDB）
	if err := w.xaddWithRetry(ctx, persistStream, values); err != nil {
		// 持久化流失败不阻塞业务
		log.Printf("[redis.writer] xadd %s failed (non-fatal): %v", persistStream, err)
	}

	return nil
}

// xaddWithRetry 带重试的 XADD
func (w *TickWriter) xaddWithRetry(ctx context.Context, stream string, values map[string]interface{}) error {
	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := RetryBackoffBase * time.Duration(1<<(attempt-1))
			time.Sleep(backoff)
		}

		writeCtx, cancel := context.WithTimeout(ctx, WriteTimeout)
		err := w.client.XAdd(writeCtx, stream, values)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("[redis.writer] XADD %s attempt %d/%d: %v", stream, attempt+1, MaxRetries, err)
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

// ============================================================
// GoRedisClient — go-redis 适配器（实现 RedisClient 接口）
// ============================================================

// GoRedisAdapter 将 go-redis 客户端适配为 RedisClient 接口
// 等 go-redis 加到 go.mod 后启用
type GoRedisAdapter struct {
	// TODO: 内嵌 *redis.Client
	addr string
}

// NewGoRedisAdapter 创建 go-redis 适配器
func NewGoRedisAdapter(addr string) *GoRedisAdapter {
	return &GoRedisAdapter{addr: addr}
}

// XAdd go-redis 实现
func (a *GoRedisAdapter) XAdd(ctx context.Context, stream string, values map[string]interface{}) error {
	// TODO: 使用 go-redis client.XAdd(ctx, &redis.XAddArgs{...})
	log.Printf("[redis] XADD %s (mock)", stream)
	return nil
}

// Ping go-redis 实现
func (a *GoRedisAdapter) Ping(ctx context.Context) error {
	log.Printf("[redis] PING (mock)")
	return nil
}

// Close go-redis 实现
func (a *GoRedisAdapter) Close() error {
	log.Printf("[redis] close (mock)")
	return nil
}
