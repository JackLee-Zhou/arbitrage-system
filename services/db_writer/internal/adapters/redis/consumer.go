// db_writer/adapters/redis -- Redis Streams 消费者
package redis

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arbitrage-system/db_writer/internal/adapters/config"
	"github.com/arbitrage-system/db_writer/internal/adapters/influxdb"
	"github.com/arbitrage-system/db_writer/internal/domain"
	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// ============================================================
// 常量
// ============================================================

// ConsumerGroupName 消费组名称（导出供 main.go 使用）
const ConsumerGroupName = "db_writer"

// maxDLQRetries 写入 DLQ 时记录的已重试次数
const maxDLQRetries = 3

// bufferedMessage 缓存的消息（含记录和 Stream 消息 ID）
type bufferedMessage struct {
	id     string
	record string
}

// ============================================================
// StreamConsumer -- 单 Stream 消费者
// ============================================================

// StreamConsumer Redis Stream 消费者
// 每个实例消费一个 stream，运行在独立 goroutine 中
type StreamConsumer struct {
	client       pkgredis.RedisClient
	stream       string
	bucket       string
	groupName    string
	consumerName string
	batchRule    domain.BatchRule

	influxWriter *influxdb.Writer // 共享的 InfluxDB 写入器

	buffer    []bufferedMessage // 缓存的 Line Protocol 记录
	lastFlush time.Time         // 上次 flush 时间
	mu        sync.Mutex

	writeTotal     *atomic.Int64
	writeErrors    *atomic.Int64
	writeLatencyMs *atomic.Int64

	stopCh chan struct{}
	doneCh chan struct{}

	// 配置化字段（来自 config）
	xReadBlockMs    int64
	xReadBatchSize  int64
	dlqWriteTimeout time.Duration
	dlqConcurrency  int
}

// NewStreamConsumer 创建 Stream 消费者
func NewStreamConsumer(
	client pkgredis.RedisClient,
	stream string,
	batchRule domain.BatchRule,
	influxWriter *influxdb.Writer,
	writeTotal *atomic.Int64,
	writeErrors *atomic.Int64,
	writeLatencyMs *atomic.Int64,
	xReadBlockMs int64,
	xReadBatchSize int64,
	dlqWriteTimeout time.Duration,
	dlqConcurrency int,
) *StreamConsumer {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	return &StreamConsumer{
		client:          client,
		stream:          stream,
		bucket:          config.StreamToBucket(stream),
		groupName:       ConsumerGroupName,
		consumerName:    ConsumerGroupName + "_" + hostname,
		batchRule:       batchRule,
		influxWriter:    influxWriter,
		buffer:          make([]bufferedMessage, 0, batchRule.MaxSize),
		lastFlush:       time.Now(),
		writeTotal:      writeTotal,
		writeErrors:     writeErrors,
		writeLatencyMs:  writeLatencyMs,
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
		xReadBlockMs:    xReadBlockMs,
		xReadBatchSize:  xReadBatchSize,
		dlqWriteTimeout: dlqWriteTimeout,
		dlqConcurrency:  dlqConcurrency,
	}
}

// Consume 是主消费循环，在 goroutine 中运行
// 逻辑：XREADGROUP 阻塞读取 --> 提取 Line Protocol --> 缓存 --> 满足 batch 条件时 flush
// ctx 控制整体生命周期，退出时 flush 残余数据
func (c *StreamConsumer) Consume(ctx context.Context) {
	defer close(c.doneCh)

	log.Printf("[consumer] starting: stream=%s bucket=%s group=%s consumer=%s",
		c.stream, c.bucket, c.groupName, c.consumerName)

	// 创建消费组（如果已存在会报错，但忽略）
	if err := c.client.XGroupCreate(ctx, c.stream, c.groupName, "$"); err != nil {
		// BUSYGROUP è¡¨ç¤ºæ¶è´¹ç»å·²å­å¨ï¼å±äºæ­£å¸¸æåµ
		// å¶ä»éè¯¯éè¦è®°å½
		if !isBusyGroupError(err) {
			log.Printf("[consumer] xgroupcreate error (stream=%s): %v", c.stream, err)
		}
	}

	// 定时 flush ticker
	batchTicker := time.NewTicker(c.batchRule.MaxWait)
	defer batchTicker.Stop()

	for {
		select {
		case <-c.stopCh:
			log.Printf("[consumer] stop signal: stream=%s", c.stream)
			c.flushRemaining(context.Background())
			return

		case <-ctx.Done():
			log.Printf("[consumer] context done: stream=%s", c.stream)
			c.flushRemaining(context.Background())
			return

		case <-batchTicker.C:
			c.flushIfNeeded(ctx)

		default:
			c.readAndProcess(ctx)
		}
	}
}

// Stop 发送停止信号，等待消费循环退出
// ctx 控制等待超时，超时返回 ctx.Err()
func (c *StreamConsumer) Stop(ctx context.Context) error {
	close(c.stopCh)
	select {
	case <-c.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ============================================================
// 内部方法
// ============================================================

// readAndProcess XREADGROUP 阻塞读取并处理消息
func (c *StreamConsumer) readAndProcess(ctx context.Context) {
	msgs, err := c.client.XReadGroup(ctx, pkgredis.XReadGroupArgs{
		Group:    c.groupName,
		Consumer: c.consumerName,
		Streams:  []string{c.stream},
		IDs:      []string{">"},
		Block:    c.xReadBlockMs,
		Count:    c.xReadBatchSize,
	})
	if err != nil {
		if err != context.DeadlineExceeded && err != context.Canceled {
			log.Printf("[consumer] xreadgroup error (stream=%s): %v", c.stream, err)
		}
		return
	}

	if len(msgs) == 0 {
		return
	}

	c.processMessages(ctx, msgs)

	// 检查是否满足 flush 条件
	c.flushIfNeeded(ctx)
}

// processMessages 将消息缓存在 buffer 中（含 Stream 消息 ID）
// XACK 延迟到 InfluxDB 写入成功后执行，避免数据丢失
func (c *StreamConsumer) processMessages(ctx context.Context, msgs []pkgredis.XMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var ackIDs []string

	for _, msg := range msgs {
		record := extractRecord(msg.Values)
		if record == "" {
			// 空数据直接 ACK，不做处理
			ackIDs = append(ackIDs, msg.ID)
			continue
		}
		c.buffer = append(c.buffer, bufferedMessage{
			id:     msg.ID,
			record: record,
		})
	}

	// 空数据立即 ACK（无需持久化）
	if len(ackIDs) > 0 {
		if err := c.client.XAck(ctx, c.stream, c.groupName, ackIDs...); err != nil {
			log.Printf("[consumer] xack error (stream=%s): %v", c.stream, err)
		}
	}
}

// flushIfNeeded 检查是否满足 flush 条件并执行
func (c *StreamConsumer) flushIfNeeded(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.batchRule.ShouldFlush(len(c.buffer), c.lastFlush) {
		c.flushBuffer(ctx)
		c.lastFlush = time.Now()
	}
}

// flushRemaining 退出前刷残余数据
func (c *StreamConsumer) flushRemaining(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.buffer) > 0 {
		log.Printf("[consumer] flushing remaining %d records for bucket %s", len(c.buffer), c.bucket)
		c.flushBuffer(ctx)
	}
}

// flushBuffer 将缓存的数据批量写入 InfluxDB
// 必须先持有 c.mu 锁
func (c *StreamConsumer) flushBuffer(ctx context.Context) {
	if len(c.buffer) == 0 {
		return
	}

	batch := make([]bufferedMessage, len(c.buffer))
	copy(batch, c.buffer)
	c.buffer = c.buffer[:0]

	// 释放锁后执行写入（避免长时间持有锁）
	c.mu.Unlock()
	defer func() { c.mu.Lock() }()
	c.flushRecords(ctx, batch)
}

// flushRecords 执行实际的 InfluxDB 写入
// 写入成功则 XACK，失败则写入 DLQ，DLQ 成功则 XACK
func (c *StreamConsumer) flushRecords(ctx context.Context, batch []bufferedMessage) {
	if c.influxWriter == nil {
		log.Printf("[consumer] influxdb writer not set, skipping flush for bucket %s", c.bucket)
		return
	}

	records := make([]string, 0, len(batch))
	ids := make([]string, 0, len(batch))
	for _, m := range batch {
		records = append(records, m.record)
		ids = append(ids, m.id)
	}

	start := time.Now()
	err := c.influxWriter.WriteBatch(ctx, c.bucket, records)
	latencyMs := time.Since(start).Milliseconds()

	if c.writeLatencyMs != nil {
		c.writeLatencyMs.Store(latencyMs)
	}

	if err != nil {
		c.writeErrors.Add(1)
		log.Printf("[consumer] write to influxdb failed for bucket %s: %v (latency=%dms)", c.bucket, err, latencyMs)

		// 写入 DLQ
		dlqErr := c.writeToDLQ(ctx, records, err)
		if dlqErr == nil {
			// DLQ 写入成功，XACK 避免重复消费
			c.ackMessages(ctx, ids)
		} else {
			// InfluxDB 和 DLQ 都失败，不 XACK，消息会重新投递
			log.Printf("[consumer] both influxdb write and dlq write failed for bucket %s, %d messages will be redelivered",
				c.bucket, len(ids))
		}
	} else {
		c.writeTotal.Add(int64(len(records)))
		log.Printf("[consumer] flushed %d records to bucket %s (latency=%dms)", len(records), c.bucket, latencyMs)
		// 写入成功，XACK
		c.ackMessages(ctx, ids)
	}
}

// writeToDLQ 写入死信队列，返回首个写入错误（若有）
// 使用并发写入，并发数由 dlqConcurrency 控制
func (c *StreamConsumer) writeToDLQ(ctx context.Context, records []string, writeErr error) error {
	dlq := NewDLQWriter(c.client)
	var (
		wg       sync.WaitGroup
		firstErr error
		mu       sync.Mutex
	)
	sem := make(chan struct{}, c.dlqConcurrency)

	for _, record := range records {
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			entry := domain.DLQEntry{
				Bucket:     c.bucket,
				Data:       []byte(r),
				RetryCount: maxDLQRetries,
				LastError:  writeErr.Error(),
			}

			// 每个写入操作有独立超时
			writeCtx, cancel := context.WithTimeout(ctx, c.dlqWriteTimeout)
			defer cancel()
			if err := dlq.Write(writeCtx, c.bucket, entry); err != nil {
				log.Printf("[consumer] dlq write failed for bucket %s: %v", c.bucket, err)
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(record)
	}
	wg.Wait()
	return firstErr
}

// ackMessages 确认 Stream 消息
func (c *StreamConsumer) ackMessages(ctx context.Context, ids []string) {
	if len(ids) == 0 {
		return
	}
	if err := c.client.XAck(ctx, c.stream, c.groupName, ids...); err != nil {
		log.Printf("[consumer] xack error (stream=%s): %v", c.stream, err)
	}
}

// isBusyGroupError 判断是否为消费组已存在错误
func isBusyGroupError(err error) bool {
	return strings.Contains(err.Error(), "BUSYGROUP")
}

// ============================================================
// 消息解析
// ============================================================

// extractRecord 从 Redis Stream 消息中提取 Line Protocol 记录
// 优先使用 "data" 字段（上游服务应写入该字段）
// 其次尝试 JSON point 格式（measurement + fields + timestamp）
func extractRecord(values map[string]interface{}) string {
	if len(values) == 0 {
		return ""
	}

	// 方案 1: 使用 "data" 字段（Line Protocol 字符串）
	if data, ok := values["data"]; ok && data != nil {
		switch v := data.(type) {
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	}

	// 方案 2: JSON point 格式：measurement + fields
	if _, ok := values["measurement"]; ok {
		return formatLineProtocol(values)
	}

	return ""
}

// formatLineProtocol 从 JSON point 格式构建 Line Protocol
// 格式：measurement[,tag=val...] field=val[,field=val...] [timestamp]
func formatLineProtocol(values map[string]interface{}) string {
	measurement, _ := values["measurement"].(string)
	if measurement == "" {
		return ""
	}

	// 处理 tags
	var tagStr string
	if t, ok := values["tags"]; ok {
		if tagMap, ok := t.(map[string]interface{}); ok {
			for k, v := range tagMap {
				tagStr += fmt.Sprintf(",%s=%v", k, v)
			}
		}
	}

	// 处理 fields
	var fieldStr string
	first := true
	if f, ok := values["fields"]; ok {
		if fieldMap, ok := f.(map[string]interface{}); ok {
			for k, v := range fieldMap {
				if !first {
					fieldStr += ","
				}
				fv := formatFieldValue(v)
				fieldStr += k + "=" + fv
				first = false
			}
		}
	}
	if fieldStr == "" {
		return ""
	}

	// 处理 timestamp
	var ts string
	if t, ok := values["timestamp"]; ok {
		ts = fmt.Sprintf(" %v", t)
	}

	return measurement + tagStr + " " + fieldStr + ts
}

// formatFieldValue 格式化 InfluxDB 字段值
func formatFieldValue(v interface{}) string {
	switch val := v.(type) {
	case float64:
		return fmt.Sprintf("%v", val)
	case int64:
		return fmt.Sprintf("%di", val)
	case int:
		return fmt.Sprintf("%di", val)
	case string:
		return fmt.Sprintf("%q", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%q", fmt.Sprintf("%v", val))
	}
}
