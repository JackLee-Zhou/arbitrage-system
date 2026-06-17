// trading_agent/domain/engine — 引擎主循环
//
// 职责：行情驱动的事件循环 —— 收到 Tick → 计算指标 → 跑策略 → 决策融合 → 风控 → 报单
// 注意：本包零外部依赖，所有 IO 通过 ports.go 接口委托给 adapters/ 层
package engine

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/arbitrage-system/trading_agent/internal/domain/decision"
	"github.com/arbitrage-system/trading_agent/internal/domain/indicator"
	"github.com/arbitrage-system/trading_agent/internal/domain/position"
	"github.com/arbitrage-system/trading_agent/internal/domain/risk"
	"github.com/arbitrage-system/trading_agent/internal/domain/strategy"
)

// ============================================================
// Engine — 决策引擎主循环
// ============================================================

// Engine 是交易决策引擎，负责接收 Tick → 计算指标 → 跑策略 → 决策融合 → 风控 → 报单。
// 所有外部依赖通过 ports.go 的接口注入，Engine 本身零外部依赖。
type Engine struct {
	cfg  *EngineConfig
	mode Mode

	// Domain 业务组件
	strategyEngine *strategy.Engine
	riskManager    *risk.Manager
	positionMgr    *position.Manager

	// 端口接口（由 adapters/ 层实现）
	aiScoreReader  AIScoreReader
	orderExecutor  OrderExecutor
	statePersister StatePersister
	stateNotifier  StateChangeNotifier
	llmClient      LLMClient

	// 价格/成交量缓冲区（FIFO，容量 cfg.HistorySize）
	prices  []float64
	highs   []float64 // 当日最高价（TODO: CTP adapter 就绪后从 Tick 填充）
	lows    []float64 // 当日最低价（TODO: CTP adapter 就绪后从 Tick 填充）
	volumes []int64

	// 上次累积成交量（用于计算本 Tick 增量）
	prevVolume int64

	// 累计状态
	lastPrice       float64
	dailyPnL        float64
	lastEmergencyAt time.Time

	// gRPC 查询用追踪字段
	lastTickTime   int64 // unix ms
	lastSignalTime int64 // unix ms
	lastAIScore    float64
	lastAIDecay    float64
	openOrders     int32
	lastPersistAt  time.Time // 上次持久化时间

	// 线程安全
	mu sync.RWMutex
}

// NewEngine 创建引擎实例
func NewEngine(
	cfg *EngineConfig,
	strategyEngine *strategy.Engine,
	riskManager *risk.Manager,
	positionMgr *position.Manager,
	aiScoreReader AIScoreReader,
	orderExecutor OrderExecutor,
	statePersister StatePersister,
	stateNotifier StateChangeNotifier,
	llmClient LLMClient,
) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Engine{
		cfg:            cfg,
		mode:           ModeNormal,
		strategyEngine: strategyEngine,
		riskManager:    riskManager,
		positionMgr:    positionMgr,
		aiScoreReader:  aiScoreReader,
		orderExecutor:  orderExecutor,
		statePersister: statePersister,
		stateNotifier:  stateNotifier,
		llmClient:      llmClient,
		prices:         make([]float64, 0, cfg.HistorySize),
		volumes:        make([]int64, 0, cfg.HistorySize),
	}
}

// ============================================================
// 引擎控制
// ============================================================

// SetMode 切换引擎模式（线程安全）
func (e *Engine) SetMode(m Mode) {
	e.mu.Lock()
	e.mode = m
	e.mu.Unlock()
	e.notifyStateChange("mode_changed", string(m))
}

// Mode 获取当前引擎模式（线程安全）
func (e *Engine) Mode() Mode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.mode
}

// IsSafeMode 是否处于安全模式
func (e *Engine) IsSafeMode() bool {
	return e.Mode() == ModeSafe
}

// IsPaused 是否暂停
func (e *Engine) IsPaused() bool {
	return e.Mode() == ModePaused
}

// LoadReconciledPosition 从外部数据源加载持仓（如启动对账恢复）
func (e *Engine) LoadReconciledPosition(symbol, direction string, volume int, avgPrice float64) {
	e.positionMgr.LoadPosition(symbol, direction, volume, avgPrice)
	e.riskManager.CurrentPosition = volume
}

// GetStatus 获取引擎状态快照
func (e *Engine) GetStatus() *EngineStatus {
	e.mu.RLock()
	mode := e.mode
	lastPrice := e.lastPrice
	lastEmAt := e.lastEmergencyAt.Unix()
	dailyPnL := e.dailyPnL
	lastTickTime := e.lastTickTime
	lastSignalTime := e.lastSignalTime
	lastAIScore := e.lastAIScore
	lastAIDecay := e.lastAIDecay
	openOrders := e.openOrders
	e.mu.RUnlock()

	pos := e.positionMgr.Get(e.cfg.Symbol)
	return &EngineStatus{
		Mode:            mode,
		Symbol:          e.cfg.Symbol,
		LastPrice:       lastPrice,
		CurrentPosition: pos.Volume,
		DailyPnL:        dailyPnL,
		LastEmergencyAt: lastEmAt,
		LastTickTime:    lastTickTime,
		LastSignalTime:  lastSignalTime,
		CurrentAIScore:  lastAIScore,
		AIScoreDecay:    lastAIDecay,
		OpenOrders:      openOrders,
	}
}

// EmergencyClose 紧急平仓所有持仓
// 平仓后自动切换为 SafeMode
func (e *Engine) EmergencyClose() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pos := e.positionMgr.Get(e.cfg.Symbol)
	if pos.Volume <= 0 {
		return nil
	}

	// 根据持仓方向确定平仓方向
	closeDir := "sell"
	if pos.Direction == "short" {
		closeDir = "buy"
	}

	req := OrderRequest{
		Symbol:    e.cfg.Symbol,
		Direction: closeDir,
		Offset:    "close",
		Volume:    pos.Volume,
		Price:     0, // 市价单
		OrderType: "market",
	}

	orderID, err := e.orderExecutor.PlaceOrder(req)
	if err != nil {
		return fmt.Errorf("emergency close failed: %w", err)
	}
	_ = orderID

	// 更新持仓
	realizedPnl := (e.lastPrice - pos.AvgPrice) * float64(pos.Volume)
	e.dailyPnL += realizedPnl
	e.riskManager.DailyPnL = e.dailyPnL
	e.riskManager.CurrentPosition = 0

	pos.Volume = 0
	pos.AvgPrice = 0
	pos.Direction = ""

	e.mode = ModeSafe
	e.notifyStateChange("emergency_close", "all positions closed, switched to SafeMode")
	return nil
}

// ============================================================
// ProcessTick — 核心处理流水线
// ============================================================

// ProcessTick 处理单个行情 Tick，引擎的主处理管道
//
// 数据流水线：
//
//	Tick → appendToBuffer → 指标计算 → StrategyInput → 策略引擎
//	  → 对每个信号：AI 评分 → 老化系数 → Fuse 融合 → 风控 → 报单 → 更新持仓
//	  → 紧急条件检查 → 状态持久化
func (e *Engine) ProcessTick(tick Tick) error {
	// 只处理关注的合约
	if tick.Symbol != e.cfg.Symbol {
		return nil
	}

	// Step 1: 追加到缓冲区
	e.appendToBuffer(tick)

	// Step 2: 检查数据量是否足够计算指标（至少 20 个数据点）
	if len(e.prices) < 20 {
		e.persistEngineStatus(tick)
		return nil
	}

	// Step 3: 计算技术指标
	fastMA := indicator.SMA(e.prices, 5)
	slowMA := indicator.SMA(e.prices, 20)
	boll := indicator.Bollinger(e.prices, 20, 2.0)
	atrVals := indicator.ATR(e.highs, e.lows, e.prices, 14)
	volRatio := indicator.VolumeRatio(e.volumes, 5, 20)

	if fastMA == nil || slowMA == nil || boll == nil || atrVals == nil {
		e.persistEngineStatus(tick)
		return nil
	}

	e.mu.Lock()
	e.lastPrice = tick.LastPrice
	e.mu.Unlock()

	// 更新浮动盈亏
	e.positionMgr.Get(e.cfg.Symbol).UpdateFloatPnL(tick.LastPrice)

	// Step 4: 构造策略输入并运行
	input := strategy.StrategyInput{
		Symbol:    e.cfg.Symbol,
		FastMA:    fastMA,
		SlowMA:    slowMA,
		BollUpper: boll.Upper,
		BollLower: boll.Lower,
		ATR:       atrVals[len(atrVals)-1],
		LastPrice: tick.LastPrice,
		Volume:    tick.Volume,
	}

	// Paused 模式跳过信号处理
	if e.Mode() != ModePaused {
		signals := e.strategyEngine.Run(input)
		for _, sig := range signals {
			// 追踪最后信号时间
			e.mu.Lock()
			e.lastSignalTime = tick.Timestamp
			e.mu.Unlock()
			e.processSignal(sig, tick)
		}
	}

	// Step 5: 检查紧急条件
	e.checkEmergency(tick, boll, volRatio)

	// Step 6: 持久化引擎状态（降低频率：每 1 秒一次，减少 Redis 写入）
	if time.Since(e.lastPersistAt) >= time.Second {
		e.persistEngineStatus(tick)
		e.lastPersistAt = time.Now()
	}

	return nil
}

// Run 启动引擎主循环（阻塞），从 TickSource 持续消费行情
func (e *Engine) Run(tickSource TickSource) error {
	for {
		tick, err := tickSource.Next()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		_ = e.ProcessTick(*tick)
	}
}

// ============================================================
// 内部辅助方法
// ============================================================

// appendToBuffer 将 Tick 追加到价格和成交量缓冲区，维护 FIFO 容量
func (e *Engine) appendToBuffer(tick Tick) {
	// 成交量增量换算（CTP 传累积成交量）
	volDelta := tick.Volume - e.prevVolume
	if volDelta < 0 || e.prevVolume == 0 {
		volDelta = tick.Volume
	}
	e.prevVolume = tick.Volume

	e.prices = append(e.prices, tick.LastPrice)
	// highs/lows：Tick 有高/低价则使用，否则退化为收盘价（TODO: CTP 数据就绪后自动填充）
	high := tick.HighPrice
	low := tick.LowPrice
	if high == 0 {
		high = tick.LastPrice
	}
	if low == 0 {
		low = tick.LastPrice
	}
	e.highs = append(e.highs, high)
	e.lows = append(e.lows, low)
	e.volumes = append(e.volumes, volDelta)

	// FIFO 裁剪
	if len(e.prices) > e.cfg.HistorySize {
		excess := len(e.prices) - e.cfg.HistorySize
		e.prices = e.prices[excess:]
		e.highs = e.highs[excess:]
		e.lows = e.lows[excess:]
		e.volumes = e.volumes[excess:]
	}
}

// processSignal 处理单个策略信号（完整流程：AI → 融合 → 风控 → 报单 → 更新持仓）
func (e *Engine) processSignal(sig *strategy.Signal, tick Tick) {
	// ---- Step a/b: 读取 AI 评分 + 计算老化系数 ----
	scoreVal := 0.0
	confVal := 0.5
	ageSec := int64(1800) // 默认 30 分钟已过期

	aiScore, err := e.aiScoreReader.GetLatest(e.cfg.Symbol)
	if err == nil && aiScore != nil {
		scoreVal = aiScore.Score
		confVal = aiScore.Confidence
		ageSec = time.Now().Unix() - aiScore.Timestamp
	}

	decay := decision.DecayScore(ageSec)

	// 追踪 AI 评分值用于 gRPC 查询
	e.mu.Lock()
	e.lastAIScore = scoreVal
	e.lastAIDecay = decay
	e.mu.Unlock()

	// 评分老化通知
	if decision.IsStale(ageSec) {
		e.notifyStateChange("ai_score_stale",
			fmt.Sprintf("AI score is %ds old (decay=0), trading on tech signal only", ageSec))
	} else if decision.IsDegrading(ageSec) {
		e.notifyStateChange("ai_score_degrading",
			fmt.Sprintf("AI score is %ds old (decay=%.1f)", ageSec, decay))
	}

	// ---- Step c: 决策融合 ----
	fused := decision.Fuse(scoreVal, confVal, decay, e.cfg.AIAlpha, sig.Strength, sig.Action)
	if fused.Action == "hold" || fused.PositionSize <= 0 {
		return
	}

	// ⚠️ isClose 必须在 Fuse 之后计算——Fuse 可能改变 action
	pos := e.positionMgr.Get(e.cfg.Symbol)
	isClose := fused.Action == "sell" && pos.Volume > 0

	// 无持仓可平
	if fused.Action == "sell" && pos.Volume <= 0 {
		return
	}

	// SafeMode 只允许平仓
	if e.Mode() == ModeSafe && !isClose {
		e.notifyStateChange("signal_blocked",
			fmt.Sprintf("SafeMode blocked %s (current=%d)", fused.Action, pos.Volume))
		return
	}

	// ---- Step d: 风控检查 ----
	avgPrice := pos.AvgPrice
	if avgPrice == 0 {
		avgPrice = tick.LastPrice // 无持仓时用当前价作为虚拟均价
	}

	result := e.riskManager.CheckNewOrder(fused.PositionSize, tick.LastPrice, avgPrice)
	if !result.Passed {
		e.notifyStateChange("risk_blocked", result.Reason)
		return
	}

	// ---- Step e: 报单 ----
	offset := "open"
	if isClose {
		offset = "close"
	}

	req := OrderRequest{
		Symbol:    e.cfg.Symbol,
		Direction: fused.Action,
		Offset:    offset,
		Volume:    fused.PositionSize,
		Price:     tick.LastPrice,
		OrderType: "market",
	}

	orderID, err := e.orderExecutor.PlaceOrder(req)
	if err != nil {
		e.notifyStateChange("order_failed", err.Error())
		return
	}
	_ = orderID

	// ---- Step f: 更新持仓 ----
	e.updatePosition(pos, fused.Action, fused.PositionSize, tick.LastPrice, isClose)

	// 记录信号流水
	record := &SignalRecord{
		Symbol:    e.cfg.Symbol,
		Action:    fused.Action,
		Strength:  fused.Confidence,
		Reason:    sig.Reason,
		Timestamp: time.Now().Unix(),
	}
	_ = e.statePersister.SaveSignal(record)

	// 持久化持仓快照
	ps := &PositionState{
		Symbol:    e.cfg.Symbol,
		Direction: pos.Direction,
		Volume:    pos.Volume,
		AvgPrice:  pos.AvgPrice,
	}
	_ = e.statePersister.SavePosition(ps)

	e.notifyStateChange("order_placed",
		fmt.Sprintf("%s %s %d手@%.2f order=%s", fused.Action, offset, fused.PositionSize, tick.LastPrice, orderID))
}

// updatePosition 更新持仓和风控状态
func (e *Engine) updatePosition(pos *position.Position, action string, volume int, price float64, isClose bool) {
	if isClose {
		// 平仓：计算已实现盈亏
		realizedPnl := (price - pos.AvgPrice) * float64(volume)
		e.mu.Lock()
		e.dailyPnL += realizedPnl
		e.mu.Unlock()
		e.riskManager.DailyPnL = e.dailyPnL

		pos.Volume -= volume
		if pos.Volume <= 0 {
			pos.Volume = 0
			pos.AvgPrice = 0
			pos.Direction = ""
		}
	} else {
		// 开仓/加仓：加权平均持仓成本
		totalCost := pos.AvgPrice*float64(pos.Volume) + price*float64(volume)
		pos.Volume += volume
		pos.AvgPrice = totalCost / float64(pos.Volume)
		pos.Direction = "long"
	}

	e.riskManager.CurrentPosition = pos.Volume
}

// checkEmergency 检查紧急触发条件
func (e *Engine) checkEmergency(tick Tick, boll *indicator.BollingerBand, volRatio float64) {
	// 计算涨跌幅（缓冲区倒数第 60 个数据点 ≈ 1 分钟前）
	pctChange := e.calcPriceChangePct()

	// 风险层内置检查：涨跌幅 > ±1% 或量比 > 3.0
	triggered := e.riskManager.EmergencyCheck(pctChange, volRatio)

	// 额外检查：价格突破布林上/下轨
	if boll != nil && (tick.LastPrice >= boll.Upper || tick.LastPrice <= boll.Lower) {
		triggered = true
	}

	if !triggered {
		return
	}

	// 5 分钟防抖
	e.mu.RLock()
	lastEm := e.lastEmergencyAt
	e.mu.RUnlock()
	if time.Since(lastEm) < e.cfg.EmergencyCooldown {
		return
	}

	e.mu.Lock()
	e.lastEmergencyAt = time.Now()
	e.mu.Unlock()

	// 确定触发原因
	reason := e.determineEmergencyReason(pctChange, volRatio, boll, tick)

	// 异步触发紧急分析（不阻塞主流程）
	go func() {
		data := &VolatilityData{
			PriceChangePct1m: pctChange,
			VolumeMultiplier: volRatio,
		}
		_ = e.llmClient.RequestEmergencyAnalysis(e.cfg.Symbol, reason, data)
	}()

	e.notifyStateChange("emergency_triggered",
		fmt.Sprintf("reason=%s pct=%.2f%% volRatio=%.1f", reason, pctChange*100, volRatio))
}

// calcPriceChangePct 计算近 N 个 Tick 的价格涨跌幅
// 取缓冲区倒数第 60 个数据点作为约 1 分钟前的参考价格
func (e *Engine) calcPriceChangePct() float64 {
	n := len(e.prices)
	if n < 2 {
		return 0
	}
	lookback := 60
	if n-1 < lookback {
		lookback = n - 1
	}
	oldPrice := e.prices[n-1-lookback]
	if oldPrice == 0 {
		return 0
	}
	return (e.prices[n-1] - oldPrice) / oldPrice
}

// determineEmergencyReason 确定紧急触发原因
func (e *Engine) determineEmergencyReason(pctChange, volRatio float64, boll *indicator.BollingerBand, tick Tick) string {
	if math.Abs(pctChange) > 0.01 {
		return "price_spike"
	}
	if volRatio > 3.0 {
		return "volume_surge"
	}
	if boll != nil && (tick.LastPrice >= boll.Upper || tick.LastPrice <= boll.Lower) {
		return "boll_breakout"
	}
	return "unknown"
}

// persistEngineStatus 持久化引擎状态
func (e *Engine) persistEngineStatus(tick Tick) {
	pos := e.positionMgr.Get(e.cfg.Symbol)

	e.mu.RLock()
	mode := e.mode
	dailyPnL := e.dailyPnL
	lastEmAt := e.lastEmergencyAt.Unix()
	e.mu.RUnlock()

	status := &EngineStatus{
		Mode:            mode,
		Symbol:          e.cfg.Symbol,
		LastPrice:       tick.LastPrice,
		CurrentPosition: pos.Volume,
		DailyPnL:        dailyPnL,
		LastEmergencyAt: lastEmAt,
	}
	_ = e.statePersister.SaveEngineStatus(status)
}

// notifyStateChange 发送状态变更通知（nil-safe）
func (e *Engine) notifyStateChange(eventType, message string) {
	if e.stateNotifier == nil {
		return
	}
	event := &StateChangeEvent{
		Type:      eventType,
		Symbol:    e.cfg.Symbol,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}
	_ = e.stateNotifier.Notify(event)
}
