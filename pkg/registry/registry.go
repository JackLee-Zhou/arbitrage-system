// pkg/registry — 服务注册/心跳/注销包
// 统一封装服务注册到 Redis 的完整生命周期：
//
//  1. Register — 启动时将服务信息写入 Redis（SET services:<name> <json> EX <ttl>）
//  2. 心跳续约 — 后台 goroutine 每 interval 刷新 TTL
//  3. Shutdown — 停止心跳 + DEL key
//
// Redis Key 格式：services:<name>（与 ENGINEERING_GUIDE.md Redis Key 规范一致）
// Value 格式：ServiceInfo 的 JSON 序列化
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// ============================================================
// 常量
// ============================================================

const (
	// DefaultTTL 服务注册的默认 TTL（秒）
	DefaultTTL int64 = 30

	// DefaultInterval 心跳续约的默认间隔
	DefaultInterval = 10 * time.Second

	// DefaultShutdownTimeout 等待心跳 goroutine 退出的默认超时
	DefaultShutdownTimeout = 5 * time.Second

	// MaxRenewalFailures 连续心跳续约失败阈值，达到此值时输出 ERROR 日志
	MaxRenewalFailures = 3
)

// ============================================================
// ServiceInfo — 服务注册信息
// ============================================================

// ServiceInfo 描述一个服务的注册信息，序列化为 JSON 存入 Redis。
type ServiceInfo struct {
	// Name 服务名，如 "ctp_tick"
	Name string `json:"name"`

	// Version 版本号
	Version string `json:"version,omitempty"`

	// StartedAt 服务启动时间（ISO8601）
	// 注意：omitempty 对 time.Time 无效（struct 类型），零值也会被序列化，
	// 因此明确移除 omitempty 以避免未来 Go 版本行为变化导致丢失该字段。
	StartedAt time.Time `json:"started_at"`

	// HealthURL 健康检查地址，如 ":8081"
	HealthURL string `json:"health_url,omitempty"`

	// MetricsURL 指标地址，如 ":8081"
	MetricsURL string `json:"metrics_url,omitempty"`

	// GRPCAddr gRPC 地址（可选），如 ":9090"
	GRPCAddr string `json:"grpc_addr,omitempty"`

	// Capabilities 能力列表，如 ["tick", "focus_push"]
	Capabilities []string `json:"capabilities,omitempty"`

	// Metadata 扩展字段
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ============================================================
// Registry — 服务注册器
// ============================================================

// Registry 管理与 Redis 之间的服务注册和心跳续约。
// 通过 Register 创建，通过 Shutdown 销毁。
type Registry struct {
	client    pkgredis.RedisClient
	info      ServiceInfo
	ttlSec    int64
	interval  time.Duration
	stopCh    chan struct{}
	doneCh    chan struct{}
	mu        sync.Mutex
	shutdown  bool

	// renewFailures 连续心跳续约失败次数
	renewFailures atomic.Int64

	// infoBytes 预计算的 ServiceInfo JSON 序列化，避免每次心跳重复 Marshal
	infoBytes []byte
}

// serviceKey 返回 Redis 中的服务注册 key。
// 注意：name 应在调用前经过校验（Register 中已校验不包含冒号等特殊字符）。
func serviceKey(name string) string {
	return "services:" + name
}

// Register 注册服务并启动心跳续约。
//
// 该方法完成以下工作：
//  1. 检查参数合法性（client != nil, name != ""）
//  2. 检查是否已被注册（Get key，若存在则拒绝）
//  3. 序列化 ServiceInfo 为 JSON
//  4. SET key 到 Redis
//  5. 启动后台 goroutine 每 interval 续约一次
//
// 典型用法：
//
//	reg, err := registry.Register(ctx, redisClient, registry.ServiceInfo{
//	    Name:      "ctp_tick",
//	    Version:   "1.0.0",
//	    HealthURL: ":8081",
//	})
//	if err != nil {
//	    log.Fatalf("注册服务失败: %v", err)
//	}
//	defer reg.Shutdown(ctx)
//
// 可通过 opts 定制 TTL 和续约间隔：
//
//	reg, err := registry.Register(ctx, client, info,
//	    registry.WithTTL(60),
//	    registry.WithInterval(15*time.Second),
//	)
func Register(ctx context.Context, client pkgredis.RedisClient, info ServiceInfo, opts ...Option) (*Registry, error) {
	if client == nil {
		return nil, errors.New("redis client cannot be nil")
	}
	if info.Name == "" {
		return nil, errors.New("service name cannot be empty")
	}
	// 校验 name 不含冒号等特殊字符，防止 key 格式混乱
	for _, r := range info.Name {
		if r == ':' || r == '\n' {
			return nil, errors.New("service name contains invalid character '" + string(r) + "'")
		}
	}

	r := &Registry{
		client:   client,
		info:     info,
		ttlSec:   DefaultTTL,
		interval: DefaultInterval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	for _, opt := range opts {
		opt(r)
	}

	// 检查是否已注册（幂等性防御）
	key := serviceKey(info.Name)
	existing, err := client.Get(ctx, key)
	if err == nil && existing != "" {
		return nil, fmt.Errorf("service %q already registered (value: %s)", info.Name, existing)
	}
	// err != nil 说明 key 不存在或 Redis 不可达，继续尝试注册

	// 序列化 ServiceInfo
	data, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("marshal service info for %q failed: %w", info.Name, err)
	}
	r.infoBytes = data // 缓存序列化结果，避免每次心跳重复 Marshal

	// 写入 Redis
	if err := client.Set(ctx, key, string(data), r.ttlSec); err != nil {
		return nil, fmt.Errorf("set service key %q failed: %w", key, err)
	}

	// TODO: 未来改用 SET NX 原子操作替代 Get+Set 模式，避免分布式场景下的竞态条件
	// read-after-write 验证：确认我们写入的值被正确保存
	if stored, err := client.Get(ctx, key); err != nil || stored != string(data) {
		// 清理可能存在的脏数据
		_ = client.Del(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("verify write for service key %q failed: %w", key, err)
		}
		return nil, fmt.Errorf("verify write for service key %q failed: value mismatch (our data was overwritten)", key)
	}

	slog.Info("服务注册成功", "service", info.Name, "key", key, "ttl", r.ttlSec)

	// 启动心跳 goroutine
	go r.heartbeat()

	return r, nil
}

// heartbeat 后台运行，周期执行心跳续约。
// 通过监听 stopCh 实现优雅退出，退出时关闭 doneCh。
func (r *Registry) heartbeat() {
	defer close(r.doneCh)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			slog.Debug("心跳 goroutine 收到停止信号", "service", r.info.Name)
			return
		case <-ticker.C:
			r.renew()
		}
	}
}

// renew 执行一次心跳续约：重新 SET key 以刷新 TTL。
// 失败只记警告日志，不退出心跳循环（Redis 瞬断不应杀死服务）。
func (r *Registry) renew() {
	key := serviceKey(r.info.Name)
	// 使用预计算的 infoBytes，避免每次心跳重复 Marshal
	data := r.infoBytes
	if len(data) == 0 {
		// 防御性回退：正常流程 Register 时已预计算，此处仅作兜底
		var err error
		data, err = json.Marshal(r.info)
		if err != nil {
			failures := r.renewFailures.Add(1)
			slog.Warn("序列化 ServiceInfo 失败，跳过本次续约",
				"service", r.info.Name,
				"error", err,
				"failures", failures,
			)
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.interval)
	defer cancel()
	if err := r.client.Set(ctx, key, string(data), r.ttlSec); err != nil {
		failures := r.renewFailures.Add(1)
		if failures >= MaxRenewalFailures {
			slog.Error("Redis 心跳续约连续失败次数超过阈值",
				"service", r.info.Name,
				"key", key,
				"error", err,
				"failures", failures,
				"threshold", MaxRenewalFailures,
			)
		} else {
			slog.Warn("Redis 心跳续约失败",
				"service", r.info.Name,
				"key", key,
				"error", err,
				"failures", failures,
			)
		}
		return
	}

	// 续约成功，重置失败计数器
	r.renewFailures.Store(0)
}

// Shutdown 优雅关闭：停止心跳并删除 Redis 中的注册 key。
//
// 步骤：
//  1. 发送停止信号给心跳 goroutine
//  2. 等待 goroutine 退出（受 ctx 超时控制，默认 5s）
//  3. 从 Redis 删除 key
//
// ctx 可带超时控制等待心跳 goroutine 退出的最大时间。
// 如果 ctx 没有 deadline，自动施加 DefaultShutdownTimeout 超时。
// 多次调用 Shutdown 安全返回（第二次及后续返回 error）。
func (r *Registry) Shutdown(ctx context.Context) error {
	// 防重入
	r.mu.Lock()
	if r.shutdown {
		r.mu.Unlock()
		return errors.New("registry already shut down")
	}
	r.shutdown = true
	r.mu.Unlock()

	// 通知心跳 goroutine 退出
	close(r.stopCh)

	// 如果 ctx 没有 deadline，施加默认超时
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultShutdownTimeout)
		defer cancel()
	}

	// 等待心跳 goroutine 退出
	select {
	case <-r.doneCh:
		slog.Debug("心跳 goroutine 已退出", "service", r.info.Name)
	case <-ctx.Done():
		slog.Warn("等待心跳 goroutine 退出超时，继续清理",
			"service", r.info.Name,
			"timeout", ctx.Err(),
		)
	}

	// 删除 Redis 注册 key
	// 为 Del 创建独立 context，不受调用方（可能已超时）影响，确保清理一定执行
	key := serviceKey(r.info.Name)
	delCtx, delCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer delCancel()
	if err := r.client.Del(delCtx, key); err != nil {
		return fmt.Errorf("清理服务注册 key %q 失败: %w", key, err)
	}

	slog.Info("服务注销成功", "service", r.info.Name, "key", key)
	return nil
}
