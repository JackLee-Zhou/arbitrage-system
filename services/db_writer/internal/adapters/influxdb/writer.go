// db_writer/adapters/influxdb -- InfluxDB 批量写入器
package influxdb

import (
	"context"
	"fmt"
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	"github.com/arbitrage-system/db_writer/internal/domain"
)

// Writer InfluxDB 批量写入器
type Writer struct {
	client influxdb2.Client
	org    string
	policy domain.RetryPolicy
}

// NewWriter 创建 InfluxDB 写入器
func NewWriter(url, token, org string, policy domain.RetryPolicy) (*Writer, error) {
	client := influxdb2.NewClient(url, token)

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Health(ctx)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("influxdb health check failed: %w", err)
	}

	w := &Writer{
		client: client,
		org:    org,
		policy: policy,
	}

	log.Printf("[influxdb] writer created: url=%s org=%s", url, org)
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
// 使用 WriteAPIBlocking 同步写入所有记录
func (w *Writer) writeBatchOnce(ctx context.Context, bucket string, records []string) error {
	writeAPI := w.client.WriteAPIBlocking(w.org, bucket)
	return writeAPI.WriteRecord(ctx, records...)
}

// Ping 检查 InfluxDB 连通性
func (w *Writer) Ping(ctx context.Context) error {
	_, err := w.client.Health(ctx)
	return err
}

// Close 关闭 InfluxDB 连接
func (w *Writer) Close() {
	w.client.Close()
	log.Println("[influxdb] client closed")
}
