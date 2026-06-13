# pkg/redis — 项目级共享 Redis 包

所有 Go 服务通过此包统一连接 Redis，禁止各服务独立创建 `*redis.Client`。

## 使用方式

### 连接

```go
import pkgredis "github.com/arbitrage-system/pkg/redis"

// 创建适配器（Docker Compose 环境下 addr = "redis:6379"）
client := pkgredis.NewAdapter("redis:6379")
defer client.Close()
```

### Streams 写入（ctp_tick / trading_agent 等）

```go
values := map[string]interface{}{
    "symbol": "MA2509",
    "price":  2350.0,
}
err := client.XAdd(ctx, "tick:MA2509", values)
```

### Streams 消费（trading_agent / db_writer / llm_scorer 等）

```go
msgs, err := client.XRead(ctx, pkgredis.XReadArgs{
    Streams: []string{"tick:MA2509"},
    IDs:     []string{"$"},
    Block:   5000, // 阻塞 5 秒等待
    Count:   10,
})
```

### KV 操作（orchestrator 状态聚合）

```go
score, err := client.Get(ctx, "ai_score:MA2509:live")
err = client.Set(ctx, "engine:status", "running", 30)
```

### 单元测试（Mock）

```go
mock := pkgredis.NewMockClient()
writer := NewTickWriter(mock) // 注入 MockRedisClient
```

## 接口定义

见 `client.go` 的 `RedisClient` 接口。GoRedisAdapter 是真实实现，MockRedisClient 是测试 mock。

## 设计原则

- **统一入口**：所有 Go 服务通过同一个包连接 Redis（ENGINEERING_GUIDE.md 规则 7）
- **接口隔离**：服务只依赖 `RedisClient` 接口，不依赖具体实现
- **测试友好**：MockRedisClient 内存实现，unit 测试无需真 Redis
