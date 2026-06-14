// pkg/registry — ServiceInfo 与简易注册 API
//
// 本文件提供 ServiceInfo 类型定义以及 RegisterAndKeepAlive 简易注册函数，
// 适合只需要默认 TTL/间隔、无需自定义选项的场景。
package registry

import (
	"context"

	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// RegisterAndKeepAlive 注册服务并自动维持心跳，返回一个 cancel 函数用于优雅注销。
//
// 与 Register 不同之处：
//   - 不返回 *Registry 类型，调用方无需了解内部细节
//   - 返回的 cancel 函数内部调用 Shutdown，默认超时 5s
//   - 使用默认 TTL（30s）和默认心跳间隔（10s），不支持自定义选项
//
// 典型用法：
//
//	cancel, err := registry.RegisterAndKeepAlive(ctx, redisClient, registry.ServiceInfo{
//	    Name:      "my_service",
//	    Version:   "1.0.0",
//	    HealthURL: ":8081",
//	})
//	if err != nil {
//	    log.Fatalf("注册服务失败: %v", err)
//	}
//	defer cancel()
func RegisterAndKeepAlive(ctx context.Context, client pkgredis.RedisClient, info ServiceInfo) (cancel func(), err error) {
	reg, err := Register(ctx, client, info)
	if err != nil {
		return nil, err
	}

	cancelFn := func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer shutdownCancel()
		_ = reg.Shutdown(shutdownCtx)
	}
	return cancelFn, nil
}

// KeepAlive 为已注册的 *Registry 创建一个 cancel 风格的封装。
//
// 适用于已通过 Register 创建 Registry，但希望以 cancel() 风格统一管理的场景。
//
//	reg, err := registry.Register(ctx, client, info)
//	cancel := registry.KeepAlive(reg)
//	defer cancel()
func KeepAlive(reg *Registry) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer cancel()
		_ = reg.Shutdown(ctx)
	}
}
