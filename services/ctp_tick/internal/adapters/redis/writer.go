// ctp_tick/adapters/redis — 行情写入管道（基于 pkg/redis 共享连接）
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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

	// InfluxDB Line Protocol（供 db_writer 直接消费）
	lp := formatLineProtocol(protoTick)

	// 完整 JSON（供调试和其他消费者）
	payload, err := json.Marshal(protoTick)
	if err != nil {
		return false
	}

	values := map[string]interface{}{
		"symbol":    protoTick.Symbol,
		"timestamp": protoTick.Timestamp,
		"price":     protoTick.LastPrice,
		"volume":    protoTick.Volume,
		"data":      lp,              // Line Protocol → db_writer Option 1
		"json_data": string(payload), // 完整 JSON → 调试
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

// formatFloatLP 将 float64 格式化为 InfluxDB Line Protocol 兼容的浮点字面量
// 关键：必须始终包含小数点，否则 InfluxDB 会将整数值推断为 int 类型，
// 与后续浮点值冲突报错 "field type conflict"。
func formatFloatLP(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// escapeLP 转义 InfluxDB Line Protocol tag value 中的特殊字符
// (, =, 空格, \ 需反斜杠转义)
func escapeLP(s string) string {
	if !strings.ContainsAny(s, `,= \`) {
		return s
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `,`, `\,`)
	s = strings.ReplaceAll(s, `=`, `\=`)
	s = strings.ReplaceAll(s, ` `, `\ `)
	return s
}

// formatLineProtocol 将 ProtoTick 格式化为 InfluxDB Line Protocol
// 格式: measurement,tag_set field_set timestamp_ns
func formatLineProtocol(t *domain.ProtoTick) string {
	// 测量名: tick
	// Tag: symbol（索引维度，特殊字符已转义）
	// Fields: 所有行情字段（浮点数强制保留小数点）
	// Timestamp: 毫秒 → 纳秒（InfluxDB 要求纳秒精度）

	fields := []string{
		fmt.Sprintf("last_price=%s", formatFloatLP(t.LastPrice)),
		fmt.Sprintf("volume=%di", t.Volume),
		fmt.Sprintf("open_price=%s", formatFloatLP(t.OpenPrice)),
		fmt.Sprintf("high_price=%s", formatFloatLP(t.HighPrice)),
		fmt.Sprintf("low_price=%s", formatFloatLP(t.LowPrice)),
		fmt.Sprintf("pre_close=%s", formatFloatLP(t.PreClose)),
		fmt.Sprintf("pre_settle=%s", formatFloatLP(t.PreSettle)),
		fmt.Sprintf("open_interest=%s", formatFloatLP(t.OpenInterest)),
		fmt.Sprintf("upper_limit=%s", formatFloatLP(t.UpperLimit)),
		fmt.Sprintf("lower_limit=%s", formatFloatLP(t.LowerLimit)),
		fmt.Sprintf("bid_price1=%s", formatFloatLP(t.BidPrice1)),
		fmt.Sprintf("bid_volume1=%di", t.BidVolume1),
		fmt.Sprintf("ask_price1=%s", formatFloatLP(t.AskPrice1)),
		fmt.Sprintf("ask_volume1=%di", t.AskVolume1),
	}

	// InfluxDB 时间戳用纳秒
	tsNs := t.Timestamp * 1_000_000

	return fmt.Sprintf("tick,symbol=%s %s %d", escapeLP(t.Symbol), strings.Join(fields, ","), tsNs)
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
