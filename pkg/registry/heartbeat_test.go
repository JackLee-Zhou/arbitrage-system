package registry

import (
	"context"
	"strings"
	"testing"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// ============================================================
// TestRegisterAndKeepAlive
// ============================================================

func TestRegisterAndKeepAlive_Success(t *testing.T) {
	mock := pkgredis.NewMockClient()
	now := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)

	info := ServiceInfo{
		Name:         "keepalive_test",
		Version:      "1.0.0",
		StartedAt:    now,
		HealthURL:    ":8081",
		Capabilities: []string{"test"},
	}

	cancel, err := RegisterAndKeepAlive(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("RegisterAndKeepAlive failed: %v", err)
	}
	if cancel == nil {
		t.Fatal("cancel function should not be nil")
	}
	defer cancel()

	// 验证 key 已写入
	val, err := mock.Get(context.Background(), "services:keepalive_test")
	if err != nil {
		t.Fatalf("key should exist after register: %v", err)
	}
	if val == "" {
		t.Fatal("value should not be empty")
	}
}

func TestRegisterAndKeepAlive_EmptyName(t *testing.T) {
	mock := pkgredis.NewMockClient()
	info := ServiceInfo{Name: ""}

	_, err := RegisterAndKeepAlive(context.Background(), mock, info)
	if err == nil {
		t.Fatal("expected error for empty service name, got nil")
	}
	if !strings.Contains(err.Error(), "service name cannot be empty") {
		t.Fatalf("expected 'service name cannot be empty', got: %v", err)
	}
}

func TestRegisterAndKeepAlive_NilClient(t *testing.T) {
	info := ServiceInfo{Name: "nil_client_test"}

	_, err := RegisterAndKeepAlive(context.Background(), nil, info)
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
	if !strings.Contains(err.Error(), "redis client cannot be nil") {
		t.Fatalf("expected 'redis client cannot be nil', got: %v", err)
	}
}

func TestRegisterAndKeepAlive_CancelDeletesKey(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{
		Name:      "cancel_del_test",
		Version:   "1.0.0",
		HealthURL: ":8081",
	}

	cancel, err := RegisterAndKeepAlive(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("RegisterAndKeepAlive failed: %v", err)
	}

	// 验证 key 存在
	_, err = mock.Get(context.Background(), "services:cancel_del_test")
	if err != nil {
		t.Fatalf("key should exist after register: %v", err)
	}

	// 调用 cancel
	cancel()

	// 验证 key 已删除
	_, err = mock.Get(context.Background(), "services:cancel_del_test")
	if err == nil {
		t.Fatal("key should be deleted after cancel")
	}
}

func TestRegisterAndKeepAlive_CancelIdempotent(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{Name: "cancel_idempotent_test"}

	cancel, err := RegisterAndKeepAlive(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("RegisterAndKeepAlive failed: %v", err)
	}

	// 多次调用 cancel 应安全（幂等）
	cancel()
	cancel()
	cancel()

	// key 应已被删除
	_, err = mock.Get(context.Background(), "services:cancel_idempotent_test")
	if err == nil {
		t.Fatal("key should be deleted after cancel")
	}
}

// ============================================================
// TestKeepAlive
// ============================================================

func TestKeepAlive_Wrapper(t *testing.T) {
	mock := pkgredis.NewMockClient()

	info := ServiceInfo{
		Name:      "keepalive_wrapper_test",
		Version:   "1.0.0",
		HealthURL: ":8081",
	}

	reg, err := Register(context.Background(), mock, info)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	cancel := KeepAlive(reg)
	if cancel == nil {
		t.Fatal("KeepAlive cancel should not be nil")
	}

	// 验证 key 存在
	_, err = mock.Get(context.Background(), "services:keepalive_wrapper_test")
	if err != nil {
		t.Fatalf("key should exist after register: %v", err)
	}

	cancel()

	// 验证 key 已删除
	_, err = mock.Get(context.Background(), "services:keepalive_wrapper_test")
	if err == nil {
		t.Fatal("key should be deleted after KeepAlive cancel")
	}
}
