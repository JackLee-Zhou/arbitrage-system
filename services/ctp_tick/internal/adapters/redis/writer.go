// ctp_tick/adapters/redis — 行情写入管道（基于 pkg/redis 共享连接）
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"

	"github.com/arbitrage-system/ctp_tick/internal/adapters/ctp"
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
// TickWriter — 异步行情写入器
// ============================================================

// TickWriter 异步行情写入管道
// 从 CTP 回调线程接收 Tick（非阻塞 send），在独立 goroutine 中消费
//
//	CTP 回调 → tickChannel (非阻塞) → writerLoop (单 goroutine)
//	                                     ├→ batchFlush: write:tick (100条/1s)
//	                                     └→ focusSend:  tick:<symbol> (逐笔)
type TickWriter struct {
	client       pkgredis.RedisClient
	focusSymbols map[string]bool // 关注合约集合（SUBSCRIBE_SYMBOLS）

	tickCh  chan *writeTask // 内部缓冲通道（非阻塞 send）
	dropped int64           // 通道满丢弃计数

	batchMu   sync.Mutex
	batchBuf  []map[string]interface{} // 攒批缓冲区
	batchLast time.Time

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// writeTask 写入任务
type writeTask struct {
	symbol string
	values map[string]interface{}
}

// NewTickWriter 创建异步行情写入器
// tickChannel: 来自 CTP 回调的入口（外部创建，注入）
func NewTickWriter(client pkgredis.RedisClient, focusSymbols []string) *TickWriter {
	fs := make(map[string]bool, len(focusSymbols))
	for _, s := range focusSymbols {
		fs[s] = true
	}

	w := &TickWriter{
		client:       client,
		focusSymbols: fs,
		tickCh:       make(chan *writeTask, ctp.TickChannelBufferSize),
		batchBuf:     make([]map[string]interface{}, 0, ctp.BatchWriteSize),
		batchLast:    time.Now(),
		stopCh:       make(chan struct{}),
	}

	if len(fs) > 0 {
		log.Printf("[redis.writer] focus symbols: %v", focusSymbols)
	}
	log.Printf("[redis.writer] batch: %d items / %v", ctp.BatchWriteSize, ctp.BatchWriteInterval)

	w.wg.Add(1)
	go w.writerLoop()

	return w
}

// Send 非阻塞发送 Tick 到写入管道
// 由 CTP 回调线程调用，绝对不能阻塞
// 返回 true 表示成功入队，false 表示通道满已丢弃
func (w *TickWriter) Send(symbol string, tick *domain.StandardTick) bool {
	protoTick := tick.ToProto()

	payload, err := json.Marshal(protoTick)
	if err != nil {
		return false
	}

	values := map[string]interface{}{
		"symbol":    protoTick.Symbol,
		"timestamp": protoTick.Timestamp,
		"price":     protoTick.LastPrice,
		"volume":    protoTick.Volume,
		"data":      string(payload),
	}

	task := &writeTask{symbol: symbol, values: values}

	select {
	case w.tickCh <- task:
		return true
	default:
		// 通道满，丢弃（不阻塞 CTP 回调）
		w.dropped++
		if w.dropped%1000 == 0 {
			log.Printf("[redis.writer] WARNING: tickChannel full, dropped %d ticks", w.dropped)
		}
		return false
	}
}

// Close 优雅关闭写入器
func (w *TickWriter) Close() {
	close(w.stopCh)
	w.wg.Wait()

	// 最后 flush 一次残余数据
	w.batchMu.Lock()
	if len(w.batchBuf) > 0 {
		w.flushBatch()
	}
	w.batchMu.Unlock()

	if w.dropped > 0 {
		log.Printf("[redis.writer] total dropped: %d ticks", w.dropped)
	}
}

// Dropped 返回丢弃计数
func (w *TickWriter) Dropped() int64 {
	return w.dropped
}

// ============================================================
// 内部：writer goroutine
// ============================================================

// writerLoop 单 goroutine 消费 tickChannel
// 批量写入 write:tick（100条/1s），近实时写入 tick:<symbol>
func (w *TickWriter) writerLoop() {
	defer w.wg.Done()
	log.Printf("[redis.writer] writerLoop started")

	batchTicker := time.NewTicker(ctp.BatchWriteInterval)
	defer batchTicker.Stop()

	for {
		select {
		case <-w.stopCh:
			return

		case task := <-w.tickCh:
			// 全部合约 → 攒批 write:tick
			w.addToBatch(task.values)

			// 关注合约 → 逐笔近实时 XADD tick:<symbol>
			if w.focusSymbols[task.symbol] {
				w.sendFocus(task.symbol, task.values)
			}

		case <-batchTicker.C:
			// 定时 flush（1秒）
			w.batchMu.Lock()
			if len(w.batchBuf) > 0 {
				w.flushBatch()
			}
			w.batchMu.Unlock()
		}
	}
}

// addToBatch 将数据加入批处理缓冲区
func (w *TickWriter) addToBatch(values map[string]interface{}) {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()

	w.batchBuf = append(w.batchBuf, values)

	// 达到阈值 → flush
	if len(w.batchBuf) >= ctp.BatchWriteSize {
		w.flushBatch()
	}
}

// flushBatch 批量写入 write:tick（调用方已持有 batchMu）
func (w *TickWriter) flushBatch() {
	if len(w.batchBuf) == 0 {
		return
	}
	batch := make([]map[string]interface{}, len(w.batchBuf))
	copy(batch, w.batchBuf)
	w.batchBuf = w.batchBuf[:0]

	// 异步批量写入，不阻塞 writerLoop
	go func(items []map[string]interface{}) {
		ctx := context.Background()
		for _, values := range items {
			if err := w.xaddWithRetry(ctx, "write:tick", values); err != nil {
				log.Printf("[redis.writer] batch XADD write:tick failed: %v", err)
			}
		}
	}(batch)
}

// sendFocus 近实时写入 tick:<symbol>（关注合约独有）
func (w *TickWriter) sendFocus(symbol string, values map[string]interface{}) {
	ctx := context.Background()
	stream := fmt.Sprintf("tick:%s", symbol)
	if err := w.client.XAdd(ctx, stream, values); err != nil {
		log.Printf("[redis.writer] XADD %s failed: %v", stream, err)
	}
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
	}
	return lastErr
}
