// trading_agent/domain/engine — 引擎端口接口
//
// 本文件定义引擎所需的全部外部依赖接口（Ports），
// 由 adapters/ 层实现，domain/ 零外部依赖。
package engine

// Tick 引擎层行情结构
// 注意：不同于 adapters/broker 的 Tick，本结构包含高低价用于 ATR 计算
type Tick struct {
	Symbol    string
	Timestamp int64
	LastPrice float64
	HighPrice float64 // 当日最高价（CTP 提供，用于 ATR；0 表示不可用）
	LowPrice  float64 // 当日最低价（CTP 提供，用于 ATR；0 表示不可用）
	Volume    int64   // 累积成交量（引擎内部转换为增量）
}

// AIScore AI 评分结果
type AIScore struct {
	Score      float64
	Confidence float64
	Timestamp  int64 // Unix 秒，用于评分老化
}

// OrderRequest 报单请求
type OrderRequest struct {
	Symbol    string
	Direction string // "buy" / "sell"
	Offset    string // "open" / "close"
	Volume    int
	Price     float64 // 0 = 市价单
	OrderType string  // "limit" / "market"
}

// OrderID 报单 ID
type OrderID string

// PositionState 持仓快照（持久化用）
type PositionState struct {
	Symbol    string
	Direction string
	Volume    int
	AvgPrice  float64
}

// SignalRecord 信号流水记录（持久化用）
type SignalRecord struct {
	Symbol    string
	Action    string
	Strength  float64
	Reason    string
	Timestamp int64
}

// StateChangeEvent 状态变更事件（通知用）
type StateChangeEvent struct {
	Type      string // "signal_generated" / "order_placed" / "emergency_triggered" / "mode_changed"
	Symbol    string
	Message   string
	Timestamp int64
}

// VolatilityData 波动数据（紧急分析用）
type VolatilityData struct {
	PriceChangePct1m float64
	VolumeMultiplier float64
}

// Mode 引擎模式
type Mode string

const (
	ModeNormal Mode = "normal"    // 正常开平仓
	ModeSafe   Mode = "safe_mode" // 只平不开
	ModePaused Mode = "paused"    // 暂停所有信号
)

// EngineStatus 引擎状态快照
type EngineStatus struct {
	Mode            Mode
	Symbol          string
	LastPrice       float64
	CurrentPosition int
	DailyPnL        float64
	LastEmergencyAt int64 // Unix 秒

	// gRPC 查询用字段
	LastTickTime   int64   // 最后 Tick 时间 (unix ms)
	LastSignalTime int64   // 最后信号时间 (unix ms)
	CurrentAIScore float64 // 当前 AI 评分
	AIScoreDecay   float64 // AI 评分老化系数
	OpenOrders     int32   // 未成交报单数
}

// --- 端口接口 ---

// TickSource 行情数据源
// 适配器实现：从 Redis Stream 消费 tick:MA2509
type TickSource interface {
	Next() (*Tick, error)
}

// AIScoreReader AI 评分读取器
// 适配器实现：从 Redis KV 读取 ai_score:SYMBOL:live
type AIScoreReader interface {
	GetLatest(symbol string) (*AIScore, error)
}

// OrderExecutor 报单执行器
// 适配器实现：通过 Broker 接口报单（CTP / QMT）
// 注意：domain 层定义自己的接口，不直接 import adapters/broker
type OrderExecutor interface {
	PlaceOrder(req OrderRequest) (OrderID, error)
	CancelOrder(orderID string) error
}

// StatePersister 状态持久化器
// 适配器实现：写 Redis KV（position:current、engine:status 等）
type StatePersister interface {
	SavePosition(state *PositionState) error
	SaveEngineStatus(status *EngineStatus) error
	SaveSignal(record *SignalRecord) error
}

// StateChangeNotifier 状态变更通知器
// 适配器实现：gRPC SubscribeStateChanges server stream
type StateChangeNotifier interface {
	Notify(event *StateChangeEvent) error
}

// LLMClient 紧急分析客户端
// 适配器实现：调 llm_scorer /emergency_analysis 端点
type LLMClient interface {
	RequestEmergencyAnalysis(symbol string, reason string, data *VolatilityData) error
}
