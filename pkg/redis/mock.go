// pkg/redis — Mock 实现（单元测试用）
package redis

import (
	"context"
	"fmt"
	"sync"
)

// ============================================================
// MockRedisClient — 内存实现，不依赖外部 Redis 服务
// 用于各个 Go 服务的 unit 测试（ENGINEERING_GUIDE.md 规则 5）
// ============================================================

// MockRedisClient 内存 Redis 实现
// 所有数据存在内存 map 中，stream 用切片模拟
type MockRedisClient struct {
	mu      sync.RWMutex
	kv      map[string]string          // KV 存储
	streams map[string][]*mockStreamMsg // Stream 存储
	ttl     map[string]int64           // TTL（unix 过期时间）
	closed  bool

	// 失败注入（用于测试 retry 逻辑）
	xAddFailCount int // 前 N 次 XAdd 调用返回错误，0 = 不注入
	xAddErr       error
	getFailCount  int // 前 N 次 Get 调用返回错误，0 = 不注入
	getErr        error
	setFailCount  int // 前 N 次 Set 调用返回错误，0 = 不注入
	setErr        error
}

type mockStreamMsg struct {
	id     string
	values map[string]interface{}
}

// NewMockClient 创建 Mock Redis 客户端（单元测试用）
func NewMockClient() *MockRedisClient {
	return &MockRedisClient{
		kv:      make(map[string]string),
		streams: make(map[string][]*mockStreamMsg),
		ttl:     make(map[string]int64),
	}
}

// ---- Streams ----

// SetXAddFailures 注入失败：前 n 次 XAdd 调用返回给定错误（0 表示取消注入）
func (m *MockRedisClient) SetXAddFailures(n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.xAddFailCount = n
	m.xAddErr = err
}

// XAdd mock 实现
func (m *MockRedisClient) XAdd(ctx context.Context, stream string, values map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 失败注入
	if m.xAddFailCount > 0 {
		m.xAddFailCount--
		return m.xAddErr
	}

	id := fmt.Sprintf("%d-%d", len(m.streams[stream]), len(m.streams[stream]))
	m.streams[stream] = append(m.streams[stream], &mockStreamMsg{
		id:     id,
		values: values,
	})

	// 裁剪到 10000 条
	if len(m.streams[stream]) > 10000 {
		m.streams[stream] = m.streams[stream][len(m.streams[stream])-10000:]
	}

	return nil
}

// XRead mock 实现
func (m *MockRedisClient) XRead(ctx context.Context, args XReadArgs) ([]XMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []XMessage
	for i, stream := range args.Streams {
		msgs := m.streams[stream]
		startID := "0"
		if i < len(args.IDs) {
			startID = args.IDs[i]
		}
		for _, msg := range msgs {
			if msg.id > startID {
				result = append(result, XMessage{
					ID:     msg.id,
					Stream: stream,
					Values: msg.values,
				})
				if args.Count > 0 && int64(len(result)) >= args.Count {
					break
				}
			}
		}
	}
	return result, nil
}

// XReadGroup mock 实现（与 XRead 行为相同）
func (m *MockRedisClient) XReadGroup(ctx context.Context, args XReadGroupArgs) ([]XMessage, error) {
	xrArgs := XReadArgs{
		Streams: args.Streams,
		IDs:     args.IDs,
		Block:   args.Block,
		Count:   args.Count,
	}
	return m.XRead(ctx, xrArgs)
}

// XAck mock 实现（no-op）
func (m *MockRedisClient) XAck(ctx context.Context, stream, group string, ids ...string) error {
	return nil
}

// XLen mock 实现
func (m *MockRedisClient) XLen(ctx context.Context, stream string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.streams[stream])), nil
}

// XGroupCreate mock 实现（no-op）
func (m *MockRedisClient) XGroupCreate(ctx context.Context, stream, group, start string) error {
	return nil
}

// Expire mock 实现：设置 TTL（写入 ttl map）
func (m *MockRedisClient) Expire(ctx context.Context, key string, ttlSeconds int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ttl[key] = ttlSeconds
	return nil
}

// ---- KV ----

// SetGetFailures 注入失败：前 n 次 Get 调用返回给定错误（0 表示取消注入）
func (m *MockRedisClient) SetGetFailures(n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getFailCount = n
	m.getErr = err
}

// SetSetFailures 注入失败：前 n 次 Set 调用返回给定错误（0 表示取消注入）
func (m *MockRedisClient) SetSetFailures(n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setFailCount = n
	m.setErr = err
}

// Get mock 实现
func (m *MockRedisClient) Get(ctx context.Context, key string) (string, error) {
	// 验证 context 是否已超时/取消
	if err := ctx.Err(); err != nil {
		return "", err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 失败注入
	if m.getFailCount > 0 {
		m.getFailCount--
		return "", m.getErr
	}

	v, ok := m.kv[key]
	if !ok {
		return "", fmt.Errorf("redis: nil")
	}
	return v, nil
}

// Set mock 实现
func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, ttlSeconds int64) error {
	// 验证 context 是否已超时/取消
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 失败注入
	if m.setFailCount > 0 {
		m.setFailCount--
		return m.setErr
	}

	m.kv[key] = fmt.Sprintf("%v", value)
	return nil
}

// Del mock 实现
func (m *MockRedisClient) Del(ctx context.Context, keys ...string) error {
	// 验证 context 是否已超时/取消
	if err := ctx.Err(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.kv, k)
	}
	return nil
}

// Keys mock 实现
func (m *MockRedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []string
	for k := range m.kv {
		result = append(result, k)
	}
	return result, nil
}

// ---- 基础 ----

// Ping mock 实现（永远健康）
func (m *MockRedisClient) Ping(ctx context.Context) error {
	if m.closed {
		return fmt.Errorf("redis: client closed")
	}
	return nil
}

// Close mock 实现
func (m *MockRedisClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
