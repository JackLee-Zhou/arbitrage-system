// db_writer/domain — 重试策略（指数退避）
package domain

import "time"

const (
	MaxRetries        = 3
	InitialBackoff    = 1 * time.Second
	BackoffMultiplier = 4
)

// RetryPolicy 重试策略
type RetryPolicy struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	BackoffMultiplier int
}

// DefaultRetryPolicy 默认重试策略：1s → 4s → 16s
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:        MaxRetries,
		InitialBackoff:    InitialBackoff,
		BackoffMultiplier: BackoffMultiplier,
	}
}

// Backoff 计算第 n 次重试的等待时间
func (r RetryPolicy) Backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return r.InitialBackoff
	}
	delay := r.InitialBackoff
	for i := 0; i < attempt; i++ {
		delay *= time.Duration(r.BackoffMultiplier)
	}
	return delay
}

// ShouldRetry 判断是否应该继续重试
func (r RetryPolicy) ShouldRetry(attempt int) bool {
	return attempt < r.MaxRetries
}
