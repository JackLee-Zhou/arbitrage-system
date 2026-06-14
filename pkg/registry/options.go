// pkg/registry — 服务注册/心跳/注销包
// 统一封装服务注册到 Redis 的逻辑，所有 Go 服务通过此包上报自身状态。
package registry

import "time"

// ============================================================
// Option — 函数选项模式
// ============================================================

// Option 配置 Registry 的函数选项。
type Option func(*Registry)

// WithTTL 设置服务注册在 Redis 中的 TTL 秒数。
// 默认值 DefaultTTL（30 秒）。
func WithTTL(ttlSec int64) Option {
	return func(r *Registry) {
		if ttlSec > 0 {
			r.ttlSec = ttlSec
		}
	}
}

// WithInterval 设置心跳续约间隔。
// 默认值 DefaultInterval（10 秒）。
func WithInterval(interval time.Duration) Option {
	return func(r *Registry) {
		if interval > 0 {
			r.interval = interval
		}
	}
}
