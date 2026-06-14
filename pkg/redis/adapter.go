// pkg/redis — go-redis 适配器实现
package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ============================================================
// 常量
// ============================================================
const (
	DialTimeout  = 5 * time.Second
	ReadTimeout  = 3 * time.Second
	WriteTimeout = 3 * time.Second
	MaxRetries   = -1
)

// ============================================================
// GoRedisAdapter — 封装 *redis.Client 实现 RedisClient 接口
// ============================================================

type GoRedisAdapter struct {
	client *goredis.Client
}

// NewAdapter 创建 Redis 客户端适配器（统一初始化入口）
func NewAdapter(addr string) *GoRedisAdapter {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         addr,
		DialTimeout:  DialTimeout,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		MaxRetries:   MaxRetries,
	})
	return &GoRedisAdapter{client: rdb}
}

// ============================================================
// Streams 操作
// ============================================================

// XAdd 写入 Stream（MAXLEN ~ 10000 自动裁剪）
func (a *GoRedisAdapter) XAdd(ctx context.Context, stream string, values map[string]interface{}) error {
	return a.client.XAdd(ctx, &goredis.XAddArgs{
		Stream: stream,
		MaxLen: 10000,
		Approx: true,
		Values: values,
	}).Err()
}

// XRead 读取 Stream
// go-redis 的 Streams 是 interleaved 格式：[stream1, id1, stream2, id2, ...]
func (a *GoRedisAdapter) XRead(ctx context.Context, args XReadArgs) ([]XMessage, error) {
	if len(args.Streams) == 0 {
		return nil, fmt.Errorf("redis: XRead requires at least one stream")
	}

	// 构建 interleaved 数组
	streams := make([]string, 0, len(args.Streams)*2)
	for i, s := range args.Streams {
		streams = append(streams, s)
		if i < len(args.IDs) {
			streams = append(streams, args.IDs[i])
		} else {
			streams = append(streams, "0")
		}
	}

	msgs, err := a.client.XRead(ctx, &goredis.XReadArgs{
		Streams: streams,
		Block:   time.Duration(args.Block) * time.Millisecond,
		Count:   args.Count,
	}).Result()
	if err != nil {
		return nil, err
	}

	return convertMessages(msgs), nil
}

// XReadGroup 消费组读取 Stream
func (a *GoRedisAdapter) XReadGroup(ctx context.Context, args XReadGroupArgs) ([]XMessage, error) {
	if len(args.Streams) == 0 {
		return nil, fmt.Errorf("redis: XReadGroup requires at least one stream")
	}

	// 构建 interleaved 数组：[stream1, id1, stream2, id2, ...]
	streams := make([]string, 0, len(args.Streams)*2)
	for i, s := range args.Streams {
		streams = append(streams, s)
		if i < len(args.IDs) {
			streams = append(streams, args.IDs[i])
		} else {
			streams = append(streams, ">")
		}
	}

	msgs, err := a.client.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    args.Group,
		Consumer: args.Consumer,
		Streams:  streams,
		Count:    args.Count,
		Block:    time.Duration(args.Block) * time.Millisecond,
	}).Result()
	if err != nil {
		return nil, err
	}

	return convertMessages(msgs), nil
}

// XAck 确认消息
func (a *GoRedisAdapter) XAck(ctx context.Context, stream, group string, ids ...string) error {
	return a.client.XAck(ctx, stream, group, ids...).Err()
}

// ============================================================
// KV 操作
// ============================================================

// Get 读取 key
func (a *GoRedisAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}

// Set 写入 key（带 TTL）
func (a *GoRedisAdapter) Set(ctx context.Context, key string, value interface{}, ttlSeconds int64) error {
	if ttlSeconds <= 0 {
		return a.client.Set(ctx, key, value, 0).Err()
	}
	return a.client.Set(ctx, key, value, time.Duration(ttlSeconds)*time.Second).Err()
}

// Del 删除 key
func (a *GoRedisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.client.Del(ctx, keys...).Err()
}

// Keys 模糊查询 key（仅低频操作，禁止高频使用）
func (a *GoRedisAdapter) Keys(ctx context.Context, pattern string) ([]string, error) {
	return a.client.Keys(ctx, pattern).Result()
}

// XLen 获取 Stream 消息数量
func (a *GoRedisAdapter) XLen(ctx context.Context, stream string) (int64, error) {
	return a.client.XLen(ctx, stream).Result()
}

// XGroupCreate 创建消费组（MKSTREAM 确保 stream 存在）
func (a *GoRedisAdapter) XGroupCreate(ctx context.Context, stream, group, start string) error {
	return a.client.XGroupCreateMkStream(ctx, stream, group, start).Err()
}

// Expire 设置 key 的 TTL（秒）
func (a *GoRedisAdapter) Expire(ctx context.Context, key string, ttlSeconds int64) error {
	return a.client.Expire(ctx, key, time.Duration(ttlSeconds)*time.Second).Err()
}

// ============================================================
// 基础命令
// ============================================================

// Ping 健康检查
func (a *GoRedisAdapter) Ping(ctx context.Context) error {
	return a.client.Ping(ctx).Err()
}

// Close 释放连接池
func (a *GoRedisAdapter) Close() error {
	return a.client.Close()
}

// ============================================================
// helpers
// ============================================================

func convertMessages(msgs []goredis.XStream) []XMessage {
	result := make([]XMessage, 0)
	for _, stream := range msgs {
		for _, msg := range stream.Messages {
			values := make(map[string]interface{}, len(msg.Values))
			for k, v := range msg.Values {
				values[k] = v
			}
			result = append(result, XMessage{
				ID:     msg.ID,
				Stream: stream.Stream,
				Values: values,
			})
		}
	}
	return result
}
