# pkg/registry — 服务注册/心跳/注销

统一封装服务注册到 Redis 的完整生命周期，所有 Go 服务通过此包上报自身状态。

## 职责

- **注册** — 服务启动时写入 `SET services:<name> <JSON> EX <ttl>`
- **心跳** — 后台 goroutine 每 interval 刷新 TTL，失败仅记 WARN 不退出
- **注销** — 停止心跳 + `DEL services:<name>`

## Redis Key 规范

```
services:<name>  →  ServiceInfo 的 JSON 序列化
```

示例：

```
services:ctp_tick  →  {"name":"ctp_tick","version":"1.0.0","started_at":"..."}
services:watchdog  →  {"name":"watchdog","version":"1.0.0","capabilities":["health_check"]}
```

符合 ENGINEERING_GUIDE.md Redis Key 命名规范。

## 使用方式

### 简易方式（推荐）

`RegisterAndKeepAlive` 返回一个 cancel 函数，适合大多数场景：

```go
import (
    "context"
    "log"

    pkgredis "github.com/arbitrage-system/pkg/redis"
    "github.com/arbitrage-system/pkg/registry"
)

func main() {
    ctx := context.Background()
    client := pkgredis.NewAdapter("redis:6379")

    cancel, err := registry.RegisterAndKeepAlive(ctx, client, registry.ServiceInfo{
        Name:      "my_service",
        Version:   "1.0.0",
        HealthURL: ":8081",
        Capabilities: []string{"tick", "focus_push"},
    })
    if err != nil {
        log.Fatalf("注册失败: %v", err)
    }
    defer cancel()

    // ... 业务逻辑 ...
}
```

### 标准方式（支持自定义选项）

```go
reg, err := registry.Register(ctx, client, registry.ServiceInfo{
    Name:      "my_service",
    Version:   "1.0.0",
    HealthURL: ":8081",
    Capabilities: []string{"tick", "focus_push"},
})
if err != nil {
    log.Fatalf("注册失败: %v", err)
}
defer reg.Shutdown(ctx)
```

#### 自定义选项

```go
reg, err := registry.Register(ctx, client, info,
    registry.WithTTL(60),                     // TTL 60 秒（默认 30）
    registry.WithInterval(15*time.Second),    // 续约间隔 15 秒（默认 10s）
)
```

#### 超时控制

```go
// 5 秒超时
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := reg.Shutdown(ctx); err != nil {
    log.Printf("注销超时: %v", err)
}
```

### 从 Registry 转为 cancel

```go
reg, err := registry.Register(ctx, client, info)
// ...
cancel := registry.KeepAlive(reg)
defer cancel()
```

## ServiceInfo 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `Name` | string | 是 | 服务名，如 `ctp_tick` |
| `Version` | string | 否 | 版本号 |
| `StartedAt` | time.Time | 否 | 启动时间（ISO8601） |
| `HealthURL` | string | 否 | 健康检查地址 |
| `MetricsURL` | string | 否 | 指标地址 |
| `GRPCAddr` | string | 否 | gRPC 地址 |
| `Capabilities` | []string | 否 | 能力列表 |
| `Metadata` | map[string]string | 否 | 扩展字段 |

## 设计决策

1. **心跳失败只记 WARN 不退出** — Redis 瞬断不应杀死服务。心跳 goroutine 持续运行，Redis 恢复后自动续约。
2. **Shutdown 超时保护** — 默认 5s 超时后强制删除 Redis key，避免僵尸注册。
3. **幂等 Shutdown** — 多次调用安全，仅第一次实际执行。
4. **Option 模式** — 参数可扩展，新增配置项不破坏已有调用方。
5. **零额外依赖** — 仅依赖 `github.com/arbitrage-system/pkg/redis` 和标准库（`encoding/json` + `log/slog`）。

## 单元测试

```bash
cd pkg/registry
go test -v -count=1 ./...
```

使用 `pkg/redis.NewMockClient()` 模拟 Redis，不依赖外部服务。

## 集成测试

在真 Redis 上测试心跳续约（当前包内无集成测试，由各服务自行验证）。
