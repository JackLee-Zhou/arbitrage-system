// orchestrator/adapters/audit_writer — 审计日志写入 Redis Stream
// 所有修改型操作 XADD write:audit → db_writer 消费 → 持久化到 InfluxDB
// 注意：Redis Stream 不支持嵌套 map，所有字段扁平化到顶层 key-value
package audit_writer

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"

	"github.com/arbitrage-system/orchestrator/internal/domain"
)

// Writer 审计日志 Redis Stream 写入器
type Writer struct {
	client       pkgredis.RedisClient
	writeTimeout time.Duration
}

// NewWriter 创建审计日志写入器
func NewWriter(client pkgredis.RedisClient) *Writer {
	return &Writer{
		client:       client,
		writeTimeout: 3 * time.Second,
	}
}

// Write 写入一条审计日志到 Redis Stream write:audit
// 使用 InfluxDB Line Protocol 写入 data 字段，db_writer 的 extractRecord Option 1 直接消费
func (w *Writer) Write(ctx context.Context, entry domain.AuditEntry) error {
	// 构造 InfluxDB Line Protocol
	// 格式: audit_log,action=xxx,operator=xxx target="...",old_value="...",new_value="...",result="..." ts_ns
	lp := fmt.Sprintf("audit_log,action=%s,operator=%s target=%q,old_value=%q,new_value=%q,result=%q %d",
		escapeLP(entry.Action),
		escapeLP(entry.Operator),
		fmt.Sprintf("%v", entry.Target),
		entry.OldValue,
		entry.NewValue,
		entry.Result,
		entry.Timestamp.UnixMilli()*1_000_000,
	)

	values := map[string]interface{}{
		"data": lp,
	}

	ctx, cancel := context.WithTimeout(ctx, w.writeTimeout)
	defer cancel()

	if err := w.client.XAdd(ctx, "write:audit", values); err != nil {
		return fmt.Errorf("audit XADD write:audit: %w", err)
	}

	log.Printf("[audit_writer] recorded: action=%s operator=%s target=%s result=%s",
		entry.Action, entry.Operator, entry.Target, entry.Result)
	return nil
}

// escapeLP 转义 InfluxDB Line Protocol tag value 中的特殊字符 (, =, 空格, \)
func escapeLP(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `,`, `\,`)
	s = strings.ReplaceAll(s, `=`, `\=`)
	s = strings.ReplaceAll(s, ` `, `\ `)
	return s
}
