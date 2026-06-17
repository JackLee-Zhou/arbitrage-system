// trading_agent/app — 依赖注入装配层
//
// 手动 DI 将所有组件串联起来。
// 实现 domain/engine/ports.go 定义的接口，将 adapters 层注入 domain 层。
package app

import (
	"context"
	"fmt"
	"log"
	"time"

	pkgredis "github.com/arbitrage-system/pkg/redis"

	"github.com/arbitrage-system/trading_agent/internal/adapters/influxdb"
	llmclient "github.com/arbitrage-system/trading_agent/internal/adapters/llm_client"
	redisadapters "github.com/arbitrage-system/trading_agent/internal/adapters/redis"
	"github.com/arbitrage-system/trading_agent/internal/api"
	"github.com/arbitrage-system/trading_agent/internal/api/grpc"
	"github.com/arbitrage-system/trading_agent/internal/domain/engine"
	"github.com/arbitrage-system/trading_agent/internal/domain/position"
	"github.com/arbitrage-system/trading_agent/internal/domain/risk"
	"github.com/arbitrage-system/trading_agent/internal/domain/strategy"
)

// ============================================================
// Config — 应用配置
// ============================================================

// Config 应用配置，在 main 中从环境变量加载后传入 NewDependencies。
type Config struct {
	RedisAddr       string
	InfluxDBURL     string
	InfluxDBToken   string
	InfluxDBOrg     string
	HealthPort      string
	GRPCPort        string
	Symbol          string
	AIAlpha         float64
	InitialCapital  float64
	MaxPosition     int
	DailyMaxLossPct float64
	StopLossPct     float64
	LLMScorerURL    string
}

// ============================================================
// Dependencies — 所有组件引用
// ============================================================

// Dependencies 持有所有组件实例，由 NewDependencies 统一装配。
type Dependencies struct {
	// Domain 层
	Engine *engine.Engine

	// Adapters
	RedisClient  *pkgredis.GoRedisAdapter
	InfluxReader *influxdb.InfluxReader
	TickSub      *redisadapters.TickSubscriber
	StateStore   *redisadapters.StateStore
	LLMClient    *llmclient.LLMClient

	// API 层
	GRPCServer    *grpc.GRPCServer
	HealthChecker *api.HealthChecker
	Metrics       *api.MetricsRegistry
}

// ============================================================
// Port 接口实现（domain/engine 所需的外部依赖）
// ============================================================

// ---- AIScoreReader 实现 ----

// aiScoreReader 包装 StateStore 实现 engine.AIScoreReader
type aiScoreReader struct {
	store  *redisadapters.StateStore
	symbol string
}

func (r *aiScoreReader) GetLatest(symbol string) (*engine.AIScore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	score, err := r.store.GetAIScore(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("get ai score: %w", err)
	}
	return &engine.AIScore{
		Score:      score.Score,
		Confidence: score.Confidence,
		Timestamp:  score.Timestamp,
	}, nil
}

// ---- OrderExecutor 实现 ----

// orderExecutor 包装 Broker 接口实现 engine.OrderExecutor
// V1：只记录日志，不实际下单（等 CTP broker 就绪后注入真实实现）
type orderExecutor struct {
	logOnly bool
}

func (e *orderExecutor) PlaceOrder(req engine.OrderRequest) (engine.OrderID, error) {
	log.Printf("[order] PlaceOrder: %s %s %d@%.2f type=%s",
		req.Symbol, req.Direction, req.Volume, req.Price, req.OrderType)
	return engine.OrderID(fmt.Sprintf("mock-%d", time.Now().UnixNano())), nil
}

func (e *orderExecutor) CancelOrder(orderID string) error {
	log.Printf("[order] CancelOrder: %s", orderID)
	return nil
}

// ---- StatePersister 实现 ----

// statePersister 包装 StateStore 实现 engine.StatePersister
type statePersister struct {
	store  *redisadapters.StateStore
	symbol string
}

func (p *statePersister) SavePosition(state *engine.PositionState) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pos := &redisadapters.Position{
		Symbol:    state.Symbol,
		Direction: state.Direction,
		Volume:    state.Volume,
		AvgPrice:  state.AvgPrice,
	}
	return p.store.SavePosition(ctx, pos)
}

func (p *statePersister) SaveEngineStatus(status *engine.EngineStatus) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	es := redisadapters.EngineStatus{
		Status:          string(status.Mode),
		Timestamp:       time.Now().Unix(),
		Mode:            string(status.Mode),
		Symbol:          status.Symbol,
		LastPrice:       status.LastPrice,
		CurrentPosition: status.CurrentPosition,
		DailyPnL:        status.DailyPnL,
		LastEmergencyAt: status.LastEmergencyAt,
	}
	return p.store.SaveEngineStatus(ctx, es)
}

func (p *statePersister) SaveSignal(record *engine.SignalRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	signal := &redisadapters.Signal{
		Symbol:    record.Symbol,
		Action:    record.Action,
		Strength:  record.Strength,
		Reason:    record.Reason,
		Timestamp: record.Timestamp,
	}
	return p.store.PublishSignal(ctx, signal)
}

// ---- StateChangeNotifier 实现 ----

// stateChangeNotifier 记录状态变更日志（V1 简单实现）
// TODO(P2): 实现 gRPC SubscribeStateChanges server stream 推送
type stateChangeNotifier struct{}

func (n *stateChangeNotifier) Notify(event *engine.StateChangeEvent) error {
	log.Printf("[engine] event: type=%s symbol=%s msg=%s",
		event.Type, event.Symbol, event.Message)
	return nil
}

// ---- LLMClient 适配器（wrap llm_client.LLMClient → engine.LLMClient） ----

// llmClientAdapter 包装 llm_client.LLMClient 实现 engine.LLMClient
type llmClientAdapter struct {
	client *llmclient.LLMClient
}

func (a *llmClientAdapter) RequestEmergencyAnalysis(symbol string, reason string, data *engine.VolatilityData) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := llmclient.EmergencyRequest{
		Symbol:        symbol,
		TriggerReason: reason,
		Volatility: llmclient.EmergencyVolatility{
			PriceChangePct:   data.PriceChangePct1m,
			VolumeMultiplier: data.VolumeMultiplier,
		},
	}

	resp, err := a.client.RequestEmergency(ctx, req)
	if err != nil {
		log.Printf("[engine] LLM emergency analysis failed: %v", err)
		return err
	}
	log.Printf("[engine] LLM emergency analysis: score=%.3f action=%s", resp.Base.Score, resp.SuggestedAction)
	return nil
}

// ============================================================
// NewDependencies — 依赖注入装配函数
// ============================================================

// NewDependencies 创建并装配所有组件。
func NewDependencies(cfg Config, redisClient *pkgredis.GoRedisAdapter, influxRdr *influxdb.InfluxReader) (*Dependencies, error) {
	log.Println("[app] assembling dependencies...")

	// ---- 创建 adapter 实例（不依赖 domain） ----
	tickSub := redisadapters.NewTickSubscriber(redisClient)
	stateStore := redisadapters.NewStateStore(redisClient)
	rawLLMClient := llmclient.NewLLMClient(cfg.LLMScorerURL)

	log.Printf("[app] adapters created: influx=%t llm=%t",
		influxRdr != nil, cfg.LLMScorerURL != "")

	// ---- 创建 port 接口实现 ----
	aiReader := &aiScoreReader{store: stateStore, symbol: cfg.Symbol}
	orderExec := &orderExecutor{logOnly: true}
	statePersist := &statePersister{store: stateStore, symbol: cfg.Symbol}
	notifier := &stateChangeNotifier{}
	llmAdapter := &llmClientAdapter{client: rawLLMClient}

	// ---- 创建 domain 组件 ----
	stratEng := strategy.NewEngine()
	stratEng.Register(&strategy.MATrendStrategy{FastMA: 5, SlowMA: 20})
	riskMgr := &risk.Manager{
		MaxPosition:   cfg.MaxPosition,
		MaxLossPerDay: cfg.InitialCapital * cfg.DailyMaxLossPct,
		StopLossPct:   cfg.StopLossPct,
		DailyPnL:      0,
	}
	posMgr := &position.Manager{}

	// ---- 创建 Engine ----
	engCfg := &engine.EngineConfig{
		Symbol:            cfg.Symbol,
		AIAlpha:           cfg.AIAlpha,
		HistorySize:       200,
		MaxPosition:       cfg.MaxPosition,
		DailyMaxLossPct:   cfg.DailyMaxLossPct,
		StopLossPct:       cfg.StopLossPct,
		EmergencyCooldown: 5 * time.Minute,
	}
	eng := engine.NewEngine(engCfg, stratEng, riskMgr, posMgr, aiReader, orderExec, statePersist, notifier, llmAdapter)
	log.Printf("[app] engine created: symbol=%s alpha=%.2f maxPos=%d", cfg.Symbol, cfg.AIAlpha, cfg.MaxPosition)

	// ---- gRPC Server ----
	// 创建 gRPC handler（实现 TradingAgentServer 接口）
	grpcHandler := grpc.NewTradingAgentHandler(
		&engineAccessorAdapter{engine: eng},
		&positionAccessorAdapter{mgr: posMgr},
		&riskAccessorAdapter{mgr: riskMgr},
		"trading_agent",
		"0.1.0",
	)
	grpcSrv := grpc.NewGRPCServer(":"+cfg.GRPCPort, grpcHandler)
	log.Printf("[app] gRPC server created: port=%s", cfg.GRPCPort)

	// ---- HealthChecker ----
	healthChecker := api.NewHealthChecker()
	healthChecker.Register("redis", func(ctx context.Context) error {
		return redisClient.Ping(ctx)
	})
	if influxRdr != nil {
		healthChecker.Register("influxdb", func(ctx context.Context) error {
			return influxRdr.Ping(ctx)
		})
	}
	healthChecker.Register("engine", func(ctx context.Context) error {
		if eng.IsSafeMode() {
			return fmt.Errorf("engine in safe_mode")
		}
		return nil
	})
	log.Println("[app] health checker created")

	// ---- Metrics ----
	metricsRegistry := api.NewMetricsRegistry()
	log.Println("[app] metrics registry created")

	log.Println("[app] dependencies assembled successfully")
	return &Dependencies{
		Engine:        eng,
		RedisClient:   redisClient,
		InfluxReader:  influxRdr,
		TickSub:       tickSub,
		StateStore:    stateStore,
		LLMClient:     rawLLMClient,
		GRPCServer:    grpcSrv,
		HealthChecker: healthChecker,
		Metrics:       metricsRegistry,
	}, nil
}

// ============================================================
// gRPC 接口适配器（engine → grpc.EngineAccessor 等）
// ============================================================

// engineAccessorAdapter 包装 engine.Engine 实现 grpc.EngineAccessor
type engineAccessorAdapter struct {
	engine *engine.Engine
}

func (a *engineAccessorAdapter) GetStatus() string {
	return string(a.engine.Mode())
}

func (a *engineAccessorAdapter) GetLastTickTime() int64 {
	return a.engine.GetStatus().LastTickTime
}

func (a *engineAccessorAdapter) GetLastSignalTime() int64 {
	return a.engine.GetStatus().LastSignalTime
}

func (a *engineAccessorAdapter) GetCurrentAIScore() float64 {
	return a.engine.GetStatus().CurrentAIScore
}

func (a *engineAccessorAdapter) GetAIScoreDecay() float64 {
	return a.engine.GetStatus().AIScoreDecay
}

func (a *engineAccessorAdapter) GetOpenOrders() int32 {
	return a.engine.GetStatus().OpenOrders
}

func (a *engineAccessorAdapter) PauseStrategy(name string) error {
	a.engine.SetMode(engine.ModePaused)
	return nil
}

func (a *engineAccessorAdapter) ResumeStrategy(name string) error {
	a.engine.SetMode(engine.ModeNormal)
	return nil
}

func (a *engineAccessorAdapter) EnterSafeMode(reason string) error {
	a.engine.SetMode(engine.ModeSafe)
	return nil
}

func (a *engineAccessorAdapter) EmergencyClosePositions() error {
	return a.engine.EmergencyClose()
}

// positionAccessorAdapter 包装 position.Manager 实现 grpc.PositionAccessor
type positionAccessorAdapter struct {
	mgr *position.Manager
}

func (a *positionAccessorAdapter) GetPositions() []grpc.PositionInfo {
	snapshot := a.mgr.Snapshot()
	result := make([]grpc.PositionInfo, 0, len(snapshot))
	for _, pos := range snapshot {
		if pos.Volume > 0 {
			result = append(result, grpc.PositionInfo{
				Symbol:      pos.Symbol,
				Direction:   pos.Direction,
				Volume:      int32(pos.Volume),
				AvgPrice:    pos.AvgPrice,
				LastPrice:   pos.LastPrice,
				FloatPnl:    pos.FloatPnL,
				TodayVolume: int32(pos.TodayVolume),
				YdVolume:    int32(pos.YdVolume),
			})
		}
	}
	return result
}

// riskAccessorAdapter 包装 risk.Manager 实现 grpc.RiskAccessor
type riskAccessorAdapter struct {
	mgr *risk.Manager
}

func (a *riskAccessorAdapter) UpdateRiskParams(param string, value float64) error {
	switch param {
	case "MaxPosition":
		a.mgr.MaxPosition = int(value)
	case "MaxLossPerDay":
		a.mgr.MaxLossPerDay = value
	case "StopLossPct":
		a.mgr.StopLossPct = value
	default:
		return fmt.Errorf("unknown risk param: %s", param)
	}
	log.Printf("[risk] UpdateRiskParams: %s=%f", param, value)
	return nil
}
