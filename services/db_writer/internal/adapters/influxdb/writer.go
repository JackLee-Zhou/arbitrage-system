// db_writer/adapters/influxdb -- InfluxDB 批量写入器
package influxdb

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"

	"github.com/arbitrage-system/db_writer/internal/domain"
)

// Writer InfluxDB 批量写入器
type Writer struct {
	client       influxdb2.Client
	org          string
	policy       domain.RetryPolicy
	writeTimeout time.Duration
	writeAPIs    map[string]api.WriteAPIBlocking
	mu           sync.RWMutex
}

// NewWriter 创建 InfluxDB 写入器
func NewWriter(url, token, org string, policy domain.RetryPolicy, writeTimeout time.Duration) (*Writer, error) {
	client := influxdb2.NewClient(url, token)

	// 验证连接
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer healthCancel()
	_, err := client.Health(healthCtx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("influxdb health check failed: %w", err)
	}

	w := &Writer{
		client:       client,
		org:          org,
		policy:       policy,
		writeTimeout: writeTimeout,
		writeAPIs:    make(map[string]api.WriteAPIBlocking),
	}

	log.Printf("[influxdb] writer created: url=%s org=%s write_timeout=%s", url, org, writeTimeout)
	return w, nil
}

// WriteBatch 批量写入 InfluxDB
// records 是 Line Protocol 格式的字符串数组
// 失败使用 RetryPolicy 重试（指数退避）
// 3次都失败返回 error（调用方写入DLQ）
func (w *Writer) WriteBatch(ctx context.Context, bucket string, records []string) error {
	if len(records) == 0 {
		return nil
	}

	var lastErr error
	for attempt := 0; attempt <= w.policy.MaxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避等待
			backoff := w.policy.Backoff(attempt - 1)
			log.Printf("[influxdb] retry %d/%d after %v for bucket %s (%d records)",
				attempt, w.policy.MaxRetries, backoff, bucket, len(records))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		lastErr = w.writeBatchOnce(ctx, bucket, records)
		if lastErr == nil {
			return nil
		}

		log.Printf("[influxdb] write batch failed (attempt %d/%d): %v", attempt+1, w.policy.MaxRetries+1, lastErr)

		if !w.policy.ShouldRetry(attempt) {
			break
		}
	}

	return fmt.Errorf("influxdb write batch failed after %d retries: %w", w.policy.MaxRetries, lastErr)
}

// writeBatchOnce 执行一次批量写入
// 使用 WriteAPIBlocking 同步写入所有记录（缓存 WriteAPI 实例避免重复创建）
func (w *Writer) writeBatchOnce(ctx context.Context, bucket string, records []string) error {
	// 获取或缓存 WriteAPIBlocking 实例
	writeAPI := w.getOrCreateWriteAPI(bucket)

	// 应用写入超时
	if w.writeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, w.writeTimeout)
		defer cancel()
	}

	return writeAPI.WriteRecord(ctx, records...)
}

// getOrCreateWriteAPI 获取缓存的 WriteAPIBlocking，不存在则创建
func (w *Writer) getOrCreateWriteAPI(bucket string) api.WriteAPIBlocking {
	w.mu.RLock()
	writeAPI, ok := w.writeAPIs[bucket]
	w.mu.RUnlock()
	if ok {
		return writeAPI
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	// Double-check
	writeAPI, ok = w.writeAPIs[bucket]
	if !ok {
		writeAPI = w.client.WriteAPIBlocking(w.org, bucket)
		w.writeAPIs[bucket] = writeAPI
	}
	return writeAPI
}

// Ping 检查 InfluxDB 连通性
func (w *Writer) Ping(ctx context.Context) error {
	_, err := w.client.Health(ctx)
	return err
}

// Close 关闭 InfluxDB 连接和缓存的 WriteAPI
func (w *Writer) Close() {
	w.mu.Lock()
	w.writeAPIs = nil
	w.mu.Unlock()
	w.client.Close()
	log.Println("[influxdb] client closed")
}
