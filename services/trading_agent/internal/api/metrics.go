// trading_agent/api — /metrics HTTP 端点
//
// Prometheus 格式指标输出。
// 使用 sync/atomic 维护计数器/仪表盘值，纯 net/http 输出 text/plain 格式。
package api

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// labeledCounter 带标签的计数器（按 label 维度统计）
type labeledCounter struct {
	mu     sync.Mutex
	values map[string]*atomic.Int64
}

func newLabeledCounter() *labeledCounter {
	return &labeledCounter{values: make(map[string]*atomic.Int64)}
}

// Inc 增加指定标签的计数
func (c *labeledCounter) Inc(label string, delta int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.values[label]
	if !ok {
		v = &atomic.Int64{}
		c.values[label] = v
	}
	v.Add(delta)
}

// Snapshot 返回所有标签的当前值快照
func (c *labeledCounter) Snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.values))
	for k, v := range c.values {
		out[k] = v.Load()
	}
	return out
}

// MetricsRegistry Prometheus 指标注册表
//
// 暴露的指标清单：
//   - trading_agent_ticks_processed_total       counter   已处理 Tick 数
//   - trading_agent_signals_generated_total     counter   生成的信号数（按 action 分 label）
//   - trading_agent_orders_placed_total         counter   已报单数（按 direction 分 label）
//   - trading_agent_orders_rejected_total       counter   风控拒绝数
//   - trading_agent_emergency_triggers_total    counter   紧急触发次数
//   - trading_agent_ai_score_decay              gauge     当前 AI 评分老化系数
//   - trading_agent_engine_mode                 gauge     引擎模式（0=normal, 1=safe_mode, 2=paused）
//   - trading_agent_position_volume             gauge     当前持仓手数
//   - trading_agent_float_pnl                   gauge     浮动盈亏
//   - trading_agent_open_orders                 gauge     未成交报单数
//   - trading_agent_uptime_seconds              gauge     服务运行时长
type MetricsRegistry struct {
	startTime time.Time

	// ======== Counters ========
	ticksProcessedTotal    atomic.Int64
	signalsGeneratedTotal  *labeledCounter // label: action (buy/sell/hold)
	ordersPlacedTotal      *labeledCounter // label: direction (long/short)
	ordersRejectedTotal    atomic.Int64
	emergencyTriggersTotal atomic.Int64

	// ======== Gauges ========
	aiScoreDecay   atomic.Uint64 // float64 以 bits 存储
	engineMode     atomic.Int64  // 0=normal, 1=safe_mode, 2=paused
	positionVolume atomic.Int64
	floatPnl       atomic.Uint64 // float64 以 bits 存储
	openOrders     atomic.Int64
}

// NewMetricsRegistry 创建指标注册表
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		startTime:             time.Now(),
		signalsGeneratedTotal: newLabeledCounter(),
		ordersPlacedTotal:     newLabeledCounter(),
	}
}

// ============================================================
// IncCounter — 计数器操作方法
// ============================================================

// IncTicksProcessed 增加已处理 Tick 计数
func (m *MetricsRegistry) IncTicksProcessed(delta int64) {
	m.ticksProcessedTotal.Add(delta)
}

// IncSignalsGenerated 增加指定 action 的信号计数
func (m *MetricsRegistry) IncSignalsGenerated(action string, delta int64) {
	m.signalsGeneratedTotal.Inc(action, delta)
}

// IncOrdersPlaced 增加指定 direction 的报单计数
func (m *MetricsRegistry) IncOrdersPlaced(direction string, delta int64) {
	m.ordersPlacedTotal.Inc(direction, delta)
}

// IncOrdersRejected 增加风控拒绝计数
func (m *MetricsRegistry) IncOrdersRejected(delta int64) {
	m.ordersRejectedTotal.Add(delta)
}

// IncEmergencyTriggers 增加紧急触发计数
func (m *MetricsRegistry) IncEmergencyTriggers(delta int64) {
	m.emergencyTriggersTotal.Add(delta)
}

// ============================================================
// SetGauge — 仪表盘操作方法
// ============================================================

// SetAIScoreDecay 设置 AI 评分老化系数
func (m *MetricsRegistry) SetAIScoreDecay(v float64) {
	m.aiScoreDecay.Store(math.Float64bits(v))
}

// SetEngineMode 设置引擎模式
// 0=normal, 1=safe_mode, 2=paused
func (m *MetricsRegistry) SetEngineMode(mode int64) {
	m.engineMode.Store(mode)
}

// SetPositionVolume 设置当前持仓手数
func (m *MetricsRegistry) SetPositionVolume(v int64) {
	m.positionVolume.Store(v)
}

// SetFloatPnl 设置浮动盈亏
func (m *MetricsRegistry) SetFloatPnl(v float64) {
	m.floatPnl.Store(math.Float64bits(v))
}

// SetOpenOrders 设置未成交报单数
func (m *MetricsRegistry) SetOpenOrders(v int64) {
	m.openOrders.Store(v)
}

// ============================================================
// ServeHTTP — Prometheus text/plain 输出
// ============================================================

// ServeHTTP 实现 http.Handler 接口，输出 Prometheus 格式指标
func (m *MetricsRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	uptime := int64(time.Since(m.startTime).Seconds())

	// ---- Gauges ----
	_, _ = fmt.Fprintf(w, "# HELP trading_agent_uptime_seconds Service uptime\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_uptime_seconds gauge\n")
	_, _ = fmt.Fprintf(w, "trading_agent_uptime_seconds %d\n", uptime)

	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_ai_score_decay Current AI score decay coefficient\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_ai_score_decay gauge\n")
	_, _ = fmt.Fprintf(w, "trading_agent_ai_score_decay %v\n", math.Float64frombits(m.aiScoreDecay.Load()))

	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_engine_mode Engine mode (0=normal, 1=safe_mode, 2=paused)\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_engine_mode gauge\n")
	_, _ = fmt.Fprintf(w, "trading_agent_engine_mode %d\n", m.engineMode.Load())

	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_position_volume Current position volume (lots)\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_position_volume gauge\n")
	_, _ = fmt.Fprintf(w, "trading_agent_position_volume %d\n", m.positionVolume.Load())

	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_float_pnl Current floating PnL\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_float_pnl gauge\n")
	_, _ = fmt.Fprintf(w, "trading_agent_float_pnl %v\n", math.Float64frombits(m.floatPnl.Load()))

	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_open_orders Number of open orders\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_open_orders gauge\n")
	_, _ = fmt.Fprintf(w, "trading_agent_open_orders %d\n", m.openOrders.Load())

	// ---- Counters ----
	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_ticks_processed_total Total ticks processed\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_ticks_processed_total counter\n")
	_, _ = fmt.Fprintf(w, "trading_agent_ticks_processed_total %d\n", m.ticksProcessedTotal.Load())

	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_orders_rejected_total Total orders rejected by risk control\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_orders_rejected_total counter\n")
	_, _ = fmt.Fprintf(w, "trading_agent_orders_rejected_total %d\n", m.ordersRejectedTotal.Load())

	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_emergency_triggers_total Total emergency triggers\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_emergency_triggers_total counter\n")
	_, _ = fmt.Fprintf(w, "trading_agent_emergency_triggers_total %d\n", m.emergencyTriggersTotal.Load())

	// Labeled counter: signals_generated_total
	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_signals_generated_total Total signals generated by action\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_signals_generated_total counter\n")
	for action, count := range m.signalsGeneratedTotal.Snapshot() {
		_, _ = fmt.Fprintf(w, "trading_agent_signals_generated_total{action=\"%s\"} %d\n", action, count)
	}

	// Labeled counter: orders_placed_total
	_, _ = fmt.Fprintf(w, "\n# HELP trading_agent_orders_placed_total Total orders placed by direction\n")
	_, _ = fmt.Fprintf(w, "# TYPE trading_agent_orders_placed_total counter\n")
	for direction, count := range m.ordersPlacedTotal.Snapshot() {
		_, _ = fmt.Fprintf(w, "trading_agent_orders_placed_total{direction=\"%s\"} %d\n", direction, count)
	}
}
