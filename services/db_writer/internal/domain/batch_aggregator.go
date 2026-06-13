// db_writer/domain — 批量聚合规则（纯逻辑）
package domain

import "time"

// BatchRule 批量聚合规则
type BatchRule struct {
	MaxSize   int           // 最大条数（默认 100）
	MaxWait   time.Duration // 最大等待时间（默认 1s）
}

// ShouldFlush 判断是否应触发批量写入
func (r *BatchRule) ShouldFlush(currentSize int, lastFlush time.Time) bool {
	if currentSize >= r.MaxSize {
		return true
	}
	if time.Since(lastFlush) >= r.MaxWait && currentSize > 0 {
		return true
	}
	return false
}
