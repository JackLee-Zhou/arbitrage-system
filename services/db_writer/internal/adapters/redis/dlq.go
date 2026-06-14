// db_writer/adapters/redis -- 死信队列操作
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	pkgredis "github.com/arbitrage-system/pkg/redis"

	"github.com/arbitrage-system/db_writer/internal/domain"
)

// dlqKeyPrefix 死信队列 key 前缀
const dlqKeyPrefix = "dlq:write:"

// DLQWriter 死信队列写入器
type DLQWriter struct {
	client pkgredis.RedisClient
}

// NewDLQWriter 创建死信队列写入器
func NewDLQWriter(client pkgredis.RedisClient) *DLQWriter {
	return &DLQWriter{client: client}
}

// dlqKey 返回死信队列的 Redis key
func dlqKey(bucket string) string {
	return dlqKeyPrefix + bucket
}

// Write 将死信条目写入 Redis Stream
// XADD dlq:write:<bucket> * data <json>
// 并设置 7 天 TTL
func (d *DLQWriter) Write(ctx context.Context, bucket string, entry domain.DLQEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal dlq entry: %w", err)
	}

	values := map[string]interface{}{
		"data":   string(data),
		"bucket": bucket,
	}

	key := dlqKey(bucket)
	if err := d.client.XAdd(ctx, key, values); err != nil {
		return fmt.Errorf("xadd dlq %s: %w", key, err)
	}

	// 设置 7 天 TTL
	if err := d.client.Expire(ctx, key, domain.DLQTTL); err != nil {
		log.Printf("[dlq] warning: failed to set ttl for %s: %v", key, err)
	}

	log.Printf("[dlq] wrote dead letter: bucket=%s retry_count=%d last_error=%s",
		bucket, entry.RetryCount, entry.LastError)
	return nil
}

// Size 返回死信队列的消息数量
// XLEN dlq:write:<bucket>
func (d *DLQWriter) Size(ctx context.Context, bucket string) (int64, error) {
	count, err := d.client.XLen(ctx, dlqKey(bucket))
	if err != nil {
		return 0, fmt.Errorf("xlen dlq for bucket %s: %w", bucket, err)
	}
	return count, nil
}
