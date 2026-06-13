// pkg/redis — 项目级共享 Redis 包
// 提供统一的 RedisClient 接口，所有 Go 服务引用此包进行 Redis 操作。
// 禁止各服务单独创建 go-redis 连接（ENGINEERING_GUIDE.md 规则 7）。
package redis

import "context"

// ============================================================
// 通用 option 类型
// ============================================================

// XReadArgs XREAD 命令参数
type XReadArgs struct {
	Streams []string // stream key 列表
	IDs     []string // 起始 ID 列表（与 Streams 一一对应）
	Block   int64    // 阻塞等待毫秒数（0 = 不阻塞）
	Count   int64    // 每次返回最大条数（0 = 不限）
}

// XReadGroupArgs XREADGROUP 命令参数
type XReadGroupArgs struct {
	Group    string   // 消费组名
	Consumer string   // 消费者名
	Streams  []string // stream key 列表
	IDs      []string // 起始 ID 列表（">" 表示新消息）
	Block    int64    // 阻塞等待毫秒数
	Count    int64    // 每次返回最大条数
}

// XMessage Stream 消息
type XMessage struct {
	ID     string
	Stream string
	Values map[string]interface{}
}

// ============================================================
// RedisClient — 统一 Redis 客户端接口
// 覆盖 Streams 操作 + KV 操作 + 基础命令，所有 Go 服务通过此接口操作 Redis
// ============================================================

type RedisClient interface {
	// ---- Streams 操作 ----
	XAdd(ctx context.Context, stream string, values map[string]interface{}) error
	XRead(ctx context.Context, args XReadArgs) ([]XMessage, error)
	XReadGroup(ctx context.Context, args XReadGroupArgs) ([]XMessage, error)
	XAck(ctx context.Context, stream, group string, ids ...string) error

	// ---- KV 操作 ----
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttlSeconds int64) error
	Del(ctx context.Context, keys ...string) error
	Keys(ctx context.Context, pattern string) ([]string, error)

	// ---- 基础命令 ----
	Ping(ctx context.Context) error
	Close() error
}
