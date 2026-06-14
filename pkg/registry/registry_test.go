package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// ============================================================
// TestRegister
// ============================================================

func TestRegister_Success(t *testing.T) {
	mock := pkgredis.NewMockClient()
	now := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)

	info := ServiceInfo{
		Name:         "test_service_a",
		Version:      "1.2.3",
		StartedAt:    now,
		HealthURL:    ":8080",
		MetricsURL:   ":8081",
		GRPCAddr:     ":9090",
		Capabilities: []string{"tick", "focus_push"},
		Metadata:     map[string]string{"env": "staging"},
	}

	reg, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer safeShutdown(t, reg)

	// 验证 key 已写入
	val, err := mock.Get(context.Background(), "services:test_service_a")
	if err != nil {
		t.Fatalf("key should exist after register: %v", err)
	}

	// 反序列化并校验字段
	var stored ServiceInfo
	if err := json.Unmarshal([]byte(val), &stored); err != nil {
		t.Fatalf("unmarshal stored value failed: %v", err)
	}

	if stored.Name != "test_service_a" {
		t.Fatalf("expected Name 'test_service_a', got %q", stored.Name)
	}
	if stored.Version != "1.2.3" {
		t.Fatalf("expected Version '1.2.3', got %q", stored.Version)
	}
	if !stored.StartedAt.Equal(now) {
		t.Fatalf("expected StartedAt %v, got %v", now, stored.StartedAt)
	}
	if stored.HealthURL != ":8080" {
		t.Fatalf("expected HealthURL ':8080', got %q", stored.HealthURL)
	}
	if stored.MetricsURL != ":8081" {
		t.Fatalf("expected MetricsURL ':8081', got %q", stored.MetricsURL)
	}
	if stored.GRPCAddr != ":9090" {
		t.Fatalf("expected GRPCAddr ':9090', got %q", stored.GRPCAddr)
	}
	if len(stored.Capabilities) != 2 || stored.Capabilities[0] != "tick" {
		t.Fatalf("expected Capabilities ['tick','focus_push'], got %v", stored.Capabilities)
	}
	if stored.Metadata["env"] != "staging" {
		t.Fatalf("expected Metadata['env']='staging', got %q", stored.Metadata["env"])
	}
}

func TestRegister_EmptyName(t *testing.T) {
	mock := pkgredis.NewMockClient()
	info := ServiceInfo{Name: ""}

	_, err := Register(context.Background(), mock, info)
	if err == nil {
		t.Fatal("expected error for empty service name, got nil")
	}
	if !strings.Contains(err.Error(), "service name cannot be empty") {
		t.Fatalf("expected 'service name cannot be empty', got: %v", err)
	}
}

func TestRegister_NilClient(t *testing.T) {
	info := ServiceInfo{Name: "test_service"}

	_, err := Register(context.Background(), nil, info)
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
	if !strings.Contains(err.Error(), "redis client cannot be nil") {
		t.Fatalf("expected 'redis client cannot be nil', got: %v", err)
	}
}

func TestRegister_Duplicate(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "dup_test"}

	reg1, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	defer safeShutdown(t, reg1)

	// 第二次注册同一服务名应失败
	_, err = Register(context.Background(), mock, info)
	if err == nil {
		t.Fatal("expected error on duplicate registration, got nil")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("expected 'already registered' error, got: %v", err)
	}
}

// ============================================================
// TestShutdown
// ============================================================

func TestShutdown_DeletesKey(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "shutdown_test"}
	reg, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 验证 key 存在
	_, err = mock.Get(context.Background(), "services:shutdown_test")
	if err != nil {
		t.Fatalf("key should exist after register: %v", err)
	}

	// 执行 Shutdown
	if err := reg.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// 验证 key 已删除
	_, err = mock.Get(context.Background(), "services:shutdown_test")
	if err == nil {
		t.Fatal("key should be deleted after shutdown")
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "idempotent_test"}
	reg, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 第一次 Shutdown
	if err := reg.Shutdown(context.Background()); err != nil {
		t.Fatalf("first Shutdown failed: %v", err)
	}

	// 第二次 Shutdown 应返回 error
	if err := reg.Shutdown(context.Background()); err == nil {
		t.Fatal("expected error on second Shutdown")
	}
}

func TestShutdown_WithContextTimeout(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "timeout_test"}
	reg, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 用极短超时的 context 调用 Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Shutdown 应在超时后完成清理
	if err := reg.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown with timeout should complete: %v", err)
	}

	// key 应已被删除
	_, err = mock.Get(context.Background(), "services:timeout_test")
	if err == nil {
		t.Fatal("key should be deleted after shutdown")
	}
}

// ============================================================
// TestHeartbeat
// ============================================================

func TestHeartbeat_GoroutineExitsCleanly(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "heartbeat_clean_exit"}
	reg, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 快速 Shutdown，验证 goroutine 退出且 key 被删除
	if err := reg.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	_, err = mock.Get(context.Background(), "services:heartbeat_clean_exit")
	if err == nil {
		t.Fatal("key should be deleted after shutdown")
	}
}

func TestHeartbeat_RenewalDoesNotBlockShutdown(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "heartbeat_renew"}
	reg, err := Register(context.Background(), mock, info,
		WithInterval(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 等待几次心跳执行
	time.Sleep(120 * time.Millisecond)

	// 验证 key 因心跳续约依然存在
	val, err := mock.Get(context.Background(), "services:heartbeat_renew")
	if err != nil {
		t.Fatalf("key should exist after heartbeats: %v", err)
	}
	if val == "" {
		t.Fatal("value should not be empty after heartbeats")
	}

	// 关闭
	if err := reg.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

func TestHeartbeat_ConcurrentShutdown(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "concurrent_shutdown"}
	reg, err := Register(context.Background(), mock, info,
		WithInterval(20*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 多个 goroutine 同时调用 Shutdown
	var wg sync.WaitGroup
	errCh := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- reg.Shutdown(context.Background())
		}()
	}
	wg.Wait()
	close(errCh)

	// 至少有一个成功，其余的返回 error（幂等）
	successCount := 0
	for err := range errCh {
		if err == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful Shutdown, got %d", successCount)
	}

	// key 应已被删除
	_, err = mock.Get(context.Background(), "services:concurrent_shutdown")
	if err == nil {
		t.Fatal("key should be deleted after concurrent shutdown")
	}
}

// ============================================================
// TestOptions
// ============================================================

func TestWithTTL(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "ttl_test"}
	reg, err := Register(context.Background(), mock, info, WithTTL(60))
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer safeShutdown(t, reg)

	if reg.ttlSec != 60 {
		t.Fatalf("expected ttlSec=60, got %d", reg.ttlSec)
	}
}

func TestWithInterval(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "interval_test"}
	reg, err := Register(context.Background(), mock, info, WithInterval(5*time.Second))
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer safeShutdown(t, reg)

	if reg.interval != 5*time.Second {
		t.Fatalf("expected interval=5s, got %v", reg.interval)
	}
}

func TestWithTTL_ZeroIgnored(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "ttl_zero_test"}
	reg, err := Register(context.Background(), mock, info, WithTTL(0))
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer safeShutdown(t, reg)

	if reg.ttlSec != DefaultTTL {
		t.Fatalf("expected ttlSec=%d (default), got %d", DefaultTTL, reg.ttlSec)
	}
}

func TestWithInterval_ZeroIgnored(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "interval_zero_test"}
	reg, err := Register(context.Background(), mock, info, WithInterval(0))
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	defer safeShutdown(t, reg)

	if reg.interval != DefaultInterval {
		t.Fatalf("expected interval=%v (default), got %v", DefaultInterval, reg.interval)
	}
}

// ============================================================
// TestServiceInfo JSON
// ============================================================

func TestServiceInfo_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 30, 0, 5, time.UTC)

	original := ServiceInfo{
		Name:         "roundtrip_test",
		Version:      "2.0.0",
		StartedAt:    now,
		HealthURL:    ":8082",
		MetricsURL:   ":8083",
		Capabilities: []string{"alpha", "beta"},
		Metadata:     map[string]string{"region": "cn-shanghai"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var restored ServiceInfo
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if restored.Name != original.Name {
		t.Fatalf("Name mismatch: %q vs %q", restored.Name, original.Name)
	}
	if restored.Version != original.Version {
		t.Fatalf("Version mismatch: %q vs %q", restored.Version, original.Version)
	}
	if !restored.StartedAt.Equal(original.StartedAt) {
		t.Fatalf("StartedAt mismatch: %v vs %v", restored.StartedAt, original.StartedAt)
	}
	if restored.HealthURL != original.HealthURL {
		t.Fatalf("HealthURL mismatch: %q vs %q", restored.HealthURL, original.HealthURL)
	}
	if restored.MetricsURL != original.MetricsURL {
		t.Fatalf("MetricsURL mismatch: %q vs %q", restored.MetricsURL, original.MetricsURL)
	}
	if len(restored.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(restored.Capabilities))
	}
	if restored.Metadata["region"] != "cn-shanghai" {
		t.Fatalf("Metadata region mismatch: %q", restored.Metadata["region"])
	}
}

func TestServiceInfo_JSONEmptyFields(t *testing.T) {
	// 验证 omitempty 正确工作：空字符串不应出现在 JSON 中
	info := ServiceInfo{
		Name: "minimal",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// 只有 name 和 started_at 应出现（started_at 是零值 time.Time，也会序列化）
	if _, ok := result["version"]; ok {
		t.Fatal("empty version should be omitted")
	}
	if _, ok := result["health_url"]; ok {
		t.Fatal("empty health_url should be omitted")
	}
	if _, ok := result["metrics_url"]; ok {
		t.Fatal("empty metrics_url should be omitted")
	}
}

// ============================================================
// TestRenewal
// ============================================================

func TestRenewal_FailureDoesNotExit(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "renewal_failure_exit"}
	reg, err := Register(context.Background(), mock, info,
		WithInterval(30*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 注入 Set 失败，验证心跳 goroutine 不退出
	injectErr := fmt.Errorf("simulated set failure")
	mock.SetSetFailures(5, injectErr)

	// 等待足够时间让心跳执行几次续约
	time.Sleep(150 * time.Millisecond)

	// 验证心跳 goroutine 仍在运行（Shutdown 应正常完成）
	if err := reg.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed after renewal failures: %v", err)
	}

	// 验证 key 已被删除
	_, err = mock.Get(context.Background(), "services:renewal_failure_exit")
	if err == nil {
		t.Fatal("key should be deleted after shutdown")
	}
}

func TestRegister_RedisUnreachable(t *testing.T) {
	mock := pkgredis.NewMockClient()

	// 注入 Get 失败模拟 Redis 不可达
	mock.SetGetFailures(1, fmt.Errorf("redis unreachable"))

	info := ServiceInfo{Name: "unreachable_test"}
	reg, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("Register should succeed even when initial Get fails (transient): %v", err)
	}
	defer safeShutdown(t, reg)

	// 验证 key 被写入
	val, err := mock.Get(context.Background(), "services:unreachable_test")
	if err != nil {
		t.Fatalf("key should exist after register: %v", err)
	}
	if val == "" {
		t.Fatal("value should not be empty")
	}
}

// ============================================================
// 自定义 Mock：用于模拟 Set 阻塞场景
// ============================================================

// blockableMock 包装 MockRedisClient，通过 setBlockCh 控制 Set 行为。
// 当 setBlockCh 非 nil 时，Set 在此 channel 上等待，用于模拟心跳续约阻塞场景。
type blockableMock struct {
	*pkgredis.MockRedisClient
	setBlockCh chan struct{}
}

func (m *blockableMock) Set(ctx context.Context, key string, value interface{}, ttlSeconds int64) error {
	// 在真正的 Set 执行前，如果 setBlockCh 已设置，先阻塞等待
	if m.setBlockCh != nil {
		select {
		case <-m.setBlockCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.MockRedisClient.Set(ctx, key, value, ttlSeconds)
}

// TestShutdown_TimeoutStillDeletesKey 验证 Shutdown 超时后 key 仍被删除。
//
// 场景：心跳 goroutine 在执行 renew（Set 阻塞）时收到停止信号，
// Shutdown 等待 goroutine 退出超时，但仍需确保 Redis key 被清理。
func TestShutdown_TimeoutStillDeletesKey(t *testing.T) {
	baseMock := pkgredis.NewMockClient()
	mock := &blockableMock{MockRedisClient: baseMock}

	info := ServiceInfo{Name: "shutdown_timeout_del"}
	reg, err := Register(context.Background(), mock, info,
		WithInterval(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 注册完成后，使 Set 阻塞（心跳续约将卡在 Set 中）
	ch := make(chan struct{})
	mock.setBlockCh = ch

	// 等待一次心跳触发，确保心跳 goroutine 进入 renew 并阻塞在 Set 中
	time.Sleep(60 * time.Millisecond)

	// 用极短超时的 context 调用 Shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	if err := reg.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown should complete despite heartbeat timeout: %v", err)
	}

	// 验证 key 已被删除（Shutdown 使用独立 context 执行 Del）
	_, err = baseMock.Get(context.Background(), "services:shutdown_timeout_del")
	if err == nil {
		t.Fatal("key should be deleted after shutdown even when heartbeat goroutine timed out")
	}

	// 释放阻塞的 Set，让心跳 goroutine 得以退出
	close(ch)
}

// ============================================================
// helper
// ============================================================

// safeShutdown 安全关闭 Registry，忽略已关闭的错误。
func safeShutdown(t *testing.T, reg *Registry) {
	t.Helper()
	_ = reg.Shutdown(context.Background())
}
