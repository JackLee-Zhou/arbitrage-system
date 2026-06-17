// trading_agent/adapters/redis — Redis Stream 消费 + KV 状态读写
//
// Stream 消费（XREADGROUP + XACK 模式）：
//   - tick:SYMBOL（行情订阅）
//
// KV 状态读写（SET/GET with JSON serialization）：
//   - ai_score:SYMBOL:live      — 常规 AI 评分
//   - ai_score:SYMBOL:emergency — 紧急 AI 评分
//   - position:current          — 当前持仓
//   - engine:status             — 引擎运行状态
//
// Stream 发布（供 db_writer 消费写 InfluxDB）：
//   - signal:SYMBOL             — 策略信号
//   - write:order               — 报单事件
//   - write:position_snapshot   — 持仓快照
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"
)

// ============================================================
// 常量
// ============================================================
const (
	consumerGroupName  = "trading_agent"
	xReadBlockMs       = 5000
	xReadBatchSize     = 10
	retryMaxAttempts   = 3
	retryBackoffBase   = 500 * time.Millisecond
	writeTimeout       = 3 * time.Second
	positionTTLSeconds = int64(3600) // 持仓快照 1 小时
	statusTTLSeconds   = int64(60)   // 引擎状态 1 分钟
)

// ============================================================
// 数据模型（适配器层 DTO，与 contracts/proto 对齐）
// ============================================================

// Tick 行情数据
type Tick struct {
	Symbol    string
	Timestamp int64
	LastPrice float64
	Volume    int64
}

// AIScore AI 评分
type AIScore struct {
	Symbol     string   `json:"symbol"`
	Timestamp  int64    `json:"timestamp"`
	ExpiresAt  int64    `json:"expires_at"`
	Score      float64  `json:"score"`
	Confidence float64  `json:"confidence"`
	Horizon    string   `json:"horizon"`
	Regime     string   `json:"regime"`
	KeyDrivers []string `json:"key_drivers,omitempty"`
	RawText    string   `json:"raw_text,omitempty"`
}

// EmergencyAIScore 紧急 AI 评分
type EmergencyAIScore struct {
	AIScore

	TriggerReason string `json:"trigger_reason"`

	Volatility struct {
		PriceChangePct1m float64 `json:"price_change_pct_1m"`
		VolumeMultiplier float64 `json:"volume_multiplier"`
		OneSidedRatio    float64 `json:"one_sided_ratio"`
	} `json:"volatility"`

	BreakingNews     []string `json:"breaking_news,omitempty"`
	SuggestedAction  string   `json:"suggested_action"`
	ActionConfidence float64  `json:"action_confidence"`
}

// Position 持仓快照
type Position struct {
	Symbol    string  `json:"symbol"`
	Direction string  `json:"direction"` // "long" / "short"
	Volume    int     `json:"volume"`
	AvgPrice  float64 `json:"avg_price"`
	LastPrice float64 `json:"last_price"`
	FloatPnL  float64 `json:"float_pnl"`
}

// EngineStatus 引擎运行状态
type EngineStatus struct {
	Status          string  `json:"status"` // "normal" / "safe_mode" / "paused"
	Timestamp       int64   `json:"timestamp"`
	Mode            string  `json:"mode"`
	Symbol          string  `json:"symbol"`
	LastPrice       float64 `json:"last_price"`
	CurrentPosition int     `json:"current_position"`
	DailyPnL        float64 `json:"daily_pnl"`
	LastEmergencyAt int64   `json:"last_emergency_at"`
}

// Signal 策略信号
type Signal struct {
	Symbol    string  `json:"symbol"`
	Action    string  `json:"action"` // "buy" / "sell" / "hold"
	Strength  float64 `json:"strength"`
	Reason    string  `json:"reason"`
	Timestamp int64   `json:"timestamp"`
}

// OrderEvent 报单事件
type OrderEvent struct {
	OrderID     string  `json:"order_id"`
	DecisionID  string  `json:"decision_id"`
	Symbol      string  `json:"symbol"`
	Direction   string  `json:"direction"` // "buy" / "sell"
	Offset      string  `json:"offset"`    // "open" / "close" / "close_today"
	Volume      int32   `json:"volume"`
	Price       float64 `json:"price"`
	OrderType   string  `json:"order_type"` // "limit" / "market"
	Status      string  `json:"status"`     // "submitted" / "filled" / "cancelled" / "rejected"
	FilledPrice float64 `json:"filled_price"`
	FilledVol   int32   `json:"filled_volume"`
	Timestamp   int64   `json:"timestamp"`
	Message     string  `json:"message,omitempty"`
}

// ============================================================
// TickSubscriber — Redis Streams 消费者（XREADGROUP 模式）
// ============================================================

// TickSubscriber 使用 XREADGROUP + XACK 消费 tick:SYMBOL 流。
// 支持消费组模式，可多实例并行消费（但每个 tick 只投递到一个消费者）。
type TickSubscriber struct {
	client       pkgredis.RedisClient
	consumerName string
}

// NewTickSubscriber 创建 Tick 订阅器
func NewTickSubscriber(client pkgredis.RedisClient) *TickSubscriber {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return &TickSubscriber{
		client:       client,
		consumerName: consumerGroupName + "_" + hostname,
	}
}

// Subscribe 订阅 tick Stream，通过回调推送。
// Stream: tick:SYMBOL（例如 tick:MA2509）
// 使用 XREADGROUP + XACK 消费模式：
//   - 自动创建消费组（忽略 BUSYGROUP）
//   - 阻塞等待新消息
//   - 成功回调后 XACK 确认
//
// ctx 取消时返回；handler 在消费 goroutine 中同步调用。
func (s *TickSubscriber) Subscribe(ctx context.Context, symbol string, handler func(Tick)) error {
	stream := fmt.Sprintf("tick:%s", symbol)

	// 创建消费组（已存在返回 BUSYGROUP，忽略）
	if err := s.client.XGroupCreate(ctx, stream, consumerGroupName, "$"); err != nil {
		if !isBusyGroupError(err) {
			log.Printf("[redis.subscriber] xgroupcreate error (stream=%s): %v", stream, err)
		}
	}

	log.Printf("[redis.subscriber] start consuming stream=%s group=%s consumer=%s",
		stream, consumerGroupName, s.consumerName)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[redis.subscriber] context done, stop stream=%s", stream)
			return ctx.Err()
		default:
		}

		msgs, err := s.client.XReadGroup(ctx, pkgredis.XReadGroupArgs{
			Group:    consumerGroupName,
			Consumer: s.consumerName,
			Streams:  []string{stream},
			IDs:      []string{">"},
			Block:    xReadBlockMs,
			Count:    xReadBatchSize,
		})
		if err != nil {
			if err != context.DeadlineExceeded && err != context.Canceled {
				log.Printf("[redis.subscriber] xreadgroup error (stream=%s): %v", stream, err)
				// 非超时错误退避，避免空转
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
				}
			}
			continue
		}

		for _, msg := range msgs {
			tick := parseTick(msg.Values)
			if tick != nil {
				handler(*tick)
			}
			// 无论解析是否成功都 ACK，防止损坏数据阻塞消费
			if err := s.client.XAck(ctx, stream, consumerGroupName, msg.ID); err != nil {
				log.Printf("[redis.subscriber] xack error (stream=%s, id=%s): %v", stream, msg.ID, err)
			}
		}
	}
}

// parseTick 从 Redis Stream 消息中解析 Tick。
// 优先解析扁平字段（symbol / timestamp / price / volume），
// 降级尝试从 json_data 字段（JSON 字符串）解析。
func parseTick(values map[string]interface{}) *Tick {
	if len(values) == 0 {
		return nil
	}

	// 尝试从扁平字段解析
	symbol, _ := toString(values["symbol"])
	if symbol != "" {
		tick := &Tick{Symbol: symbol}
		if ts, ok := toInt64(values["timestamp"]); ok {
			tick.Timestamp = ts
		}
		if price, ok := toFloat64(values["price"]); ok {
			tick.LastPrice = price
		}
		if vol, ok := toInt64(values["volume"]); ok {
			tick.Volume = vol
		}
		return tick
	}

	// 降级：从 json_data 字段解析完整 JSON
	if raw, ok := toString(values["json_data"]); ok && raw != "" {
		var tick Tick
		if err := json.Unmarshal([]byte(raw), &tick); err == nil && tick.Symbol != "" {
			return &tick
		}
	}

	return nil
}

// ============================================================
// StateStore — Redis KV 状态读写 + Stream 发布
// ============================================================

// StateStore 封装 Redis KV 操作和 Stream 发布操作。
// KV: AI 评分 / 持仓快照 / 引擎状态
// Stream: 信号 / 报单事件 / 持仓快照（供 db_writer 消费写入 InfluxDB）
type StateStore struct {
	client pkgredis.RedisClient
}

// NewStateStore 创建状态存储器
func NewStateStore(client pkgredis.RedisClient) *StateStore {
	return &StateStore{client: client}
}

// ---- AI 评分读取 ----

// GetAIScore 读取常规 AI 评分。
// Redis Key: ai_score:SYMBOL:live
func (s *StateStore) GetAIScore(ctx context.Context, symbol string) (*AIScore, error) {
	key := fmt.Sprintf("ai_score:%s:live", symbol)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get ai_score from %s failed: %w", key, err)
	}
	var score AIScore
	if err := json.Unmarshal([]byte(data), &score); err != nil {
		return nil, fmt.Errorf("json unmarshal ai_score from %s failed: %w", key, err)
	}
	return &score, nil
}

// GetEmergencyScore 读取紧急 AI 评分。
// Redis Key: ai_score:SYMBOL:emergency
func (s *StateStore) GetEmergencyScore(ctx context.Context, symbol string) (*EmergencyAIScore, error) {
	key := fmt.Sprintf("ai_score:%s:emergency", symbol)
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get emergency score from %s failed: %w", key, err)
	}
	var score EmergencyAIScore
	if err := json.Unmarshal([]byte(data), &score); err != nil {
		return nil, fmt.Errorf("json unmarshal emergency score from %s failed: %w", key, err)
	}
	return &score, nil
}

// ---- 持仓快照 ----

// SavePosition 保存当前持仓快照。
// Redis Key: position:current, TTL: 1 小时
func (s *StateStore) SavePosition(ctx context.Context, pos *Position) error {
	data, err := json.Marshal(pos)
	if err != nil {
		return fmt.Errorf("json marshal position failed: %w", err)
	}
	key := "position:current"
	if err := s.client.Set(ctx, key, string(data), positionTTLSeconds); err != nil {
		return fmt.Errorf("set %s failed: %w", key, err)
	}
	return nil
}

// GetPosition 读取当前持仓快照。
// Redis Key: position:current
func (s *StateStore) GetPosition(ctx context.Context, symbol string) (*Position, error) {
	key := "position:current"
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get position from %s failed: %w", key, err)
	}
	var pos Position
	if err := json.Unmarshal([]byte(data), &pos); err != nil {
		return nil, fmt.Errorf("json unmarshal position from %s failed: %w", key, err)
	}
	return &pos, nil
}

// ---- 引擎状态 ----

// SaveEngineStatus 保存引擎运行状态。
// Redis Key: engine:status, TTL: 1 分钟
func (s *StateStore) SaveEngineStatus(ctx context.Context, status EngineStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("json marshal engine status failed: %w", err)
	}
	key := "engine:status"
	if err := s.client.Set(ctx, key, string(data), statusTTLSeconds); err != nil {
		return fmt.Errorf("set %s failed: %w", key, err)
	}
	return nil
}

// ---- Stream 发布（供 db_writer 消费写 InfluxDB） ----

// PublishSignal 发布策略信号到 Stream。
// Stream: signal:SYMBOL
func (s *StateStore) PublishSignal(ctx context.Context, signal *Signal) error {
	stream := fmt.Sprintf("signal:%s", signal.Symbol)
	return s.publishWithRetry(ctx, stream, signal)
}

// PublishOrderEvent 发布报单事件到 Stream。
// Stream: write:order
func (s *StateStore) PublishOrderEvent(ctx context.Context, event *OrderEvent) error {
	return s.publishWithRetry(ctx, "write:order", event)
}

// PublishPositionSnapshot 发布持仓快照到 Stream。
// Stream: write:position_snapshot
func (s *StateStore) PublishPositionSnapshot(ctx context.Context, pos *Position) error {
	return s.publishWithRetry(ctx, "write:position_snapshot", pos)
}

// publishWithRetry 通用 Stream 发布（带指数退避重试）
func (s *StateStore) publishWithRetry(ctx context.Context, stream string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("json marshal failed: %w", err)
	}

	values := map[string]interface{}{
		"data": string(data),
	}

	var lastErr error
	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		if attempt > 0 {
			backoff := retryBackoffBase * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
		err := s.client.XAdd(writeCtx, stream, values)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("[redis.store] xadd to %s failed (attempt %d/%d): %v",
			stream, attempt+1, retryMaxAttempts, err)
	}
	return fmt.Errorf("xadd to %s failed after %d retries: %w", stream, retryMaxAttempts, lastErr)
}

// ============================================================
// 类型转换工具
// ============================================================

// toString 安全转换为 string
func toString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	default:
		return "", false
	}
}

// toFloat64 从 interface{} 安全解析 float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		return f, err == nil
	case int64:
		return float64(val), true
	case int:
		return float64(val), true
	default:
		return 0, false
	}
}

// toInt64 从 interface{} 安全解析 int64
func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case string:
		i, err := strconv.ParseInt(val, 10, 64)
		return i, err == nil
	case float64:
		return int64(val), true
	case int:
		return int64(val), true
	default:
		return 0, false
	}
}

// isBusyGroupError 判断是否为消费组已存在错误
// 使用大小写不敏感匹配，兼容不同 Redis 版本的错误消息格式
func isBusyGroupError(err error) bool {
	return strings.Contains(strings.ToUpper(err.Error()), "BUSYGROUP")
}
