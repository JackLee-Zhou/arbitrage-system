// orchestrator/adapters/redis — Redis 只读查询适配器
// orchestrator 不写 Redis（审计日志通过 db_writer 间接落库），只读取其他服务发布的数据
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// Reader 从 Redis 读取各服务发布的数据
type Reader struct {
	client pkgredis.RedisClient
}

// NewReader 创建 Redis 只读适配器
func NewReader(client pkgredis.RedisClient) *Reader {
	return &Reader{client: client}
}

// Ping 检查 Redis 连通性
func (r *Reader) Ping(ctx context.Context) error {
	return r.client.Ping(ctx)
}

// AIScore 读取指定 symbol 的 AI 评分
func (r *Reader) AIScore(ctx context.Context, symbol string) (map[string]interface{}, error) {
	key := fmt.Sprintf("ai_score:%s:live", symbol)
	return r.getJSON(ctx, key)
}

// Position 读取当前持仓
func (r *Reader) Position(ctx context.Context) (map[string]interface{}, error) {
	return r.getJSON(ctx, "position:current")
}

// EngineStatus 读取交易引擎状态
func (r *Reader) EngineStatus(ctx context.Context) (map[string]interface{}, error) {
	return r.getJSON(ctx, "engine_status")
}

// Services 读取所有已注册服务的信息
func (r *Reader) Services(ctx context.Context) (map[string]json.RawMessage, error) {
	keys, err := r.client.Keys(ctx, "services:*")
	if err != nil {
		return nil, err
	}
	result := make(map[string]json.RawMessage, len(keys))
	for _, key := range keys {
		raw, err := r.client.Get(ctx, key)
		if err != nil || raw == "" {
			continue
		}
		result[key] = json.RawMessage(raw)
	}
	return result, nil
}

// Indicator 读取指定 symbol 的最新指标
func (r *Reader) Indicator(ctx context.Context, symbol string) (map[string]interface{}, error) {
	key := fmt.Sprintf("indicator:%s:current", symbol)
	return r.getJSON(ctx, key)
}

// getJSON 从 Redis 读取并解析 JSON
func (r *Reader) getJSON(ctx context.Context, key string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	raw, err := r.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("redis GET %s: %w", key, err)
	}
	if raw == "" {
		return nil, fmt.Errorf("redis key %s not found", key)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("json parse %s: %w", key, err)
	}
	return result, nil
}
