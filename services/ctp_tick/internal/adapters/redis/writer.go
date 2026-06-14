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
type TickWriter struct {
	client       pkgredis.RedisClient
	focusSymbols map[string]bool

	tickCh  chan *writeTask
	dropped int64

	batchMu   sync.Mutex
	batchBuf  []map[string]interface{}
	batchLast time.Time

	flushWg sync.WaitGroup // 追踪 flushBatch goroutine
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

type writeTask struct {
	symbol string
	values map[string]interface{}
}

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

// Send 非阻塞发送 Tick 到写入管道。返回 true=入队成功，false=丢弃。
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
		w.dropped++
		if w.dropped%1000 == 0 {
			log.Printf("[redis.writer] WARNING: tickChannel full, dropped %d ticks", w.dropped)
		}
		return false
	}
}

// Close 优雅关闭：停止 writerLoop → 排空 tickCh → 等待 flushBatch 完成
func (w *TickWriter) Close() {
	close(w.stopCh)
	w.wg.Wait()

	// drain 残余 tickCh
	for len(w.tickCh) > 0 {
		task := <-w.tickCh
		w.addToBatch(task.values)
	}

	// 最后 flush
	w.batchMu.Lock()
	if len(w.batchBuf) > 0 {
		w.flushBatch()
	}
	w.batchMu.Unlock()

	// 等待所有 flushBatch goroutine 完成
	w.flushWg.Wait()

	if w.dropped > 0 {
		log.Printf("[redis.writer] total dropped: %d ticks", w.dropped)
	}
}

func (w *TickWriter) Dropped() int64 { return w.dropped }

// ============================================================
// writerLoop
// ============================================================

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
			w.addToBatch(task.values)
			if w.focusSymbols[task.symbol] {
				go w.sendFocus(task.symbol, task.values) // 异步，不阻塞 writerLoop
			}

		case <-batchTicker.C:
			w.batchMu.Lock()
			if len(w.batchBuf) > 0 {
				w.flushBatch()
			}
			w.batchMu.Unlock()
		}
	}
}

func (w *TickWriter) addToBatch(values map[string]interface{}) {
	w.batchMu.Lock()
	defer w.batchMu.Unlock()
	w.batchBuf = append(w.batchBuf, values)
	if len(w.batchBuf) >= ctp.BatchWriteSize {
		w.flushBatch()
	}
}

// flushBatch 批量写入 write:tick（已被 batchMu 保护）
func (w *TickWriter) flushBatch() {
	if len(w.batchBuf) == 0 {
		return
	}
	batch := make([]map[string]interface{}, len(w.batchBuf))
	copy(batch, w.batchBuf)
	w.batchBuf = w.batchBuf[:0]

	w.flushWg.Add(1)
	go func(items []map[string]interface{}) {
		defer w.flushWg.Done()
		ctx := context.Background()
		for _, values := range items {
			if err := w.xaddWithRetry(ctx, "write:tick", values); err != nil {
				log.Printf("[redis.writer] batch XADD write:tick failed: %v", err)
			}
		}
	}(batch)
}

func (w *TickWriter) sendFocus(symbol string, values map[string]interface{}) {
	ctx := context.Background()
	stream := fmt.Sprintf("tick:%s", symbol)
	if err := w.client.XAdd(ctx, stream, values); err != nil {
		log.Printf("[redis.writer] XADD %s failed: %v", stream, err)
	}
}

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
